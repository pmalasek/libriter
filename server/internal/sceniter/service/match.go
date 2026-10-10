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
	"mediater/internal/core/plugin"
	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
)

// ErrMetadataDisabled znamená, že pro titul není k dispozici žádný zdroj
// metadat (knihovna nemá výchozí zdroj, nebo plugin není nainstalovaný).
var ErrMetadataDisabled = errors.New("zdroj metadat není nastaven")

// Matcher páruje filmy a seriály se zdroji metadat (pluginy) a stahuje
// jejich obrázky. Zdroj a jazyk pro automatické párování určuje knihovna.
type Matcher struct {
	store     *storage.Store
	plugins   *plugin.Host
	imageRoot string
	log       *slog.Logger

	mu      sync.Mutex
	running bool
	again   bool
	appCtx  context.Context
}

func NewMatcher(store *storage.Store, plugins *plugin.Host, imageRoot string) *Matcher {
	return &Matcher{store: store, plugins: plugins, imageRoot: imageRoot,
		log: slog.Default().With("component", "sceniter-match"), appCtx: context.Background()}
}

// Start nastaví kontext života aplikace pro párování na pozadí.
func (m *Matcher) Start(ctx context.Context) { m.appCtx = ctx }

// Providers vrátí pluginy, které umí filmy nebo seriály.
func (m *Matcher) Providers() []*plugin.Plugin {
	if m.plugins == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []*plugin.Plugin
	for _, kind := range []string{plugin.KindMovie, plugin.KindShow} {
		for _, p := range m.plugins.ByKind(kind) {
			if !seen[p.Name()] {
				seen[p.Name()] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// Enabled řekne, jestli je nainstalovaný aspoň jeden zdroj video metadat.
func (m *Matcher) Enabled() bool { return len(m.Providers()) > 0 }

// supports řekne, jestli plugin existuje a umí daný druh.
func (m *Matcher) supports(provider, kind string) bool {
	if m.plugins == nil || provider == "" {
		return false
	}
	p, ok := m.plugins.Get(provider)
	return ok && p.Manifest.Supports(kind)
}

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
	libs, err := m.store.ListLibraries(ctx, true)
	if err != nil {
		m.log.Warn("knihovny", "err", err)
		return
	}
	for _, lib := range libs {
		if ctx.Err() != nil {
			return
		}
		provider := ""
		if lib.Provider != nil {
			provider = *lib.Provider
		}
		if !m.supports(provider, lib.Kind) {
			continue
		}
		if lib.Kind == model.KindShow {
			m.autoMatchShows(ctx, lib, provider)
		} else {
			m.autoMatchMovies(ctx, lib, provider)
		}
	}
}

func (m *Matcher) autoMatchMovies(ctx context.Context, lib model.Library, provider string) {
	movies, err := m.store.ListMovies(ctx, storage.MovieFilter{Library: lib.ID, MatchState: model.MatchUnmatched, WithoutFiles: true})
	if err != nil {
		m.log.Warn("nespárované filmy", "knihovna", lib.Name, "err", err)
		return
	}
	for _, mv := range movies {
		if ctx.Err() != nil {
			return
		}
		id, ok := m.pick(ctx, provider, lib.Language, model.KindMovie, mv.Title, deref(mv.OriginalTitle), derefInt(mv.Year))
		if !ok {
			continue
		}
		if _, err := m.MatchMovie(ctx, mv.ID, provider, id, model.MatchAuto); err != nil {
			m.log.Warn("párování filmu", "title", mv.Title, "zdroj", provider, "err", err)
		}
	}
}

func (m *Matcher) autoMatchShows(ctx context.Context, lib model.Library, provider string) {
	shows, err := m.store.ListShows(ctx, storage.ShowFilter{Library: lib.ID, MatchState: model.MatchUnmatched, WithoutFiles: true})
	if err != nil {
		m.log.Warn("nespárované seriály", "knihovna", lib.Name, "err", err)
		return
	}
	for _, sh := range shows {
		if ctx.Err() != nil {
			return
		}
		id, ok := m.pick(ctx, provider, lib.Language, model.KindShow, sh.Title, deref(sh.OriginalTitle), derefInt(sh.Year))
		if !ok {
			continue
		}
		if err := m.MatchShow(ctx, sh.ID, provider, id, model.MatchAuto); err != nil {
			m.log.Warn("párování seriálu", "title", sh.Title, "zdroj", provider, "err", err)
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// pick vybere výsledek hledání, kterému lze věřit bez člověka: shoda roku
// ±1 a názvu, nebo jediný výsledek se shodným názvem. Jinak nic – titul
// zůstane nespárovaný a editor ho spáruje ručně.
func (m *Matcher) pick(ctx context.Context, provider, lang, kind, title, original string, year int) (string, bool) {
	for _, q := range []string{title, original} {
		if strings.TrimSpace(q) == "" {
			continue
		}
		res, err := m.plugins.Search(ctx, provider, plugin.Request{
			Kind: kind, Language: lang, Query: &plugin.Query{Title: q, OriginalTitle: original},
		})
		if err != nil {
			m.log.Warn("hledání metadat", "zdroj", provider, "query", q, "err", err)
			return "", false
		}
		if id, ok := pickResult(res, title, original, year); ok {
			return id, true
		}
	}
	return "", false
}

// pickResult je čistá část výběru (testovatelná bez sítě).
func pickResult(res []plugin.SearchResult, title, original string, year int) (string, bool) {
	names := []string{fold(title), fold(original)}
	sameName := func(r plugin.SearchResult) bool {
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
				return r.ID, r.ID != ""
			}
		}
		// Rok sedí přesně a výsledek je první (nejrelevantnější) – název se
		// může lišit diakritikou nebo podtitulem.
		if len(res) > 0 && res[0].Year == year {
			return res[0].ID, res[0].ID != ""
		}
		return "", false
	}
	if len(res) == 1 && sameName(res[0]) {
		return res[0].ID, res[0].ID != ""
	}
	return "", false
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

// Search hledá u zdroje pro ruční párování. Prázdný provider = výchozí
// zdroj knihovny, ve které titul leží; jazyk je vždy jazyk knihovny
// (lang ho může přebít).
func (m *Matcher) Search(ctx context.Context, provider, kind, lang, query string, year int) ([]plugin.SearchResult, error) {
	if !m.supports(provider, kind) {
		return nil, ErrMetadataDisabled
	}
	res, err := m.plugins.Search(ctx, provider, plugin.Request{
		Kind: kind, Language: lang, Query: &plugin.Query{Title: query, Year: year},
	})
	if res == nil {
		res = []plugin.SearchResult{}
	}
	return res, err
}

// LibraryOf vrátí knihovnu titulu.
func (m *Matcher) LibraryOf(ctx context.Context, kind string, id uuid.UUID) (*model.Library, error) {
	var libID uuid.UUID
	if kind == model.KindShow {
		sh, err := m.store.GetShow(ctx, id)
		if err != nil {
			return nil, err
		}
		libID = sh.LibraryID
	} else {
		mv, err := m.store.GetMovie(ctx, id)
		if err != nil {
			return nil, err
		}
		libID = mv.LibraryID
	}
	return m.store.GetLibrary(ctx, libID)
}

// MatchMovie spáruje film s titulem metaID u zdroje provider a uloží metadata
// i obrázky. Vrací ID filmu (jiné, když se sloučil s jinou verzí téhož filmu).
func (m *Matcher) MatchMovie(ctx context.Context, id uuid.UUID, provider, metaID string, state string) (uuid.UUID, error) {
	if !m.supports(provider, model.KindMovie) {
		return uuid.Nil, ErrMetadataDisabled
	}
	lib, err := m.LibraryOf(ctx, model.KindMovie, id)
	if err != nil {
		return uuid.Nil, err
	}
	var d plugin.VideoMetadata
	if err := m.plugins.Fetch(ctx, provider, plugin.Request{Kind: plugin.KindMovie, ID: metaID, Language: lib.Language}, &d); err != nil {
		return uuid.Nil, err
	}
	meta := m.titleMeta(ctx, provider, metaID, &d, "movie-"+id.String())
	return m.store.ApplyMovieMeta(ctx, id, meta, state)
}

// MatchShow spáruje seriál včetně metadat řad a dílů, které jsou na disku.
func (m *Matcher) MatchShow(ctx context.Context, id uuid.UUID, provider, metaID string, state string) error {
	if !m.supports(provider, model.KindShow) {
		return ErrMetadataDisabled
	}
	lib, err := m.LibraryOf(ctx, model.KindShow, id)
	if err != nil {
		return err
	}
	var d plugin.VideoMetadata
	if err := m.plugins.Fetch(ctx, provider, plugin.Request{Kind: plugin.KindShow, ID: metaID, Language: lib.Language}, &d); err != nil {
		return err
	}
	meta := m.titleMeta(ctx, provider, metaID, &d, "show-"+id.String())
	if err := m.store.ApplyShowMeta(ctx, id, meta, state); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return fmt.Errorf("seriál %s/%s už v knihovně je: %w", provider, metaID, err)
		}
		return err
	}

	seasons, err := m.store.ListSeasons(ctx, id)
	if err != nil {
		return err
	}
	for _, se := range seasons {
		s, err := m.plugins.Season(ctx, provider, plugin.Request{ID: metaID, Season: se.Number, Language: lib.Language})
		if plugin.IsNotFound(err) {
			continue
		}
		if err != nil {
			m.log.Warn("řada ze zdroje", "zdroj", provider, "show", id, "season", se.Number, "err", err)
			continue
		}
		sm := storage.SeasonMeta{Number: se.Number, Title: s.Name, Overview: s.Overview,
			PosterPath: m.download(ctx, provider, s.PosterURL, fmt.Sprintf("season-%s-s%02d", id, se.Number))}
		onDisk := map[int]bool{}
		for _, e := range se.Episodes {
			onDisk[e.Number] = true
		}
		for _, e := range s.Episodes {
			em := storage.EpisodeMeta{Number: e.Number, Title: e.Title, Overview: e.Overview,
				AirDate: e.AirDate, RuntimeMin: e.RuntimeMin}
			if onDisk[e.Number] {
				em.StillPath = m.download(ctx, provider, e.StillURL, fmt.Sprintf("still-%s-s%02de%02d", id, se.Number, e.Number))
			}
			sm.Episodes = append(sm.Episodes, em)
		}
		if err := m.store.ApplySeasonMeta(ctx, id, sm); err != nil {
			return err
		}
	}
	return nil
}

func (m *Matcher) titleMeta(ctx context.Context, provider, metaID string, d *plugin.VideoMetadata, imagePrefix string) storage.TitleMeta {
	ids := map[string]string{}
	for k, v := range d.ExternalIDs {
		ids[k] = v
	}
	ids[provider] = metaID
	meta := storage.TitleMeta{
		Provider: provider, ID: metaID, ExternalIDs: ids,
		Title: d.Title, OriginalTitle: d.OriginalTitle,
		Overview: d.Overview, Tagline: d.Tagline, Status: d.Status, Year: d.Year,
		RuntimeMin: d.RuntimeMin, Rating: d.Rating,
		PosterPath:   m.download(ctx, provider, d.PosterURL, "poster-"+imagePrefix),
		BackdropPath: m.download(ctx, provider, d.BackdropURL, "backdrop-"+imagePrefix),
		Genres:       append([]string{}, d.Genres...),
	}
	for _, n := range d.Credits.Directors {
		meta.Credits = append(meta.Credits, model.Credit{Name: n, Role: "director"})
	}
	for _, n := range d.Credits.Writers {
		meta.Credits = append(meta.Credits, model.Credit{Name: n, Role: "writer"})
	}
	for _, n := range d.Credits.Creators {
		meta.Credits = append(meta.Credits, model.Credit{Name: n, Role: "creator"})
	}
	for _, p := range d.Credits.Cast {
		meta.Credits = append(meta.Credits, model.Credit{Name: p.Name, Role: "cast", Character: p.Character})
	}
	return meta
}

// download stáhne obrázek ze zdroje do SCENITER_IMAGE_ROOT a vrátí název
// souboru; při chybě prázdný řetězec (titul se uloží bez obrázku).
func (m *Matcher) download(ctx context.Context, provider, url, name string) string {
	if url == "" || m.imageRoot == "" {
		return ""
	}
	data, contentType, err := m.plugins.DownloadImage(ctx, provider, url)
	if err != nil {
		m.log.Warn("obrázek ze zdroje", "zdroj", provider, "url", url, "err", err)
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
