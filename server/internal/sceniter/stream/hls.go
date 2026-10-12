package stream

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrBusy znamená, že běží maximum převodů (TRANSCODE_MAX_SESSIONS).
	ErrBusy = errors.New("server už převádí maximum videí najednou")
	// ErrNoFFmpeg znamená, že ffmpeg na serveru chybí.
	ErrNoFFmpeg = errors.New("ffmpeg není nainstalovaný")
	// ErrSessionGone znamená neznámou nebo ukončenou session.
	ErrSessionGone = errors.New("převod neexistuje nebo skončil")
	// ErrSegmentTimeout znamená, že segment nevznikl včas.
	ErrSegmentTimeout = errors.New("segment zatím není připravený")
)

// segmentSeconds je délka segmentu HLS. Při přebalení (-c copy) ffmpeg řeže
// jen na klíčových snímcích, takže skutečné segmenty mohou být delší.
const segmentSeconds = 6

// Options jsou parametry spuštění převodu.
type Options struct {
	UserID    uuid.UUID
	MediaID   uuid.UUID
	Path      string // absolutní cesta k souboru
	Mode      string
	Audio     *int // index zvukové stopy v souboru
	Duration  float64
	Bitrate   int64
	Width     int
	Height    int
	MaxHeight int
	HEVC      bool // obraz je HEVC – Safari chce tag hvc1
	// Start (s) je místo, od kterého má přebalení nebo převod zvuku začít
	// (skok mimo už hotovou část). Plný převod skáče po segmentech sám.
	Start float64
}

// Session je jeden běžící (nebo doběhlý) převod do HLS.
type Session struct {
	ID        string    `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	MediaID   uuid.UUID `json:"media_id"`
	Mode      string    `json:"mode"`
	Audio     *int      `json:"audio,omitempty"`
	StartedAt time.Time `json:"started_at"`
	// Start (s od začátku souboru) je místo, kde převod skutečně začíná –
	// nula časové osy playlistu. U plného převodu vždy 0.
	Start float64 `json:"start,omitempty"`

	key  string
	dir  string
	opts Options

	mu         sync.Mutex
	lastAccess time.Time
	cmd        *exec.Cmd
	cancel     context.CancelFunc
	done       chan struct{}
	err        error
	stderr     bytes.Buffer
	// start je index segmentu, od kterého běží aktuální ffmpeg (plný převod
	// se při skoku dál restartuje od jiného segmentu).
	start int
	// restartMu brání dvěma souběžným restartům (přehrávač žádá víc segmentů naráz).
	restartMu sync.Mutex
}

// SessionInfo je snímek session pro administraci.
type SessionInfo struct {
	ID         string    `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	MediaID    uuid.UUID `json:"media_id"`
	Mode       string    `json:"mode"`
	StartedAt  time.Time `json:"started_at"`
	LastAccess time.Time `json:"last_access"`
	Start      float64   `json:"start,omitempty"`
	Running    bool      `json:"running"`
	Error      string    `json:"error,omitempty"`
}

// Manager spouští a uklízí převody do HLS.
type Manager struct {
	ffmpeg  string
	ffprobe string
	root    string
	max     int
	idle    time.Duration
	log     *slog.Logger

	appCtx context.Context

	mu       sync.Mutex
	sessions map[string]*Session
	byKey    map[string]*Session
}

// NewManager vytvoří správce převodů. root je pracovní adresář (obsah se při
// startu maže), max strop souběžných převodů a idle doba nečinnosti, po
// které se převod ukončí.
func NewManager(root string, max int, idle time.Duration) *Manager {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		slog.Warn("ffmpeg nenalezen – videa půjdou přehrát jen přímo")
		ffmpeg = ""
	}
	// Bez ffprobe přebalení začíná vždy od začátku (skok jen do hotové části).
	ffprobe, _ := exec.LookPath("ffprobe")
	return &Manager{ffmpeg: ffmpeg, ffprobe: ffprobe, root: root, max: max, idle: idle,
		log: slog.Default().With("component", "sceniter-hls"), appCtx: context.Background(),
		sessions: map[string]*Session{}, byKey: map[string]*Session{}}
}

// Available řekne, jestli server umí převádět (je ffmpeg).
func (m *Manager) Available() bool { return m.ffmpeg != "" }

// Start vyčistí pracovní adresář po předchozím běhu a spustí úklid.
func (m *Manager) Start(ctx context.Context) {
	m.appCtx = ctx
	if m.root != "" {
		entries, _ := os.ReadDir(m.root)
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "hls-") {
				_ = os.RemoveAll(filepath.Join(m.root, e.Name()))
			}
		}
	}
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				m.stopAll()
				return
			case <-t.C:
				m.reap()
			}
		}
	}()
}

func sessionKey(o Options) string {
	a := "-"
	if o.Audio != nil {
		a = strconv.Itoa(*o.Audio)
	}
	return o.UserID.String() + "|" + o.MediaID.String() + "|" + o.Mode + "|" + a + "|" + strconv.FormatFloat(o.Start, 'f', 3, 64)
}

// Open vrátí session pro daný soubor a režim; existující se znovu použije
// (přehrávač se po obnovení stránky napojí na rozběhnutý převod).
func (m *Manager) Open(o Options) (*Session, error) {
	if m.ffmpeg == "" {
		return nil, ErrNoFFmpeg
	}
	if o.Mode != ModeRemux && o.Mode != ModeTranscodeAudio && o.Mode != ModeTranscode {
		return nil, fmt.Errorf("neznámý režim %q", o.Mode)
	}
	if o.Mode == ModeTranscode {
		o.Start = 0
	}
	key := sessionKey(o)

	m.mu.Lock()
	if s, ok := m.byKey[key]; ok {
		s.touch()
		m.mu.Unlock()
		return s, nil
	}
	m.mu.Unlock()

	// Místo začátku se zjišťuje mimo zámek (ffmpeg čte soubor). Přehrávač
	// master playlist načítá víc než jednou, proto se nejdřív zkusí běžící.
	start := 0.0
	if o.Start > 0 && m.ffprobe != "" {
		k, err := seekLanding(m.appCtx, m.ffmpeg, m.ffprobe, o.Path, o.Start)
		if err != nil {
			m.log.Warn("začátek převodu pro skok nezjištěn, převod od začátku", "media", o.MediaID, "err", err)
		}
		start = k
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.byKey[key]; ok {
		s.touch()
		return s, nil
	}
	// Stejný uživatel přepnul stopu nebo film: jeho dřívější převody
	// téhož souboru se ukončí, ať nezabírají místo.
	for _, s := range m.sessions {
		if s.UserID == o.UserID && s.MediaID == o.MediaID {
			m.removeLocked(s)
		}
	}
	if len(m.sessions) >= m.max {
		return nil, ErrBusy
	}

	id := randomID()
	s := &Session{ID: id, UserID: o.UserID, MediaID: o.MediaID, Mode: o.Mode, Audio: o.Audio,
		StartedAt: time.Now(), Start: start, key: key, dir: filepath.Join(m.root, "hls-"+id), opts: o, lastAccess: time.Now()}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("pracovní adresář převodu: %w", err)
	}
	if err := m.launch(s, 0); err != nil {
		_ = os.RemoveAll(s.dir)
		return nil, err
	}
	m.sessions[id] = s
	m.byKey[key] = s
	m.log.Info("převod spuštěn", "session", id, "mode", o.Mode, "media", o.MediaID, "start", start)
	return s, nil
}

// Lookup vrátí session uživatele podle ID.
func (m *Manager) Lookup(id string, userID uuid.UUID) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || s.UserID != userID {
		return nil, ErrSessionGone
	}
	s.touch()
	return s, nil
}

// KeepAlive obnoví nečinnost všech převodů uživatele pro daný soubor a vrátí,
// kolik jich bylo. Přehrávač ho volá pravidelně: s plným bufferem (hls.js
// si u nízkého bitrate natáhne i 10 minut dopředu) nebo při pauze nežádá
// segmenty déle, než je doba nečinnosti, a převod by se mu pod rukama smazal.
func (m *Manager) KeepAlive(userID, mediaID uuid.UUID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, s := range m.sessions {
		if s.UserID == userID && s.MediaID == mediaID {
			s.touch()
			n++
		}
	}
	return n
}

// Stop ukončí session (přehrávač odešel, nebo admin).
func (m *Manager) Stop(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if ok {
		m.removeLocked(s)
	}
	return ok
}

// List vrátí běžící převody pro administraci.
func (m *Manager) List() []SessionInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SessionInfo, 0, len(m.sessions))
	for _, s := range m.sessions {
		s.mu.Lock()
		info := SessionInfo{ID: s.ID, UserID: s.UserID, MediaID: s.MediaID, Mode: s.Mode,
			StartedAt: s.StartedAt, LastAccess: s.lastAccess, Start: s.Start, Running: s.running()}
		if s.err != nil {
			info.Error = s.err.Error()
		}
		s.mu.Unlock()
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (m *Manager) reap() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		s.mu.Lock()
		idle := time.Since(s.lastAccess)
		s.mu.Unlock()
		if idle > m.idle {
			m.log.Info("převod ukončen pro nečinnost", "session", s.ID)
			m.removeLocked(s)
		}
	}
}

func (m *Manager) stopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		m.removeLocked(s)
	}
}

func (m *Manager) removeLocked(s *Session) {
	delete(m.sessions, s.ID)
	if m.byKey[s.key] == s {
		delete(m.byKey, s.key)
	}
	s.kill()
	_ = os.RemoveAll(s.dir)
}

// launch spustí ffmpeg od segmentu start. Volá se se zámkem manageru nebo
// session (restart při skoku).
func (m *Manager) launch(s *Session, start int) error {
	ctx, cancel := context.WithCancel(m.appCtx)
	from := 0.0
	if s.Start > 0 {
		from = s.opts.Start // stejné -ss, jaké zjistilo přistání (seekLanding)
	}
	args := ffmpegArgs(s.opts, s.dir, start, from)
	cmd := exec.CommandContext(ctx, m.ffmpeg, args...)
	s.stderr.Reset()
	cmd.Stderr = &limitedBuffer{buf: &s.stderr, max: 8 << 10}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("spuštění ffmpeg: %w", err)
	}
	done := make(chan struct{})
	s.cmd, s.cancel, s.done, s.err, s.start = cmd, cancel, done, nil, start
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		if err != nil && ctx.Err() == nil {
			s.err = fmt.Errorf("ffmpeg: %v: %s", err, strings.TrimSpace(s.stderr.String()))
			m.log.Warn("převod selhal", "session", s.ID, "err", s.err)
		}
		s.mu.Unlock()
		close(done)
	}()
	return nil
}

// ffmpegArgs sestaví příkaz pro převod do HLS (fMP4 segmenty). start je
// segment, od kterého jede plný převod; from (s) místo, od kterého jede
// přebalení nebo převod zvuku (skutečný začátek viz seekLanding).
func ffmpegArgs(o Options, dir string, start int, from float64) []string {
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error"}
	switch {
	case start > 0:
		args = append(args, "-ss", strconv.Itoa(start*segmentSeconds))
	case from > 0:
		// Kopírovaný obraz začne na klíčovém snímku, kam demuxer přistane;
		// převáděný zvuk musí začít tamtéž, ne přesně v from. Posun mezi
		// stopami by šel do fMP4 jen přes edit list, který ExoPlayer ignoruje.
		args = append(args, "-ss", strconv.FormatFloat(from, 'f', 3, 64), "-noaccurate_seek")
	}
	args = append(args, "-i", o.Path, "-map", "0:v:0")
	if o.Audio != nil {
		args = append(args, "-map", "0:"+strconv.Itoa(*o.Audio))
	} else {
		args = append(args, "-map", "0:a:0?")
	}
	args = append(args, "-sn", "-dn", "-map_metadata", "-1", "-map_chapters", "-1")

	switch o.Mode {
	case ModeRemux:
		args = append(args, "-c:v", "copy", "-c:a", "copy")
	case ModeTranscodeAudio:
		args = append(args, "-c:v", "copy", "-c:a", "aac", "-ac", "2", "-b:a", "192k")
	case ModeTranscode:
		maxH := o.MaxHeight
		if maxH <= 0 {
			maxH = 1080
		}
		args = append(args,
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "22", "-profile:v", "high", "-pix_fmt", "yuv420p",
			"-maxrate", "8M", "-bufsize", "16M",
			"-vf", fmt.Sprintf("scale=-2:'min(%d,ih)'", maxH),
			// Klíčový snímek přesně na hranici segmentu: segmenty pak sedí na
			// syntetický playlist a skok dopředu jde restartem od segmentu.
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", segmentSeconds),
			"-sc_threshold", "0",
			"-c:a", "aac", "-ac", "2", "-b:a", "192k")
		if start > 0 {
			args = append(args, "-output_ts_offset", strconv.Itoa(start*segmentSeconds))
		}
	}
	if o.HEVC && o.Mode != ModeTranscode {
		args = append(args, "-tag:v", "hvc1")
	}
	args = append(args,
		"-avoid_negative_ts", "make_zero",
		"-f", "hls", "-hls_time", strconv.Itoa(segmentSeconds), "-hls_list_size", "0",
		"-hls_playlist_type", "event", "-hls_segment_type", "fmp4",
		"-hls_fmp4_init_filename", "init.mp4",
		"-start_number", strconv.Itoa(start),
		"-hls_flags", "independent_segments+temp_file",
		"-hls_segment_filename", filepath.Join(dir, "seg_%05d.m4s"),
		filepath.Join(dir, "index.m3u8"))
	return args
}

func (s *Session) touch() {
	s.mu.Lock()
	s.lastAccess = time.Now()
	s.mu.Unlock()
}

func (s *Session) running() bool {
	if s.done == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

func (s *Session) kill() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}

// Dir je pracovní adresář session.
func (s *Session) Dir() string { return s.dir }

// WaitFile počká, až soubor name v adresáři session vznikne (ffmpeg píše
// přes dočasný soubor a přejmenování, takže existující soubor je úplný).
func (s *Session) WaitFile(ctx context.Context, name string, timeout time.Duration) (string, error) {
	if name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return "", ErrSessionGone
	}
	p := filepath.Join(s.dir, name)
	deadline := time.Now().Add(timeout)
	for {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() && fi.Size() > 0 {
			s.touch()
			return p, nil
		}
		s.mu.Lock()
		failed, running := s.err, s.running()
		s.mu.Unlock()
		if failed != nil {
			return "", failed
		}
		if !running || time.Now().After(deadline) {
			if !running {
				return "", ErrSessionGone
			}
			return "", ErrSegmentTimeout
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func randomID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// limitedBuffer drží jen začátek výstupu ffmpeg (chybové hlášky).
type limitedBuffer struct {
	buf *bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if room := l.max - l.buf.Len(); room > 0 {
		if len(p) > room {
			l.buf.Write(p[:room])
		} else {
			l.buf.Write(p)
		}
	}
	return len(p), nil
}

// RewritePlaylist připojí k URI segmentů a init souboru dotaz query (token
// ?t=). Prohlížeč ani hls.js dotaz z adresy playlistu na relativní URI
// nepřenesou, takže bez přepisu by segmenty skončily na 401.
func RewritePlaylist(data []byte, query string) []byte {
	var out bytes.Buffer
	for _, line := range strings.SplitAfter(string(data), "\n") {
		trimmed := strings.TrimRight(line, "\r\n")
		eol := line[len(trimmed):]
		switch {
		case strings.HasPrefix(trimmed, "#EXT-X-MAP:"):
			trimmed = strings.Replace(trimmed, `URI="init.mp4"`, `URI="init.mp4?`+query+`"`, 1)
		case trimmed != "" && !strings.HasPrefix(trimmed, "#"):
			trimmed += "?" + query
		}
		out.WriteString(trimmed + eol)
	}
	return out.Bytes()
}

// Playlist vrátí media playlist session. Přebalení a převod zvuku servírují
// playlist, který píše ffmpeg (roste, dokud převod neskončí). Plný převod
// dostane syntetický VOD playlist s celou délkou filmu – přehrávač tak zná
// délku a umí skočit kamkoli; chybějící segment vyrobí restart ffmpeg.
func (m *Manager) Playlist(ctx context.Context, s *Session) ([]byte, error) {
	if s.Mode == ModeTranscode && s.opts.Duration > 0 {
		return vodPlaylist(s.opts.Duration), nil
	}
	p, err := s.WaitFile(ctx, "index.m3u8", 30*time.Second)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// vodPlaylist vyrobí playlist se segmenty po segmentSeconds.
func vodPlaylist(duration float64) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%d\n#EXT-X-MEDIA-SEQUENCE:0\n", segmentSeconds)
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	n := 0
	for left := duration; left > 0.01; left -= segmentSeconds {
		d := min(left, float64(segmentSeconds))
		fmt.Fprintf(&b, "#EXTINF:%.3f,\nseg_%05d.m4s\n", d, n)
		n++
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return []byte(b.String())
}

// restartAhead: o kolik segmentů dopředu před už vyrobenými se ještě čeká,
// místo aby se převod restartoval (běžné přehrávání čte těsně za ffmpeg).
const restartAhead = 4

// Segment vrátí cestu k souboru segmentu (nebo init.mp4) a počká, až vznikne.
func (m *Manager) Segment(ctx context.Context, s *Session, name string) (string, error) {
	if s.Mode == ModeTranscode {
		if n, ok := segmentNumber(name); ok {
			if err := m.ensureProducing(s, n); err != nil {
				return "", err
			}
		}
	}
	return s.WaitFile(ctx, name, 45*time.Second)
}

func segmentNumber(name string) (int, bool) {
	if !strings.HasPrefix(name, "seg_") || !strings.HasSuffix(name, ".m4s") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "seg_"), ".m4s"))
	return n, err == nil
}

// ensureProducing zajistí, že ffmpeg segment n vyrobí: když už existuje nebo
// je těsně na řadě, nic nedělá; jinak převod restartuje od segmentu n.
func (m *Manager) ensureProducing(s *Session, n int) error {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()

	if fi, err := os.Stat(filepath.Join(s.dir, fmt.Sprintf("seg_%05d.m4s", n))); err == nil && fi.Size() > 0 {
		return nil
	}
	s.mu.Lock()
	start, running := s.start, s.running()
	s.mu.Unlock()
	if running && n >= start && n <= highestSegment(s.dir, start)+restartAhead {
		return nil
	}
	s.kill()
	s.mu.Lock()
	defer s.mu.Unlock()
	m.log.Info("převod restartován", "session", s.ID, "segment", n)
	return m.launch(s, n)
}

// highestSegment vrátí nejvyšší vyrobený segment od start (start-1, když žádný).
func highestSegment(dir string, start int) int {
	hi := start - 1
	for i := start; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("seg_%05d.m4s", i))); err != nil {
			return hi
		}
		hi = i
	}
}
