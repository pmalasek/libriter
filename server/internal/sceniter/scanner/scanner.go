// Package scanner sleduje VIDEO_ROOT a udržuje knihovnu filmů a seriálů.
//
// Zařazení souboru:
//   - pod adresářem filmů (SCENITER_MOVIES_DIRS) je film: soubor ležící
//     naplocho je film sám, soubor v podadresáři patří filmu toho adresáře;
//   - pod adresářem seriálů (SCENITER_SHOWS_DIRS) je díl: první podadresář je
//     seriál, další úroveň může být řada ("S01", "Season 1", "2. řada");
//   - jinde rozhoduje název: SxxEyy nebo NxNN je díl, jinak film.
//
// ffprobe běží mimo databázovou transakci; zápis souboru je pak krátký.
package scanner

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"mediater/internal/sceniter/storage"
)

const (
	stabilizeEvery = 3 * time.Second  // jak často kontrolovat velikost souboru
	stabilizeFor   = 10 * time.Second // jak dlouho musí být velikost beze změny
	ingestTimeout  = 6 * time.Hour    // filmy jsou velké a síť bývá pomalá
)

// ErrScanRunning znamená, že průchod knihovnou už běží.
var ErrScanRunning = errors.New("kontrola knihovny už běží")

// ErrRootMissing znamená, že VIDEO_ROOT neexistuje (nepřipojený disk).
// Scanner v tu chvíli nic nemaže – prázdný adresář by jinak smazal knihovnu.
var ErrRootMissing = errors.New("VIDEO_ROOT neexistuje")

// Status je snímek práce scanneru pro administraci.
type Status struct {
	Running       bool       `json:"running"`
	Trigger       string     `json:"trigger"`
	StartedAt     *time.Time `json:"started_at"`
	FinishedAt    *time.Time `json:"finished_at"`
	Processed     int        `json:"processed"`
	Added         int        `json:"added"`
	Removed       int        `json:"removed"`
	Errors        int        `json:"errors"`
	LastError     string     `json:"last_error"`
	WatcherActive bool       `json:"watcher_active"`
	FFprobe       bool       `json:"ffprobe"`
}

// Scanner udržuje knihovnu v souladu s diskem.
type Scanner struct {
	root      string
	movieDirs []string
	showDirs  []string
	store     *storage.Store
	ffprobe   string
	log       *slog.Logger

	// OnChange se zavolá po průchodu knihovnou nebo po přidání souboru
	// (spouští párování s TMDB). Nastavuje se před Start.
	OnChange func()

	appCtx  context.Context
	ingest  sync.Mutex // serializuje zakládání titulů (EnsureMovie není atomické)
	mu      sync.Mutex
	pending map[string]struct{}

	statusMu sync.Mutex
	status   Status
}

// New vytvoří scanner. ffprobe se hledá v PATH; bez něj se soubory založí
// bez údajů o stopách a přehrávání se rozhoduje jen podle přípony.
func New(root string, movieDirs, showDirs []string, store *storage.Store) *Scanner {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		slog.Warn("ffprobe nenalezen – videa se založí bez údajů o stopách")
		ffprobe = ""
	}
	s := &Scanner{
		root:      root,
		movieDirs: movieDirs,
		showDirs:  showDirs,
		store:     store,
		ffprobe:   ffprobe,
		log:       slog.Default().With("component", "sceniter-scanner"),
		appCtx:    context.Background(),
		pending:   map[string]struct{}{},
	}
	s.status.FFprobe = ffprobe != ""
	return s
}

// Start spustí počáteční průchod, sledování změn a případný pravidelný průchod.
func (s *Scanner) Start(ctx context.Context, rescanEvery time.Duration) {
	s.appCtx = ctx
	if s.begin("startup") {
		go s.run(ctx)
	}
	go s.watch(ctx)
	if rescanEvery > 0 {
		go func() {
			t := time.NewTicker(rescanEvery)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if s.begin("periodic") {
						s.run(ctx)
					}
				}
			}
		}()
	}
}

// Rescan spustí průchod na pozadí; když už běží, vrátí ErrScanRunning.
func (s *Scanner) Rescan() error {
	if !s.begin("manual") {
		return ErrScanRunning
	}
	go s.run(s.appCtx)
	return nil
}

// Status vrátí snímek stavu.
func (s *Scanner) Status() Status {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	return s.status
}

func (s *Scanner) begin(trigger string) bool {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.status.Running {
		return false
	}
	now := time.Now()
	s.status = Status{Running: true, Trigger: trigger, StartedAt: &now,
		WatcherActive: s.status.WatcherActive, FFprobe: s.status.FFprobe}
	return true
}

func (s *Scanner) end() {
	s.statusMu.Lock()
	now := time.Now()
	s.status.Running = false
	s.status.FinishedAt = &now
	s.statusMu.Unlock()
	if s.OnChange != nil {
		s.OnChange()
	}
}

func (s *Scanner) count(added bool, err error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.Processed++
	if added {
		s.status.Added++
	}
	if err != nil {
		s.status.Errors++
		s.status.LastError = err.Error()
	}
}

func (s *Scanner) fail(err error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	s.status.Errors++
	s.status.LastError = err.Error()
}

// run projde VIDEO_ROOT, založí nové a změněné soubory a smaže zmizelé.
func (s *Scanner) run(ctx context.Context) {
	defer s.end()

	if info, err := os.Stat(s.root); err != nil || !info.IsDir() {
		s.log.Warn("VIDEO_ROOT není dostupný, průchod přeskočen", "root", s.root)
		s.fail(ErrRootMissing)
		return
	}

	s.log.Info("spouštím průchod videoknihovnou", "root", s.root)
	seen := map[string]bool{}
	walkErr := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // nečitelný adresář přeskočit, zbytek projít
		}
		if path != s.root && Ignored(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() || !IsVideo(path) {
			return nil
		}
		seen[s.rel(path)] = true
		added, perr := s.process(ctx, path)
		s.count(added, perr)
		return nil
	})
	if walkErr != nil {
		s.log.Warn("průchod videoknihovnou přerušen", "err", walkErr)
		return // neúplný průchod nesmí mazat
	}

	paths, err := s.store.ListMediaPaths(ctx)
	if err != nil {
		s.fail(err)
		return
	}
	removed := 0
	for p, id := range paths {
		if seen[p] {
			continue
		}
		if err := s.store.DeleteMedia(ctx, id); err != nil {
			s.fail(err)
			continue
		}
		removed++
	}
	s.statusMu.Lock()
	s.status.Removed = removed
	s.statusMu.Unlock()
	s.log.Info("průchod videoknihovnou dokončen", "soubory", len(seen), "odebráno", removed)
}

func (s *Scanner) rel(abs string) string {
	r, err := filepath.Rel(s.root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(r)
}

// process založí nebo aktualizuje jeden soubor. Vrací true u nového souboru.
func (s *Scanner) process(ctx context.Context, abs string) (bool, error) {
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return false, nil
	}
	rel := s.rel(abs)

	existing, err := s.store.MediaByPath(ctx, rel)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return false, err
	}
	changed := existing == nil || !existing.Probed || existing.SizeBytes != info.Size() ||
		!existing.ModTime.Equal(info.ModTime().UTC().Truncate(time.Second))

	in := storage.SaveMediaInput{
		FilePath:  rel,
		SizeBytes: info.Size(),
		ModTime:   info.ModTime().UTC().Truncate(time.Second),
		Subtitles: s.subtitlesFor(abs),
	}
	if changed && s.ffprobe != "" {
		in.Probe = Probe(ctx, s.ffprobe, abs)
	}

	s.ingest.Lock()
	defer s.ingest.Unlock()

	// Vlastník (film nebo díl) se u známého souboru nemění – mohl ho ručně
	// přeřadit editor. Nový soubor se zařadí podle cesty.
	if existing != nil {
		in.MovieID, in.EpisodeID = existing.MovieID, existing.EpisodeID
	} else if err := s.classify(ctx, abs, rel, &in); err != nil {
		return false, err
	}
	if _, err := s.store.SaveMedia(ctx, in); err != nil {
		return false, err
	}
	return existing == nil, nil
}

// classify určí film nebo díl, ke kterému soubor patří, a případně ho založí.
func (s *Scanner) classify(ctx context.Context, abs, rel string, in *storage.SaveMediaInput) error {
	parts := strings.Split(rel, "/")
	stem := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(rel))

	switch {
	case len(parts) >= 2 && containsFold(s.movieDirs, parts[0]):
		return s.asMovie(ctx, parts[1:], parts[:1], stem, in)
	case len(parts) >= 2 && containsFold(s.showDirs, parts[0]):
		return s.asEpisode(ctx, abs, parts[1:], parts[:1], stem, in)
	case LooksLikeEpisode(stem):
		return s.asEpisode(ctx, abs, parts, nil, stem, in)
	default:
		return s.asMovie(ctx, parts, nil, stem, in)
	}
}

// asMovie: parts je cesta pod adresářem filmů (prefix), poslední je soubor.
func (s *Scanner) asMovie(ctx context.Context, parts, prefix []string, stem string, in *storage.SaveMediaInput) error {
	var key string
	info := ParseMovie(stem)
	if len(parts) == 1 {
		key = strings.Join(append(append([]string{}, prefix...), stem), "/")
	} else {
		// Film v adresáři: "Filmy/Pelíšky (1999)/pelisky.cd1.avi"
		key = strings.Join(append(append([]string{}, prefix...), parts[0]), "/")
		if dir := ParseMovie(parts[0]); dir.Year != 0 || info.Year == 0 {
			info = dir
		}
	}
	id, err := s.store.EnsureMovie(ctx, storage.MovieKey{
		SourceKey: key, Title: info.Title, OriginalTitle: info.OriginalTitle, Year: info.Year,
	})
	if err != nil {
		return err
	}
	in.MovieID = &id
	return nil
}

// asEpisode: parts je cesta pod adresářem seriálů, první je adresář seriálu.
func (s *Scanner) asEpisode(ctx context.Context, abs string, parts, prefix []string, stem string, in *storage.SaveMediaInput) error {
	ep := ParseEpisode(stem)

	var showDir string
	if len(parts) >= 2 {
		showDir = parts[0]
	} else {
		// Díl ležící naplocho: seriál je text před číslem dílu ("Show.S01E02").
		loc := sxxexxRe.FindStringIndex(stem)
		if loc == nil {
			loc = nxnnRe.FindStringIndex(stem)
		}
		showDir = stem
		if loc != nil && loc[0] > 0 {
			showDir = stem[:loc[0]]
		}
	}
	show := ParseMovie(showDir)
	key := strings.Join(append(append([]string{}, prefix...), showDir), "/")
	if len(parts) < 2 {
		key = strings.Join(append(append([]string{}, prefix...), "@"+show.Title), "/")
	}

	season := ep.Season
	if season == 0 {
		season = 1
		for _, dir := range parts[1:max(len(parts)-1, 1)] {
			if n, ok := ParseSeasonDir(dir); ok {
				season = n
			}
		}
	}
	number := ep.Episode
	if !ep.OK {
		number = positionInDir(abs)
	}

	id, err := s.store.EnsureEpisode(ctx, storage.EpisodeKey{
		ShowKey: key, ShowTitle: show.Title, ShowYear: show.Year,
		Season: season, Episode: number, Title: ep.Title,
	})
	if err != nil {
		return err
	}
	in.EpisodeID = &id
	return nil
}

// positionInDir vrátí pořadí souboru mezi videi v adresáři (1 = první).
// Díl bez čísla v názvu se tak řadí podle přirozeného pořadí souborů.
func positionInDir(abs string) int {
	entries, err := os.ReadDir(filepath.Dir(abs))
	if err != nil {
		return 1
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && IsVideo(e.Name()) && !Ignored(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for i, n := range names {
		if n == filepath.Base(abs) {
			return i + 1
		}
	}
	return 1
}

// subtitlesFor najde externí titulky vedle videa.
func (s *Scanner) subtitlesFor(abs string) []storage.SubtitleFile {
	dir := filepath.Dir(abs)
	stem := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []storage.SubtitleFile
	for _, e := range entries {
		if e.IsDir() || Ignored(e.Name()) {
			continue
		}
		lang, forced, ok := SubtitleMatch(stem, e.Name())
		if !ok {
			continue
		}
		out = append(out, storage.SubtitleFile{
			FilePath: s.rel(filepath.Join(dir, e.Name())),
			Language: lang,
			Format:   SubtitleExts[strings.ToLower(filepath.Ext(e.Name()))],
			Forced:   forced,
		})
	}
	return out
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// ---- sledování změn ----

func (s *Scanner) setWatcher(active bool) {
	s.statusMu.Lock()
	s.status.WatcherActive = active
	s.statusMu.Unlock()
}

func (s *Scanner) watch(ctx context.Context) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Error("nelze spustit sledování videoknihovny", "err", err)
		return
	}
	defer w.Close()

	_ = filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			if path != s.root && Ignored(d.Name()) {
				return filepath.SkipDir
			}
			_ = w.Add(path)
		}
		return nil
	})
	s.setWatcher(true)
	defer s.setWatcher(false)

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			s.handle(ctx, w, ev)
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			s.log.Warn("sledování videoknihovny", "err", err)
		}
	}
}

func (s *Scanner) handle(ctx context.Context, w *fsnotify.Watcher, ev fsnotify.Event) {
	name := filepath.Base(ev.Name)
	if Ignored(name) {
		return
	}
	if ev.Has(fsnotify.Create) {
		if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
			_ = w.Add(ev.Name)
			return
		}
	}
	switch {
	case (ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)) && IsVideo(ev.Name):
		if rec, err := s.store.MediaByPath(ctx, s.rel(ev.Name)); err == nil {
			if err := s.store.DeleteMedia(ctx, rec.ID); err != nil {
				s.log.Warn("odebrání videa", "path", s.rel(ev.Name), "err", err)
			}
		}
	case (ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write)) && IsVideo(ev.Name):
		if s.markPending(ev.Name) {
			go s.waitAndProcess(ctx, ev.Name)
		}
	case (ev.Has(fsnotify.Create) || ev.Has(fsnotify.Write) || ev.Has(fsnotify.Remove)) &&
		SubtitleExts[strings.ToLower(filepath.Ext(name))] != "":
		s.refreshSubtitles(ctx, ev.Name)
	}
}

// refreshSubtitles znovu spáruje titulky u videí ve stejném adresáři.
func (s *Scanner) refreshSubtitles(ctx context.Context, subPath string) {
	entries, err := os.ReadDir(filepath.Dir(subPath))
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !IsVideo(e.Name()) {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if _, _, ok := SubtitleMatch(stem, filepath.Base(subPath)); ok {
			abs := filepath.Join(filepath.Dir(subPath), e.Name())
			if _, err := s.store.MediaByPath(ctx, s.rel(abs)); err == nil {
				_, perr := s.process(ctx, abs)
				if perr != nil {
					s.log.Warn("titulky", "path", s.rel(abs), "err", perr)
				}
			}
		}
	}
}

func (s *Scanner) markPending(p string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[p]; ok {
		return false
	}
	s.pending[p] = struct{}{}
	return true
}

func (s *Scanner) clearPending(p string) {
	s.mu.Lock()
	delete(s.pending, p)
	s.mu.Unlock()
}

// waitAndProcess počká, až se velikost souboru přestane měnit (kopírování
// skončilo), a pak ho založí.
func (s *Scanner) waitAndProcess(ctx context.Context, abs string) {
	defer s.clearPending(abs)
	deadline := time.Now().Add(ingestTimeout)
	var last int64 = -1
	var stable time.Duration
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(stabilizeEvery):
		}
		if time.Now().After(deadline) {
			return
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return
		}
		if fi.Size() > 0 && fi.Size() == last {
			stable += stabilizeEvery
			if stable >= stabilizeFor {
				added, err := s.process(ctx, abs)
				s.count(added, err)
				if err != nil {
					s.log.Warn("založení videa", "path", s.rel(abs), "err", err)
				}
				if s.OnChange != nil {
					s.OnChange()
				}
				return
			}
		} else {
			last, stable = fi.Size(), 0
		}
	}
}
