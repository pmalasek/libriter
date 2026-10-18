package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"mediater/internal/core/textfold"
	"mediater/internal/sceniter/model"
)

const movieCols = `
	m.id, m.library_id, m.source_key, m.title, m.original_title, m.year, m.overview, m.tagline, m.runtime_min,
	m.meta_provider, m.meta_id, m.external_ids_json, m.poster_path, m.backdrop_path, m.rating,
	m.match_state, m.match_attempted_at, m.created_at, m.updated_at, m.poster_source, m.backdrop_source,
	m.aspect_ratio`

// mainMovieFile vybere hlavní soubor filmu: největší (nejlepší verzi).
const mainMovieFile = `(SELECT id FROM sc_media_files WHERE movie_id = m.id ORDER BY size_bytes DESC, file_path LIMIT 1)`

func scanMovie(sc interface{ Scan(...any) error }, extra ...any) (model.Movie, error) {
	var (
		m                                   model.Movie
		orig, overview, tagline, poster, bd sql.NullString
		provider, metaID, external, aspect  sql.NullString
		year, runtime, rating               sql.NullInt64
		attempted                           sql.NullTime
	)
	dest := []any{&m.ID, &m.LibraryID, &m.SourceKey, &m.Title, &orig, &year, &overview, &tagline, &runtime,
		&provider, &metaID, &external, &poster, &bd, &rating, &m.MatchState, &attempted, &m.CreatedAt, &m.UpdatedAt,
		&m.PosterSource, &m.BackdropSource, &aspect}
	if err := sc.Scan(append(dest, extra...)...); err != nil {
		return m, err
	}
	m.OriginalTitle, m.Year, m.Overview, m.Tagline = ptrString(orig), ptrInt(year), ptrString(overview), ptrString(tagline)
	m.RuntimeMin, m.MetaProvider, m.MetaID = ptrInt(runtime), ptrString(provider), ptrString(metaID)
	m.ExternalIDs = model.UnmarshalExternalIDs(ptrString(external))
	m.PosterPath, m.BackdropPath, m.Rating = ptrString(poster), ptrString(bd), ptrInt(rating)
	m.AspectRatio = ptrString(aspect)
	if attempted.Valid {
		m.MatchAttemptedAt = &attempted.Time
	}
	m.Credits = []model.Credit{}
	m.Genres = []string{}
	return m, nil
}

// MovieFilter omezuje výpis filmů.
type MovieFilter struct {
	// Library omezí výpis na jednu knihovnu (uuid.Nil = všechny zapnuté).
	Library uuid.UUID
	Query   string // podřetězec názvu nebo původního názvu (bez ohledu na diakritiku)
	Genre   string
	Sort    string // title | year | added | rating
	Dir     string // asc | desc; prázdné = přirozený směr klíče (viz titleOrder)
	Limit   int    // 0 = bez omezení
	Offset  int
	// WithoutFiles zahrne i filmy bez souborů – pro administraci párování.
	WithoutFiles bool
	MatchState   string
	// RetryDays vynechá tituly, jejichž automatické párování se nepovedlo
	// před méně než RetryDays dny (0 = nevynechávat).
	RetryDays int
	// Attempted rozdělí nespárované tituly: true = párování se nepovedlo,
	// false = ještě se nezkoušelo, nil = obojí.
	Attempted *bool
}

// titleWhere složí podmínky výpisu filmů nebo seriálů (alias tabulky t).
// Podmínka „má soubor“ je v WHERE, ne v JOINu, aby SQLite mohl procházet
// titulky v pořadí indexu a nemusel třídit celou knihovnu.
func titleWhere(kind string, library uuid.UUID, query, genre, matchState string, retryDays int, attempted *bool, withoutFiles bool) (string, []any) {
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("?%d", len(args))
	}
	conds := []string{"l.enabled = 1"}
	if library != uuid.Nil {
		conds = append(conds, "t.library_id = "+add(library))
	}
	if !withoutFiles {
		if kind == "show" {
			conds = append(conds, `EXISTS (SELECT 1 FROM sc_seasons se JOIN sc_episodes e ON e.season_id = se.id
				JOIN sc_media_files mf ON mf.episode_id = e.id WHERE se.show_id = t.id)`)
		} else {
			conds = append(conds, "EXISTS (SELECT 1 FROM sc_media_files f WHERE f.movie_id = t.id)")
		}
	}
	if query != "" {
		p := add(textfold.LikePattern(query))
		conds = append(conds, fmt.Sprintf(`(fold(t.title) LIKE %[1]s ESCAPE '\' OR fold(t.original_title) LIKE %[1]s ESCAPE '\')`, p))
	}
	if genre != "" {
		table, col := "sc_movie_genres", "movie_id"
		if kind == "show" {
			table, col = "sc_show_genres", "show_id"
		}
		conds = append(conds, fmt.Sprintf(`EXISTS (SELECT 1 FROM %s x JOIN sc_genres g ON g.id = x.genre_id
			WHERE x.%s = t.id AND g.name = %s)`, table, col, add(genre)))
	}
	if matchState != "" {
		conds = append(conds, "t.match_state = "+add(matchState))
	}
	if retryDays > 0 {
		conds = append(conds, fmt.Sprintf(`(t.match_attempted_at IS NULL OR t.match_attempted_at < datetime('now', %s))`,
			add(fmt.Sprintf("-%d days", retryDays))))
	}
	if attempted != nil {
		if *attempted {
			conds = append(conds, "t.match_attempted_at IS NOT NULL")
		} else {
			conds = append(conds, "t.match_attempted_at IS NULL")
		}
	}
	return strings.Join(conds, " AND "), args
}

// titleOrder je řazení výpisu. Bez směru má každý klíč svůj přirozený
// (název vzestupně, ostatní od nejnovějšího / nejlepšího). Tituly bez roku
// nebo hodnocení jdou vždy na konec; při shodě rozhoduje název vzestupně
// a nakonec jedinečné id, aby stránky na sebe navazovaly.
//
// added je výraz pro klíč „added“ – u filmů datum přidání titulu, u seriálů
// datum přidání nejnovějšího dílu (showLatestEpisode).
func titleOrder(sort, dir, added string) string {
	desc := sort != "" && sort != "title"
	switch dir {
	case "asc":
		desc = false
	case "desc":
		desc = true
	}
	d := ""
	if desc {
		d = " DESC"
	}
	switch sort {
	case "year":
		return ` ORDER BY t.year IS NULL, t.year` + d + `, t.title, t.id`
	case "added":
		return ` ORDER BY ` + added + d + `, t.title, t.id`
	case "rating":
		return ` ORDER BY t.rating IS NULL, t.rating` + d + `, t.title, t.id`
	default:
		return ` ORDER BY t.title` + d + `, t.id` + d
	}
}

func limitOffset(limit, offset int) string {
	if limit <= 0 {
		return ""
	}
	return fmt.Sprintf(` LIMIT %d OFFSET %d`, limit, max(offset, 0))
}

func (f MovieFilter) where() (string, []any) {
	return titleWhere("movie", f.Library, f.Query, f.Genre, f.MatchState, f.RetryDays, f.Attempted, f.WithoutFiles)
}

// CountMovies vrátí počet filmů odpovídajících filtru (Limit a Offset se nepoužijí).
func (s *Store) CountMovies(ctx context.Context, f MovieFilter) (int, error) {
	where, args := f.where()
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sc_movies t JOIN sc_libraries l ON l.id = t.library_id WHERE `+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count movies: %w", err)
	}
	return n, nil
}

// ListMovies vrátí filmy, které mají aspoň jeden soubor.
func (s *Store) ListMovies(ctx context.Context, f MovieFilter) ([]model.Movie, error) {
	where, args := f.where()
	// Hlavní soubor se dohledá až pro řádky na stránce.
	q := `SELECT ` + strings.ReplaceAll(movieCols, "m.", "t.") + `,
		(SELECT id FROM sc_media_files WHERE movie_id = t.id ORDER BY size_bytes DESC, file_path LIMIT 1),
		(SELECT duration_sec FROM sc_media_files WHERE movie_id = t.id ORDER BY size_bytes DESC, file_path LIMIT 1)
		FROM sc_movies t JOIN sc_libraries l ON l.id = t.library_id
		WHERE ` + where + titleOrder(f.Sort, f.Dir, "t.created_at") + limitOffset(f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	defer rows.Close()

	out := []model.Movie{}
	for rows.Next() {
		var (
			mediaID  *uuid.UUID
			duration sql.NullFloat64
		)
		m, err := scanMovie(rows, &mediaID, &duration)
		if err != nil {
			return nil, fmt.Errorf("list movies: %w", err)
		}
		m.MediaID, m.DurationSec = mediaID, ptrFloat(duration)
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	genres, err := s.genresByID(ctx, "movie", ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if g := genres[out[i].ID]; g != nil {
			out[i].Genres = g
		}
	}
	return out, nil
}

// GetMovie vrátí film podle ID (i bez souborů).
func (s *Store) GetMovie(ctx context.Context, id uuid.UUID) (*model.Movie, error) {
	q := `SELECT ` + movieCols + `, mf.id, mf.duration_sec FROM sc_movies m
		LEFT JOIN sc_media_files mf ON mf.id = ` + mainMovieFile + ` WHERE m.id = ?1`
	var (
		mediaID  *uuid.UUID
		duration sql.NullFloat64
	)
	m, err := scanMovie(s.db.QueryRowContext(ctx, q, id), &mediaID, &duration)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get movie: %w", err)
	}
	m.MediaID, m.DurationSec = mediaID, ptrFloat(duration)
	genres, err := s.genresByID(ctx, "movie", []uuid.UUID{m.ID})
	if err != nil {
		return nil, err
	}
	if g := genres[m.ID]; g != nil {
		m.Genres = g
	}
	if m.Credits, err = s.creditsByTitle(ctx, model.KindMovie, m.ID); err != nil {
		return nil, err
	}
	return &m, nil
}

// genresByID vrátí žánry zadaných filmů (kind "movie") nebo seriálů ("show").
func (s *Store) genresByID(ctx context.Context, kind string, ids []uuid.UUID) (map[uuid.UUID][]string, error) {
	byID := map[uuid.UUID][]string{}
	if len(ids) == 0 {
		return byID, nil
	}
	table, col := "sc_movie_genres", "movie_id"
	if kind == "show" {
		table, col = "sc_show_genres", "show_id"
	}
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		marks[i] = fmt.Sprintf("?%d", i+1)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT x.`+col+`, g.name FROM `+table+` x JOIN sc_genres g ON g.id = x.genre_id
		WHERE x.`+col+` IN (`+strings.Join(marks, ", ")+`) ORDER BY g.name`, args...)
	if err != nil {
		return nil, fmt.Errorf("genres: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id   uuid.UUID
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		byID[id] = append(byID[id], name)
	}
	return byID, rows.Err()
}

const showCols = `
	s.id, s.library_id, s.source_key, s.title, s.original_title, s.year, s.overview, s.status,
	s.meta_provider, s.meta_id, s.external_ids_json,
	s.poster_path, s.backdrop_path, s.rating, s.match_state, s.match_attempted_at, s.created_at, s.updated_at,
	s.poster_source, s.backdrop_source, s.aspect_ratio`

// showLatestEpisode je datum přidání nejnovějšího dílu se souborem;
// seriál bez dílů má datum přidání seriálu. Podle něj se řadí „added“.
const showLatestEpisode = `
	COALESCE((SELECT max(e.created_at) FROM sc_seasons se JOIN sc_episodes e ON e.season_id = se.id
	   WHERE se.show_id = s.id AND EXISTS (SELECT 1 FROM sc_media_files f WHERE f.episode_id = e.id)), s.created_at)`

// showCounts spočítá řady a díly, které mají soubor, a datum nejnovějšího dílu.
const showCounts = `
	(SELECT count(DISTINCT se.id) FROM sc_seasons se JOIN sc_episodes e ON e.season_id = se.id
	   WHERE se.show_id = s.id AND EXISTS (SELECT 1 FROM sc_media_files f WHERE f.episode_id = e.id)),
	(SELECT count(*) FROM sc_seasons se JOIN sc_episodes e ON e.season_id = se.id
	   WHERE se.show_id = s.id AND EXISTS (SELECT 1 FROM sc_media_files f WHERE f.episode_id = e.id)),` + showLatestEpisode

// parseDBTime přečte čas z výrazu, který driver vrátí jako text
// (sloupec bez deklarovaného typu, např. max(created_at)).
func parseDBTime(v string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02T15:04:05.999999999",
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("neznámý formát času %q", v)
}

func scanShow(sc interface{ Scan(...any) error }, extra ...any) (model.Show, error) {
	var (
		sh                                 model.Show
		orig, overview, status, poster, bd sql.NullString
		provider, metaID, external, aspect sql.NullString
		year, rating                       sql.NullInt64
		attempted                          sql.NullTime
		latest                             string
	)
	dest := []any{&sh.ID, &sh.LibraryID, &sh.SourceKey, &sh.Title, &orig, &year, &overview, &status,
		&provider, &metaID, &external,
		&poster, &bd, &rating, &sh.MatchState, &attempted, &sh.CreatedAt, &sh.UpdatedAt,
		&sh.PosterSource, &sh.BackdropSource, &aspect,
		&sh.SeasonCount, &sh.EpisodeCount, &latest}
	err := sc.Scan(append(dest, extra...)...)
	if err != nil {
		return sh, err
	}
	if sh.LatestEpisodeAt, err = parseDBTime(latest); err != nil {
		return sh, err
	}
	sh.OriginalTitle, sh.Year, sh.Overview, sh.Status = ptrString(orig), ptrInt(year), ptrString(overview), ptrString(status)
	sh.MetaProvider, sh.MetaID = ptrString(provider), ptrString(metaID)
	sh.ExternalIDs = model.UnmarshalExternalIDs(ptrString(external))
	sh.PosterPath, sh.BackdropPath, sh.Rating = ptrString(poster), ptrString(bd), ptrInt(rating)
	sh.AspectRatio = ptrString(aspect)
	if attempted.Valid {
		sh.MatchAttemptedAt = &attempted.Time
	}
	sh.Credits = []model.Credit{}
	sh.Genres = []string{}
	return sh, nil
}

// ShowFilter omezuje výpis seriálů.
type ShowFilter struct {
	Library      uuid.UUID
	Query        string
	Genre        string
	Sort         string // title | year | added | rating
	Dir          string // asc | desc; prázdné = přirozený směr klíče (viz titleOrder)
	Limit        int
	Offset       int
	WithoutFiles bool
	MatchState   string
	RetryDays    int   // viz MovieFilter.RetryDays
	Attempted    *bool // viz MovieFilter.Attempted
}

func (f ShowFilter) where() (string, []any) {
	return titleWhere("show", f.Library, f.Query, f.Genre, f.MatchState, f.RetryDays, f.Attempted, f.WithoutFiles)
}

// CountShows vrátí počet seriálů odpovídajících filtru (Limit a Offset se nepoužijí).
func (s *Store) CountShows(ctx context.Context, f ShowFilter) (int, error) {
	where, args := f.where()
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sc_shows t JOIN sc_libraries l ON l.id = t.library_id WHERE `+where, args...).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count shows: %w", err)
	}
	return n, nil
}

// ListShows vrátí seriály, které mají aspoň jeden díl se souborem.
func (s *Store) ListShows(ctx context.Context, f ShowFilter) ([]model.Show, error) {
	where, args := f.where()
	alias := strings.NewReplacer("= s.id", "= t.id", "s.created_at", "t.created_at")
	cols := strings.ReplaceAll(showCols, "s.", "t.") + `, ` + alias.Replace(showCounts)
	latest := alias.Replace(showLatestEpisode)
	q := `SELECT ` + cols + ` FROM sc_shows t JOIN sc_libraries l ON l.id = t.library_id
		WHERE ` + where + titleOrder(f.Sort, f.Dir, latest) + limitOffset(f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list shows: %w", err)
	}
	defer rows.Close()
	out := []model.Show{}
	for rows.Next() {
		sh, err := scanShow(rows)
		if err != nil {
			return nil, fmt.Errorf("list shows: %w", err)
		}
		out = append(out, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	genres, err := s.genresByID(ctx, "show", ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if g := genres[out[i].ID]; g != nil {
			out[i].Genres = g
		}
	}
	return out, nil
}

// GetShow vrátí seriál podle ID.
func (s *Store) GetShow(ctx context.Context, id uuid.UUID) (*model.Show, error) {
	q := `SELECT ` + showCols + `, ` + showCounts + ` FROM sc_shows s WHERE s.id = ?1`
	sh, err := scanShow(s.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get show: %w", err)
	}
	genres, err := s.genresByID(ctx, "show", []uuid.UUID{sh.ID})
	if err != nil {
		return nil, err
	}
	if g := genres[sh.ID]; g != nil {
		sh.Genres = g
	}
	if sh.Credits, err = s.creditsByTitle(ctx, model.KindShow, sh.ID); err != nil {
		return nil, err
	}
	return &sh, nil
}

const episodeCols = `
	e.id, e.season_id, se.show_id, se.number, e.number, e.title, e.overview, e.air_date,
	e.still_path, e.runtime_min, e.created_at, sh.title, sh.backdrop_path,
	(SELECT id FROM sc_media_files WHERE episode_id = e.id ORDER BY size_bytes DESC, file_path LIMIT 1),
	(SELECT duration_sec FROM sc_media_files WHERE episode_id = e.id ORDER BY size_bytes DESC, file_path LIMIT 1)`

const episodeFrom = ` FROM sc_episodes e JOIN sc_seasons se ON se.id = e.season_id JOIN sc_shows sh ON sh.id = se.show_id`

func scanEpisode(sc interface{ Scan(...any) error }) (model.Episode, error) {
	var (
		e                               model.Episode
		title, overview, airDate, still sql.NullString
		showBackdrop                    sql.NullString
		runtime                         sql.NullInt64
		duration                        sql.NullFloat64
		mediaID                         *uuid.UUID
	)
	err := sc.Scan(&e.ID, &e.SeasonID, &e.ShowID, &e.Season, &e.Number, &title, &overview, &airDate,
		&still, &runtime, &e.CreatedAt, &e.ShowTitle, &showBackdrop, &mediaID, &duration)
	if err != nil {
		return e, err
	}
	e.Title, e.Overview, e.AirDate, e.StillPath = ptrString(title), ptrString(overview), ptrString(airDate), ptrString(still)
	e.RuntimeMin, e.MediaID, e.DurationSec = ptrInt(runtime), mediaID, ptrFloat(duration)
	e.ShowBackdropPath = ptrString(showBackdrop)
	return e, nil
}

// ListSeasons vrátí řady seriálu s díly, které mají soubor, seřazené podle čísla.
func (s *Store) ListSeasons(ctx context.Context, showID uuid.UUID) ([]model.Season, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, show_id, number, title, overview, poster_path FROM sc_seasons
		WHERE show_id = ?1 ORDER BY number`, showID)
	if err != nil {
		return nil, fmt.Errorf("list seasons: %w", err)
	}
	seasons := []model.Season{}
	index := map[uuid.UUID]int{}
	for rows.Next() {
		var (
			se                      model.Season
			title, overview, poster sql.NullString
		)
		if err := rows.Scan(&se.ID, &se.ShowID, &se.Number, &title, &overview, &poster); err != nil {
			rows.Close()
			return nil, err
		}
		se.Title, se.Overview, se.PosterPath = ptrString(title), ptrString(overview), ptrString(poster)
		se.Episodes = []model.Episode{}
		index[se.ID] = len(seasons)
		seasons = append(seasons, se)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	erows, err := s.db.QueryContext(ctx, `SELECT `+episodeCols+episodeFrom+`
		WHERE se.show_id = ?1 AND EXISTS (SELECT 1 FROM sc_media_files f WHERE f.episode_id = e.id)
		ORDER BY se.number, e.number`, showID)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	defer erows.Close()
	for erows.Next() {
		e, err := scanEpisode(erows)
		if err != nil {
			return nil, err
		}
		if i, ok := index[e.SeasonID]; ok {
			seasons[i].Episodes = append(seasons[i].Episodes, e)
		}
	}
	if err := erows.Err(); err != nil {
		return nil, err
	}
	// Řady bez souborů (jen z TMDB) se nevracejí.
	out := seasons[:0]
	for _, se := range seasons {
		if len(se.Episodes) > 0 {
			out = append(out, se)
		}
	}
	return out, nil
}

// GetEpisode vrátí díl podle ID.
func (s *Store) GetEpisode(ctx context.Context, id uuid.UUID) (*model.Episode, error) {
	e, err := scanEpisode(s.db.QueryRowContext(ctx, `SELECT `+episodeCols+episodeFrom+` WHERE e.id = ?1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get episode: %w", err)
	}
	return &e, nil
}

// NextEpisode vrátí díl, který v pořadí (řada, díl) následuje po daném a má
// soubor, nebo ErrNotFound na konci seriálu.
func (s *Store) NextEpisode(ctx context.Context, id uuid.UUID) (*model.Episode, error) {
	e, err := scanEpisode(s.db.QueryRowContext(ctx, `SELECT `+episodeCols+episodeFrom+`
		JOIN (SELECT se2.show_id, se2.number AS sn, e2.number AS en FROM sc_episodes e2
		      JOIN sc_seasons se2 ON se2.id = e2.season_id WHERE e2.id = ?1) cur ON cur.show_id = se.show_id
		WHERE (se.number > cur.sn OR (se.number = cur.sn AND e.number > cur.en))
		  AND EXISTS (SELECT 1 FROM sc_media_files f WHERE f.episode_id = e.id)
		ORDER BY se.number, e.number LIMIT 1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("next episode: %w", err)
	}
	return &e, nil
}

// RecentEpisodes vrátí naposledy přidané díly (napříč seriály).
// library = uuid.Nil znamená všechny zapnuté knihovny.
func (s *Store) RecentEpisodes(ctx context.Context, library uuid.UUID, limit int) ([]model.Episode, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+episodeCols+episodeFrom+`
		JOIN sc_libraries lib ON lib.id = sh.library_id
		WHERE lib.enabled = 1 AND (?2 = '`+uuid.Nil.String()+`' OR sh.library_id = ?2)
		  AND EXISTS (SELECT 1 FROM sc_media_files f WHERE f.episode_id = e.id)
		ORDER BY e.created_at DESC, se.number DESC, e.number DESC LIMIT ?1`, limit, library)
	if err != nil {
		return nil, fmt.Errorf("recent episodes: %w", err)
	}
	defer rows.Close()
	out := []model.Episode{}
	for rows.Next() {
		e, err := scanEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListMedia vrátí soubory filmu nebo dílu včetně stop a titulků, největší první.
func (s *Store) ListMedia(ctx context.Context, movieID, episodeID *uuid.UUID) ([]model.MediaFile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM sc_media_files
		WHERE (?1 IS NOT NULL AND movie_id = ?1) OR (?2 IS NOT NULL AND episode_id = ?2)
		ORDER BY size_bytes DESC, file_path`, movieID, episodeID)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []model.MediaFile{}
	for _, id := range ids {
		mf, err := s.GetMedia(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *mf)
	}
	return out, nil
}

// GetMedia vrátí soubor se stopami a externími titulky.
func (s *Store) GetMedia(ctx context.Context, id uuid.UUID) (*model.MediaFile, error) {
	var (
		mf                                     model.MediaFile
		container, vcodec, vprofile, pix, perr sql.NullString
		duration                               sql.NullFloat64
		bitrate, width, height                 sql.NullInt64
		mtime, probed                          sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT id, movie_id, episode_id, file_path, size_bytes, mtime, container, duration_sec, bitrate,
		       video_codec, video_profile, width, height, pix_fmt, probed_at, probe_error
		FROM sc_media_files WHERE id = ?1`, id).Scan(
		&mf.ID, &mf.MovieID, &mf.EpisodeID, &mf.FilePath, &mf.SizeBytes, &mtime, &container, &duration, &bitrate,
		&vcodec, &vprofile, &width, &height, &pix, &probed, &perr)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get media: %w", err)
	}
	mf.Container, mf.DurationSec, mf.Bitrate = ptrString(container), ptrFloat(duration), ptrInt64(bitrate)
	mf.VideoCodec, mf.VideoProfile, mf.PixFmt = ptrString(vcodec), ptrString(vprofile), ptrString(pix)
	mf.Width, mf.Height, mf.ProbeError = ptrInt(width), ptrInt(height), ptrString(perr)
	if mtime.Valid {
		t := mtime.Time
		mf.ModTime = &t
	}
	if probed.Valid {
		t := probed.Time
		mf.ProbedAt = &t
	}

	mf.Streams = []model.Stream{}
	rows, err := s.db.QueryContext(ctx, `
		SELECT idx, kind, codec, language, title, channels, is_default, is_forced
		FROM sc_streams WHERE media_file_id = ?1 ORDER BY idx`, id)
	if err != nil {
		return nil, fmt.Errorf("streams: %w", err)
	}
	for rows.Next() {
		var (
			st                 model.Stream
			codec, lang, title sql.NullString
			channels           sql.NullInt64
		)
		if err := rows.Scan(&st.Index, &st.Kind, &codec, &lang, &title, &channels, &st.IsDefault, &st.IsForced); err != nil {
			rows.Close()
			return nil, err
		}
		st.Codec, st.Language, st.Title, st.Channels = ptrString(codec), ptrString(lang), ptrString(title), ptrInt(channels)
		mf.Streams = append(mf.Streams, st)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	mf.Subtitles = []model.ExternalSubtitle{}
	srows, err := s.db.QueryContext(ctx, `
		SELECT id, file_path, language, format, is_forced FROM sc_external_subtitles
		WHERE media_file_id = ?1 ORDER BY language, file_path`, id)
	if err != nil {
		return nil, fmt.Errorf("external subtitles: %w", err)
	}
	defer srows.Close()
	for srows.Next() {
		var (
			sub  model.ExternalSubtitle
			lang sql.NullString
		)
		if err := srows.Scan(&sub.ID, &sub.FilePath, &lang, &sub.Format, &sub.IsForced); err != nil {
			return nil, err
		}
		sub.Language = ptrString(lang)
		mf.Subtitles = append(mf.Subtitles, sub)
	}
	return &mf, srows.Err()
}

// Genres vrátí žánry, které má nějaký film nebo seriál.
// library = uuid.Nil znamená všechny knihovny.
func (s *Store) Genres(ctx context.Context, library uuid.UUID) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name FROM sc_genres g
		WHERE EXISTS (SELECT 1 FROM sc_movie_genres x JOIN sc_movies t ON t.id = x.movie_id
			WHERE x.genre_id = g.id AND (?1 = '`+uuid.Nil.String()+`' OR t.library_id = ?1))
		   OR EXISTS (SELECT 1 FROM sc_show_genres x JOIN sc_shows t ON t.id = x.show_id
			WHERE x.genre_id = g.id AND (?1 = '`+uuid.Nil.String()+`' OR t.library_id = ?1))
		ORDER BY name`, library)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Stats jsou počty pro administraci.
type Stats struct {
	Movies          int   `json:"movies"`
	Shows           int   `json:"shows"`
	Episodes        int   `json:"episodes"`
	Files           int   `json:"files"`
	TotalBytes      int64 `json:"total_bytes"`
	UnmatchedMovies int   `json:"unmatched_movies"`
	UnmatchedShows  int   `json:"unmatched_shows"`
	// Failed* jsou nespárované tituly, pro které žádný zdroj nenašel jistý výsledek.
	FailedMovies int `json:"failed_movies"`
	FailedShows  int `json:"failed_shows"`
}

// movieHasFiles omezí filmy na ty, které mají soubor (ostatní se nikde
// nevypisují); obdoba showHasFiles.
const movieHasFiles = ` EXISTS (SELECT 1 FROM sc_media_files f WHERE f.movie_id = m.id)`

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT count(*) FROM sc_movies m WHERE EXISTS (SELECT 1 FROM sc_media_files f WHERE f.movie_id = m.id)),
		  (SELECT count(*) FROM sc_shows),
		  (SELECT count(DISTINCT episode_id) FROM sc_media_files WHERE episode_id IS NOT NULL),
		  (SELECT count(*) FROM sc_media_files),
		  (SELECT COALESCE(sum(size_bytes), 0) FROM sc_media_files),
		  (SELECT count(*) FROM sc_movies m WHERE match_state = 'unmatched' AND`+movieHasFiles+`),
		  (SELECT count(*) FROM sc_shows s WHERE match_state = 'unmatched' AND`+showHasFiles+`),
		  (SELECT count(*) FROM sc_movies m WHERE match_state = 'unmatched' AND match_attempted_at IS NOT NULL AND`+movieHasFiles+`),
		  (SELECT count(*) FROM sc_shows s WHERE match_state = 'unmatched' AND match_attempted_at IS NOT NULL AND`+showHasFiles+`)`).
		Scan(&st.Movies, &st.Shows, &st.Episodes, &st.Files, &st.TotalBytes, &st.UnmatchedMovies, &st.UnmatchedShows,
			&st.FailedMovies, &st.FailedShows)
	return st, err
}
