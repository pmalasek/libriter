// Package scanner sleduje VIDEO_ROOT a udržuje knihovnu filmů a seriálů.
//
// Prochází jen podsložky VIDEO_ROOT, ze kterých admin udělal knihovnu
// (sc_libraries). Typ obsahu určuje knihovna:
//   - v knihovně filmů je soubor ležící naplocho film sám, soubor
//     v podadresáři patří filmu toho adresáře;
//   - v knihovně seriálů je první podadresář seriál, další úroveň může být
//     řada ("S01", "Season 1", "2. řada"); díl ležící naplocho patří seriálu
//     podle textu před číslem dílu ("Show.S01E02").
//
// Soubory mimo knihovny se ignorují.
//
// ffprobe běží mimo databázovou transakci; zápis souboru je pak krátký.
package scanner

import (
	"context"
	"errors"
	"fmt"
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

	"mediater/internal/sceniter/model"
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
	root    string
	store   *storage.Store
	ffprobe string
	log     *slog.Logger

	// OnChange se zavolá po průchodu knihovnou nebo po přidání souboru
	// (spouští párování metadat). Nastavuje se před Start.
	OnChange func()

	appCtx  context.Context
	ingest  sync.Mutex // serializuje zakládání titulů (EnsureMovie není atomické)
	mu      sync.Mutex
	pending map[string]struct{}

	statusMu sync.Mutex
	status   Status
	// queued: průchod vyžádaný během běžícího průchodu (změna knihoven) –
	// spustí se hned po něm.
	queued bool
}

// New vytvoří scanner. ffprobe se hledá v PATH; bez něj se soubory založí
// bez údajů o stopách a přehrávání se rozhoduje jen podle přípony.
func New(root string, store *storage.Store) *Scanner {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		slog.Warn("ffprobe nenalezen – videa se založí bez údajů o stopách")
		ffprobe = ""
	}
	s := &Scanner{
		root:    root,
		store:   store,
		ffprobe: ffprobe,
		log:     slog.Default().With("component", "sceniter-scanner"),
		appCtx:  context.Background(),
		pending: map[string]struct{}{},
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

// RescanSoon spustí průchod; když už běží, naplánuje další hned po něm.
// Volá se po změně knihoven, kdy běžící průchod pracuje se starým seznamem.
func (s *Scanner) RescanSoon() {
	if s.begin("libraries") {
		go s.run(s.appCtx)
		return
	}
	s.statusMu.Lock()
	s.queued = true
	s.statusMu.Unlock()
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
	queued := s.queued
	s.queued = false
	s.statusMu.Unlock()
	if s.OnChange != nil {
		s.OnChange()
	}
	if queued && s.appCtx.Err() == nil && s.begin("libraries") {
		go s.run(s.appCtx)
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

// run projde knihovny, založí nové a změněné soubory a smaže zmizelé.
func (s *Scanner) run(ctx context.Context) {
	defer s.end()

	if info, err := os.Stat(s.root); err != nil || !info.IsDir() {
		s.log.Warn("VIDEO_ROOT není dostupný, průchod přeskočen", "root", s.root)
		s.fail(ErrRootMissing)
		return
	}
	libs, err := s.store.ListLibraries(ctx, true)
	if err != nil {
		s.fail(err)
		return
	}

	s.log.Info("spouštím průchod videoknihovnou", "root", s.root, "knihovny", len(libs))
	seen := map[string]bool{}
	// walked jsou knihovny, které se prošly celé – jen v nich se smí mazat
	// zmizelé soubory. Chybějící složka (přejmenovaná, nepřipojená) nesmaže nic.
	walked := map[string]bool{}
	for i := range libs {
		lib := &libs[i]
		root := filepath.Join(s.root, lib.Dir)
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			s.log.Warn("složka knihovny neexistuje", "knihovna", lib.Name, "dir", root)
			s.fail(fmt.Errorf("složka knihovny %s neexistuje", lib.Dir))
			continue
		}
		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // nečitelný adresář přeskočit, zbytek projít
			}
			if path != root && Ignored(d.Name()) {
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
			added, perr := s.processIn(ctx, lib, path)
			s.count(added, perr)
			return nil
		})
		if walkErr != nil {
			s.log.Warn("průchod knihovnou přerušen", "knihovna", lib.Name, "err", walkErr)
			if ctx.Err() != nil {
				return // neúplný průchod nesmí mazat
			}
			continue
		}
		walked[lib.Dir] = true
	}

	paths, err := s.store.ListMediaPaths(ctx)
	if err != nil {
		s.fail(err)
		return
	}
	removed := 0
	for p, id := range paths {
		top, _, _ := strings.Cut(p, "/")
		if seen[p] || !walked[top] {
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

// libraryFor najde zapnutou knihovnu, do které soubor patří (podle první
// složky cesty), nebo nil.
func (s *Scanner) libraryFor(ctx context.Context, rel string) (*model.Library, error) {
	top, _, ok := strings.Cut(rel, "/")
	if !ok {
		return nil, nil // soubor přímo ve VIDEO_ROOT nepatří žádné knihovně
	}
	libs, err := s.store.ListLibraries(ctx, true)
	if err != nil {
		return nil, err
	}
	for i := range libs {
		if libs[i].Dir == top {
			return &libs[i], nil
		}
	}
	return nil, nil
}

// process založí nebo aktualizuje jeden soubor (sledování změn). Soubor
// mimo knihovny se ignoruje.
func (s *Scanner) process(ctx context.Context, abs string) (bool, error) {
	lib, err := s.libraryFor(ctx, s.rel(abs))
	if err != nil || lib == nil {
		return false, err
	}
	return s.processIn(ctx, lib, abs)
}

// processIn založí nebo aktualizuje jeden soubor knihovny. Vrací true
// u nového souboru.
func (s *Scanner) processIn(ctx context.Context, lib *model.Library, abs string) (bool, error) {
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

	// Nový soubor se zařadí podle cesty. Známý soubor zůstává u svého filmu
	// nebo dílu; jen díl, jehož číslo je v názvu souboru, se opraví, když
	// soubor patří jinam (číslování podle pořadí při postupném kopírování).
	if existing != nil {
		in.MovieID, in.EpisodeID = existing.MovieID, existing.EpisodeID
		if lib.Kind == model.KindShow && in.EpisodeID != nil {
			if err := s.reassignEpisode(ctx, lib, abs, rel, &in); err != nil {
				return false, err
			}
		}
	} else if err := s.classify(ctx, lib, abs, rel, &in); err != nil {
		return false, err
	}
	if _, err := s.store.SaveMedia(ctx, in); err != nil {
		return false, err
	}
	return existing == nil, nil
}

// classify určí film nebo díl, ke kterému soubor patří, a případně ho
// založí. Typ určuje knihovna; parts[0] je její složka.
func (s *Scanner) classify(ctx context.Context, lib *model.Library, abs, rel string, in *storage.SaveMediaInput) error {
	parts := strings.Split(rel, "/")
	stem := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(rel))

	if lib.Kind == model.KindShow {
		return s.asEpisode(ctx, lib, abs, parts[1:], parts[:1], stem, in)
	}
	return s.asMovie(ctx, lib, parts[1:], parts[:1], stem, in)
}

// asMovie: parts je cesta pod adresářem filmů (prefix), poslední je soubor.
func (s *Scanner) asMovie(ctx context.Context, lib *model.Library, parts, prefix []string, stem string, in *storage.SaveMediaInput) error {
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
		LibraryID: lib.ID, SourceKey: key, Title: info.Title, OriginalTitle: info.OriginalTitle, Year: info.Year,
	})
	if err != nil {
		return err
	}
	in.MovieID = &id
	return nil
}

// asEpisode: parts je cesta pod adresářem seriálů, první je adresář seriálu.
func (s *Scanner) asEpisode(ctx context.Context, lib *model.Library, abs string, parts, prefix []string, stem string, in *storage.SaveMediaInput) error {
	key, _ := episodeKey(lib, abs, parts, prefix, stem)
	id, err := s.store.EnsureEpisode(ctx, key)
	if err != nil {
		return err
	}
	in.EpisodeID = &id
	return nil
}

// episodeKey určí seriál, řadu a díl souboru. fromName říká, že číslo dílu
// je v názvu souboru – jinak je to pořadí v adresáři, které se mění.
func episodeKey(lib *model.Library, abs string, parts, prefix []string, stem string) (key storage.EpisodeKey, fromName bool) {
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
	if show.Year == 0 {
		// Rok seriálu často nese jen název dílu ("Seriál.e01.1999.PDTV").
		show.Year = ParseMovie(stem).Year
	}
	showKey := strings.Join(append(append([]string{}, prefix...), showDir), "/")
	if len(parts) < 2 {
		showKey = strings.Join(append(append([]string{}, prefix...), "@"+show.Title), "/")
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

	return storage.EpisodeKey{
		LibraryID: lib.ID, ShowKey: showKey, ShowTitle: show.Title, ShowYear: show.Year,
		Season: season, Episode: number, Title: ep.Title,
	}, ep.OK
}

// reassignEpisode přeřadí známý soubor k jinému dílu, když název souboru
// jednoznačně určuje jiný díl, než ke kterému patří. Opravuje díly očíslované
// podle pořadí v adresáři ve chvíli, kdy ještě nebyly zkopírované všechny.
func (s *Scanner) reassignEpisode(ctx context.Context, lib *model.Library, abs, rel string, in *storage.SaveMediaInput) error {
	parts := strings.Split(rel, "/")
	stem := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(rel))
	key, fromName := episodeKey(lib, abs, parts[1:], parts[:1], stem)
	if !fromName {
		return nil
	}
	cur, err := s.store.GetEpisode(ctx, *in.EpisodeID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if cur.Season == key.Season && cur.Number == key.Episode {
		return nil
	}
	id, err := s.store.EnsureEpisode(ctx, key)
	if err != nil {
		return err
	}
	s.log.Info("soubor přeřazen k jinému dílu", "path", rel,
		"z", fmt.Sprintf("S%02dE%02d", cur.Season, cur.Number), "na", fmt.Sprintf("S%02dE%02d", key.Season, key.Episode))
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
