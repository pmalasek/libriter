package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
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

// matchRetryDays je po kolika dnech se automatické párování zkusí znovu
// u titulu, pro který zdroj nenašel jistý výsledek.
const matchRetryDays = 7

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
	// force = příští běh zkusí i nedávno nepovedené tituly (KickAll).
	force  bool
	appCtx context.Context
	// retry spustí párování znovu po skončení pauzy zdroje (HTTP 429).
	retry *time.Timer

	// Obnova metadat spárovaných titulů (RefreshAll).
	refresh RefreshStatus
}

// RefreshStatus je průběh obnovy metadat spárovaných titulů.
type RefreshStatus struct {
	Running bool `json:"running"`
	Done    int  `json:"done"`
	Total   int  `json:"total"`
	Failed  int  `json:"failed"`
}

// ErrRefreshRunning znamená, že obnova metadat už běží.
var ErrRefreshRunning = errors.New("obnova metadat už běží")

// RefreshStatusNow vrátí průběh obnovy metadat.
func (m *Matcher) RefreshStatusNow() RefreshStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refresh
}

// RefreshAll znovu načte metadata (texty, obrázky, obsazení s fotkami)
// u všech spárovaných titulů ze zdroje, se kterým jsou spárované. Běží na
// pozadí; chyby jednotlivých titulů jen zaloguje. Přepíše ruční úpravy
// názvu a popisu stejně jako ruční přepárování.
func (m *Matcher) RefreshAll() error {
	if !m.Enabled() {
		return ErrMetadataDisabled
	}
	m.mu.Lock()
	if m.refresh.Running {
		m.mu.Unlock()
		return ErrRefreshRunning
	}
	m.refresh = RefreshStatus{Running: true}
	m.mu.Unlock()

	go func() {
		ctx := m.appCtx
		defer func() {
			m.mu.Lock()
			m.refresh.Running = false
			st := m.refresh
			m.mu.Unlock()
			m.log.Info("obnova metadat hotová", "titulů", st.Done, "chyb", st.Failed)
		}()

		type job struct {
			kind, title, provider, metaID, state string
			id                                   uuid.UUID
		}
		var jobs []job
		for _, state := range []string{model.MatchAuto, model.MatchManual} {
			movies, err := m.store.ListMovies(ctx, storage.MovieFilter{MatchState: state, WithoutFiles: true})
			if err != nil {
				m.log.Warn("obnova metadat: filmy", "err", err)
				return
			}
			for _, mv := range movies {
				if mv.MetaProvider != nil && mv.MetaID != nil {
					jobs = append(jobs, job{model.KindMovie, mv.Title, *mv.MetaProvider, *mv.MetaID, state, mv.ID})
				}
			}
			shows, err := m.store.ListShows(ctx, storage.ShowFilter{MatchState: state, WithoutFiles: true})
			if err != nil {
				m.log.Warn("obnova metadat: seriály", "err", err)
				return
			}
			for _, sh := range shows {
				if sh.MetaProvider != nil && sh.MetaID != nil {
					jobs = append(jobs, job{model.KindShow, sh.Title, *sh.MetaProvider, *sh.MetaID, state, sh.ID})
				}
			}
		}
		m.mu.Lock()
		m.refresh.Total = len(jobs)
		m.mu.Unlock()

		for _, j := range jobs {
			var err error
			for {
				if ctx.Err() != nil {
					return
				}
				if j.kind == model.KindMovie {
					_, err = m.MatchMovie(ctx, j.id, j.provider, j.metaID, j.state)
				} else {
					err = m.MatchShow(ctx, j.id, j.provider, j.metaID, j.state)
				}
				// Zdroj odmítá požadavky – počkat na konec pauzy a titul zkusit znovu.
				if !plugin.IsRateLimited(err) || !m.waitPause(ctx, j.provider) {
					break
				}
			}
			m.mu.Lock()
			m.refresh.Done++
			if err != nil {
				m.refresh.Failed++
			}
			m.mu.Unlock()
			if err != nil {
				m.log.Warn("obnova metadat", "title", j.title, "zdroj", j.provider, "err", err)
			}
		}
	}()
	return nil
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
// Tituly, pro které zdroj nedávno nenašel jistý výsledek, vynechá.
func (m *Matcher) Kick() { m.kick(false) }

// KickAll je Kick, který zkusí i nedávno nepovedené tituly (ruční
// „Spárovat vše“ z administrace).
func (m *Matcher) KickAll() { m.kick(true) }

func (m *Matcher) kick(force bool) {
	if !m.Enabled() {
		return
	}
	m.mu.Lock()
	m.force = m.force || force
	if m.running {
		m.again = true
		m.mu.Unlock()
		return
	}
	m.running = true
	force, m.force = m.force, false
	m.mu.Unlock()

	go func() {
		for {
			m.autoMatch(m.appCtx, force)
			m.mu.Lock()
			if !m.again || m.appCtx.Err() != nil {
				m.running = false
				m.mu.Unlock()
				return
			}
			m.again = false
			force, m.force = m.force, false
			m.mu.Unlock()
		}
	}()
}

func (m *Matcher) autoMatch(ctx context.Context, force bool) {
	libs, err := m.store.ListLibraries(ctx, true)
	if err != nil {
		m.log.Warn("knihovny", "err", err)
		return
	}
	limited := map[string]bool{}
	for _, lib := range libs {
		if ctx.Err() != nil {
			return
		}
		provider := ""
		if lib.Provider != nil {
			provider = *lib.Provider
		}
		if !m.supports(provider, lib.Kind) || limited[provider] {
			continue
		}
		retryDays := matchRetryDays
		if force {
			retryDays = 0
		}
		var err error
		if lib.Kind == model.KindShow {
			err = m.autoMatchShows(ctx, lib, provider, retryDays)
		} else {
			err = m.autoMatchMovies(ctx, lib, provider, retryDays)
		}
		// Zdroj odmítá požadavky: dál ho nezatěžovat, zbytek dopáruje
		// další běh po skončení pauzy.
		if plugin.IsRateLimited(err) {
			limited[provider] = true
			m.retryAfterPause(provider, force)
		}
	}
}

// markAttempt zaznamená nepovedené párování – titul se znovu zkusí až po
// matchRetryDays dnech nebo ručním „Spárovat vše“.
func (m *Matcher) markAttempt(ctx context.Context, kind string, id uuid.UUID) {
	if err := m.store.MarkMatchAttempt(ctx, kind, id); err != nil {
		m.log.Warn("záznam pokusu o párování", "id", id, "err", err)
	}
}

// pausedUntil vrátí konec pauzy zdroje po odmítnutí (nulový čas = není).
func (m *Matcher) pausedUntil(provider string) time.Time {
	if p, ok := m.plugins.Get(provider); ok {
		return p.PausedUntil()
	}
	return time.Time{}
}

// waitPause počká na konec pauzy zdroje. False = kontext skončil.
func (m *Matcher) waitPause(ctx context.Context, provider string) bool {
	wait := time.Until(m.pausedUntil(provider))
	if wait <= 0 {
		wait = time.Second
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(wait + time.Second):
		return true
	}
}

// retryAfterPause naplánuje automatické párování na konec pauzy zdroje.
func (m *Matcher) retryAfterPause(provider string, force bool) {
	wait := time.Until(m.pausedUntil(provider)) + time.Second
	m.log.Warn("zdroj odmítá požadavky, párování pokračuje po pauze", "zdroj", provider, "za", wait.Round(time.Second))
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.retry != nil {
		m.retry.Stop()
	}
	m.retry = time.AfterFunc(wait, func() { m.kick(force) })
}

// rateLimited vrátí chybu rate_limited, pokud byl zdroj během párování
// pozastavený – titul se pak neuloží (chyběly by mu obrázky) a spáruje
// se znovu po pauze.
func (m *Matcher) rateLimited(provider string) error {
	if until := m.pausedUntil(provider); !until.IsZero() {
		return &plugin.Error{Plugin: provider, Code: plugin.ErrCodeRateLimited,
			Message: "zdroj odmítá požadavky do " + until.Format("15:04:05")}
	}
	return nil
}

// autoMatchMovies vrací chybu jen tehdy, když zdroj odmítá požadavky.
func (m *Matcher) autoMatchMovies(ctx context.Context, lib model.Library, provider string, retryDays int) error {
	movies, err := m.store.ListMovies(ctx, storage.MovieFilter{Library: lib.ID, MatchState: model.MatchUnmatched, WithoutFiles: true, RetryDays: retryDays})
	if err != nil {
		m.log.Warn("nespárované filmy", "knihovna", lib.Name, "err", err)
		return nil
	}
	for _, mv := range movies {
		if ctx.Err() != nil {
			return nil
		}
		id, ok, err := m.pick(ctx, provider, lib.Language, model.KindMovie, mv.Title, deref(mv.OriginalTitle), derefInt(mv.Year))
		if plugin.IsRateLimited(err) {
			return err
		}
		if !ok {
			m.markAttempt(ctx, model.KindMovie, mv.ID)
			continue
		}
		if _, err := m.MatchMovie(ctx, mv.ID, provider, id, model.MatchAuto); err != nil {
			if plugin.IsRateLimited(err) {
				return err
			}
			m.log.Warn("párování filmu", "title", mv.Title, "zdroj", provider, "err", err)
			m.markAttempt(ctx, model.KindMovie, mv.ID)
		}
	}
	return nil
}

// autoMatchShows vrací chybu jen tehdy, když zdroj odmítá požadavky.
func (m *Matcher) autoMatchShows(ctx context.Context, lib model.Library, provider string, retryDays int) error {
	shows, err := m.store.ListShows(ctx, storage.ShowFilter{Library: lib.ID, MatchState: model.MatchUnmatched, WithoutFiles: true, RetryDays: retryDays})
	if err != nil {
		m.log.Warn("nespárované seriály", "knihovna", lib.Name, "err", err)
		return nil
	}
	for _, sh := range shows {
		if ctx.Err() != nil {
			return nil
		}
		id, ok, err := m.pick(ctx, provider, lib.Language, model.KindShow, sh.Title, deref(sh.OriginalTitle), derefInt(sh.Year))
		if plugin.IsRateLimited(err) {
			return err
		}
		if !ok {
			m.markAttempt(ctx, model.KindShow, sh.ID)
			continue
		}
		if err := m.MatchShow(ctx, sh.ID, provider, id, model.MatchAuto); err != nil {
			if plugin.IsRateLimited(err) {
				return err
			}
			m.log.Warn("párování seriálu", "title", sh.Title, "zdroj", provider, "err", err)
			m.markAttempt(ctx, model.KindShow, sh.ID)
		}
	}
	return nil
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
//
// Chyba hledání se vrací kvůli rate_limited (párování se má zastavit);
// ostatní chyby jen zaloguje.
func (m *Matcher) pick(ctx context.Context, provider, lang, kind, title, original string, year int) (string, bool, error) {
	for _, q := range []string{title, original} {
		if strings.TrimSpace(q) == "" {
			continue
		}
		res, err := m.plugins.Search(ctx, provider, plugin.Request{
			Kind: kind, Language: lang, Query: &plugin.Query{Title: q, OriginalTitle: original},
		})
		if err != nil {
			if !plugin.IsRateLimited(err) {
				m.log.Warn("hledání metadat", "zdroj", provider, "query", q, "err", err)
			}
			return "", false, err
		}
		if id, ok := pickResult(res, title, original, year); ok {
			return id, true, nil
		}
	}
	return "", false, nil
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
	// Bez roku: jediný výsledek se shodným názvem. Ostatní výsledky hledání
	// (jiné tituly se stejným slovem) shodu nezpochybňují.
	var only *plugin.SearchResult
	for i := range res {
		if sameName(res[i]) {
			if only != nil {
				return "", false // dva stejně pojmenované tituly – rozhodne člověk
			}
			only = &res[i]
		}
	}
	if only != nil {
		return only.ID, only.ID != ""
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
	if err := m.rateLimited(provider); err != nil {
		return uuid.Nil, err
	}
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

	// Řady se stáhnou celé dřív, než se cokoli uloží: když zdroj začne
	// odmítat požadavky, seriál zůstane nespárovaný a spáruje se znovu
	// (spárovaný seriál by už další metadata řad nedostal).
	seasons, err := m.store.ListSeasons(ctx, id)
	if err != nil {
		return err
	}
	var seasonMetas []storage.SeasonMeta
	for _, se := range seasons {
		var onDiskNumbers []int
		for _, e := range se.Episodes {
			onDiskNumbers = append(onDiskNumbers, e.Number)
		}
		s, err := m.plugins.Season(ctx, provider, plugin.Request{ID: metaID, Season: se.Number,
			Episodes: onDiskNumbers, Language: lib.Language})
		if plugin.IsNotFound(err) {
			continue
		}
		if plugin.IsRateLimited(err) {
			return err
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
		seasonMetas = append(seasonMetas, sm)
	}
	if err := m.rateLimited(provider); err != nil {
		return err
	}

	if err := m.store.ApplyShowMeta(ctx, id, meta, state); err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return fmt.Errorf("seriál %s/%s už v knihovně je: %w", provider, metaID, err)
		}
		return err
	}
	for _, sm := range seasonMetas {
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
	for _, group := range []struct {
		role   string
		people []plugin.Person
	}{
		{model.RoleDirector, d.Credits.Directors},
		{model.RoleWriter, d.Credits.Writers},
		{model.RoleCreator, d.Credits.Creators},
		{model.RoleCast, d.Credits.Cast},
	} {
		for _, p := range group.people {
			meta.Credits = append(meta.Credits, model.Credit{
				Name: p.Name, Role: group.role, Character: p.Character,
				ProviderID: p.ID, ImagePath: m.personPhoto(ctx, provider, p),
			})
		}
	}
	return meta
}

// personPhoto stáhne fotku osoby, pokud ji zdroj nabízí a osoba ji ještě
// nemá (fotky se nemění, opakované párování je nestahuje znovu). Vrací
// název souboru, nebo prázdný řetězec.
func (m *Matcher) personPhoto(ctx context.Context, provider string, p plugin.Person) string {
	if p.ImageURL == "" || p.ID == "" {
		return ""
	}
	if have, err := m.store.PersonPhoto(ctx, provider, p.ID); err == nil && have != "" {
		return ""
	}
	return m.download(ctx, provider, p.ImageURL, "person-"+provider+"-"+safeName(p.ID))
}

// safeName nechá v ID zdroje jen znaky bezpečné pro název souboru.
func safeName(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' {
			b = append(b, c)
		}
	}
	return string(b)
}

// download stáhne obrázek ze zdroje do SCENITER_IMAGE_ROOT a vrátí název
// souboru; při chybě prázdný řetězec (titul se uloží bez obrázku).
func (m *Matcher) download(ctx context.Context, provider, url, name string) string {
	if url == "" || m.imageRoot == "" {
		return ""
	}
	data, contentType, err := m.plugins.DownloadImage(ctx, provider, url)
	if err != nil {
		// Pauzu zdroje zalogoval hostitel; titul se pak celý zahodí.
		if !plugin.IsRateLimited(err) {
			m.log.Warn("obrázek ze zdroje", "zdroj", provider, "url", url, "err", err)
		}
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
