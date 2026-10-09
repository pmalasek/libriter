package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"mediater/internal/core/imagestore"
	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
	"mediater/internal/sceniter/tmdb"
)

// ErrMetadataDisabled znamená, že TMDB_API_KEY není nastaven.
var ErrMetadataDisabled = errors.New("metadata z TMDB nejsou nastavena")

// Matcher páruje filmy a seriály s TMDB a stahuje jejich obrázky.
type Matcher struct {
	store     *storage.Store
	tmdb      *tmdb.Client
	imageRoot string
	log       *slog.Logger

	mu      sync.Mutex
	running bool
	again   bool
	appCtx  context.Context
}

func NewMatcher(store *storage.Store, client *tmdb.Client, imageRoot string) *Matcher {
	return &Matcher{store: store, tmdb: client, imageRoot: imageRoot,
		log: slog.Default().With("component", "sceniter-match"), appCtx: context.Background()}
}

// Start nastaví kontext života aplikace pro párování na pozadí.
func (m *Matcher) Start(ctx context.Context) { m.appCtx = ctx }

// Enabled řekne, jestli je TMDB nastavené.
func (m *Matcher) Enabled() bool { return m.tmdb.Enabled() }

// Kick spustí automatické párování nespárovaných titulů na pozadí. Když už
// běží, proběhne po jeho skončení ještě jednou (mezitím přibyly soubory).
func (m *Matcher) Kick() {
	if !m.Enabled() {
		return
	}
	m.mu.Lock()
	if m.running {
		m.again = true
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go func() {
		for {
			m.autoMatch(m.appCtx)
			m.mu.Lock()
			if !m.again || m.appCtx.Err() != nil {
				m.running = false
				m.mu.Unlock()
				return
			}
			m.again = false
			m.mu.Unlock()
		}
	}()
}

func (m *Matcher) autoMatch(ctx context.Context) {
	movies, err := m.store.ListMovies(ctx, storage.MovieFilter{MatchState: model.MatchUnmatched, WithoutFiles: true})
	if err != nil {
		m.log.Warn("nespárované filmy", "err", err)
		return
	}
	for _, mv := range movies {
		if ctx.Err() != nil {
			return
		}
		year := 0
		if mv.Year != nil {
			year = *mv.Year
		}
		orig := ""
		if mv.OriginalTitle != nil {
			orig = *mv.OriginalTitle
		}
		id, ok := m.pick(ctx, model.KindMovie, mv.Title, orig, year)
		if !ok {
			continue
		}
		if _, err := m.MatchMovie(ctx, mv.ID, id, model.MatchAuto); err != nil {
			m.log.Warn("párování filmu", "title", mv.Title, "err", err)
		}
	}

	shows, err := m.store.ListShows(ctx, storage.ShowFilter{MatchState: model.MatchUnmatched, WithoutFiles: true})
	if err != nil {
		m.log.Warn("nespárované seriály", "err", err)
		return
	}
	for _, sh := range shows {
		if ctx.Err() != nil {
			return
		}
		year := 0
		if sh.Year != nil {
			year = *sh.Year
		}
		id, ok := m.pick(ctx, model.KindShow, sh.Title, "", year)
		if !ok {
			continue
		}
		if err := m.MatchShow(ctx, sh.ID, id, model.MatchAuto); err != nil {
			m.log.Warn("párování seriálu", "title", sh.Title, "err", err)
		}
	}
}

// pick vybere výsledek hledání, kterému lze věřit bez člověka: shoda roku
// ±1 a názvu, nebo jediný výsledek se shodným názvem. Jinak nic – titul
// zůstane nespárovaný a admin ho spáruje ručně.
func (m *Matcher) pick(ctx context.Context, kind, title, original string, year int) (int, bool) {
	for _, q := range []string{title, original} {
		if strings.TrimSpace(q) == "" {
			continue
		}
		var (
			res []tmdb.SearchResult
			err error
		)
		if kind == model.KindShow {
			res, err = m.tmdb.SearchTV(ctx, q, 0)
		} else {
			res, err = m.tmdb.SearchMovie(ctx, q, 0)
		}
		if err != nil {
			m.log.Warn("hledání v TMDB", "query", q, "err", err)
			return 0, false
		}
		if id, ok := pickResult(res, title, original, year); ok {
			return id, true
		}
	}
	return 0, false
}

// pickResult je čistá část výběru (testovatelná bez sítě).
func pickResult(res []tmdb.SearchResult, title, original string, year int) (int, bool) {
	names := []string{fold(title), fold(original)}
	sameName := func(r tmdb.SearchResult) bool {
		for _, n := range names {
			if n != "" && (n == fold(r.Title) || n == fold(r.OriginalTitle)) {
				return true
			}
		}
		return false
	}
	if year > 0 {
		for _, r := range res {
			if r.Year != 0 && abs(r.Year-year) <= 1 && sameName(r) {
				return r.ID, true
			}
		}
		// Rok sedí přesně a výsledek je první (nejrelevantnější) – název se
		// může lišit diakritikou nebo podtitulem.
		if len(res) > 0 && res[0].Year == year {
			return res[0].ID, true
		}
		return 0, false
	}
	if len(res) == 1 && sameName(res[0]) {
		return res[0].ID, true
	}
	return 0, false
}

// fold převede název na tvar pro porovnání: bez diakritiky, interpunkce a
// rozdílů ve velikosti písmen.
func fold(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, _ := transform.String(t, strings.ToLower(s))
	var b strings.Builder
	for _, r := range out {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Search hledá v TMDB pro ruční párování.
func (m *Matcher) Search(ctx context.Context, kind, query string, year int) ([]tmdb.SearchResult, error) {
	if !m.Enabled() {
		return nil, ErrMetadataDisabled
	}
	if kind == model.KindShow {
		return m.tmdb.SearchTV(ctx, query, year)
	}
	return m.tmdb.SearchMovie(ctx, query, year)
}

// MatchMovie spáruje film s TMDB a uloží metadata i obrázky. Vrací ID filmu
// (jiné, když se sloučil s jinou verzí téhož filmu).
func (m *Matcher) MatchMovie(ctx context.Context, id uuid.UUID, tmdbID int, state string) (uuid.UUID, error) {
	if !m.Enabled() {
		return uuid.Nil, ErrMetadataDisabled
	}
	if _, err := m.store.GetMovie(ctx, id); err != nil {
		return uuid.Nil, err
	}
	d, err := m.tmdb.Movie(ctx, tmdbID)
	if err != nil {
		return uuid.Nil, err
	}
	meta := m.titleMeta(ctx, d, "movie-"+id.String())
	return m.store.ApplyMovieMeta(ctx, id, meta, state)
}

// MatchShow spáruje seriál s TMDB včetně metadat řad a dílů, které jsou na disku.
func (m *Matcher) MatchShow(ctx context.Context, id uuid.UUID, tmdbID int, state string) error {
	if !m.Enabled() {
		return ErrMetadataDisabled
	}
	if _, err := m.store.GetShow(ctx, id); err != nil {
		return err
	}
	d, err := m.tmdb.TV(ctx, tmdbID)
	if err != nil {
		return err
	}
	meta := m.titleMeta(ctx, d, "show-"+id.String())
	for _, c := range d.Creators {
		meta.Credits = append(meta.Credits, model.Credit{Name: c, Role: "creator"})
	}
	if err := m.store.ApplyShowMeta(ctx, id, meta, state); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return fmt.Errorf("seriál s TMDB ID %d už v knihovně je: %w", tmdbID, err)
		}
		return err
	}

	seasons, err := m.store.ListSeasons(ctx, id)
	if err != nil {
		return err
	}
	for _, se := range seasons {
		s, err := m.tmdb.Season(ctx, tmdbID, se.Number)
		if errors.Is(err, tmdb.ErrNotFound) {
			continue
		}
		if err != nil {
			m.log.Warn("řada z TMDB", "show", id, "season", se.Number, "err", err)
			continue
		}
		sm := storage.SeasonMeta{Number: se.Number, Title: s.Name, Overview: s.Overview,
			PosterPath: m.download(ctx, tmdb.SizePoster, s.PosterPath, fmt.Sprintf("season-%s-s%02d", id, se.Number))}
		onDisk := map[int]bool{}
		for _, e := range se.Episodes {
			onDisk[e.Number] = true
		}
		for _, e := range s.Episodes {
			em := storage.EpisodeMeta{Number: e.Number, Title: e.Name, Overview: e.Overview,
				AirDate: e.AirDate, RuntimeMin: e.Runtime}
			if onDisk[e.Number] {
				em.StillPath = m.download(ctx, tmdb.SizeStill, e.StillPath, fmt.Sprintf("still-%s-s%02de%02d", id, se.Number, e.Number))
			}
			sm.Episodes = append(sm.Episodes, em)
		}
		if err := m.store.ApplySeasonMeta(ctx, id, sm); err != nil {
			return err
		}
	}
	return nil
}

func (m *Matcher) titleMeta(ctx context.Context, d *tmdb.Details, imagePrefix string) storage.TitleMeta {
	meta := storage.TitleMeta{
		TMDBID: d.ID, IMDBID: d.IMDBID, Title: d.Title, OriginalTitle: d.OriginalTitle,
		Overview: d.Overview, Tagline: d.Tagline, Status: d.Status, Year: d.Year,
		RuntimeMin: d.RuntimeMin, Rating: d.Rating,
		PosterPath:   m.download(ctx, tmdb.SizePoster, d.PosterPath, "poster-"+imagePrefix),
		BackdropPath: m.download(ctx, tmdb.SizeBackdrop, d.BackdropPath, "backdrop-"+imagePrefix),
		Genres:       []storage.Genre{},
	}
	for _, g := range d.Genres {
		meta.Genres = append(meta.Genres, storage.Genre{TMDBID: g.ID, Name: g.Name})
	}
	for _, n := range d.Directors {
		meta.Credits = append(meta.Credits, model.Credit{Name: n, Role: "director"})
	}
	for _, n := range d.Writers {
		meta.Credits = append(meta.Credits, model.Credit{Name: n, Role: "writer"})
	}
	for _, p := range d.Cast {
		meta.Credits = append(meta.Credits, model.Credit{Name: p.Name, Role: "cast", Character: p.Character})
	}
	return meta
}

// download stáhne obrázek do SCENITER_IMAGE_ROOT a vrátí název souboru;
// při chybě prázdný řetězec (titul se uloží bez obrázku).
func (m *Matcher) download(ctx context.Context, size, path, name string) string {
	if path == "" || m.imageRoot == "" {
		return ""
	}
	data, contentType, err := m.tmdb.Image(ctx, size, path)
	if err != nil {
		m.log.Warn("obrázek z TMDB", "path", path, "err", err)
		return ""
	}
	ext := imagestore.ExtForContentType(contentType)
	if ext == "" {
		ext = imagestore.SniffExt(data)
	}
	file, err := imagestore.Write(m.imageRoot, name, data, ext)
	if err != nil {
		m.log.Warn("uložení obrázku", "name", name, "err", err)
		return ""
	}
	return file
}
