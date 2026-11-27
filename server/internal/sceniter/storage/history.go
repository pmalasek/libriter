package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	corestorage "mediater/internal/core/storage"
	"mediater/internal/sceniter/model"
)

// WatchSummary je součet historie sledování jednoho uživatele pro přehled
// v administraci. Jména a e-maily doplní služba z jádra – úložiště Sceniteru
// tabulku users nečte.
type WatchSummary struct {
	UserID   uuid.UUID
	Seconds  int64
	Movies   int
	Shows    int
	Episodes int
	LastAt   time.Time
}

// WatchHistoryEntry je jeden řádek historie: titul a den. U filmu jsou
// vyplněná pole Movie*, u dílu Show*, Season, Episode*.
type WatchHistoryEntry struct {
	Day          string     `json:"day"`
	Kind         string     `json:"kind"`
	MovieID      *uuid.UUID `json:"movie_id,omitempty"`
	MovieTitle   *string    `json:"movie_title,omitempty"`
	MovieYear    *int       `json:"movie_year,omitempty"`
	ShowID       *uuid.UUID `json:"show_id,omitempty"`
	ShowTitle    *string    `json:"show_title,omitempty"`
	Season       *int       `json:"season,omitempty"`
	EpisodeID    *uuid.UUID `json:"episode_id,omitempty"`
	Episode      *int       `json:"episode,omitempty"`
	EpisodeTitle *string    `json:"episode_title,omitempty"`
	Seconds      int64      `json:"seconds"`
	FirstAt      time.Time  `json:"first_at"`
	LastAt       time.Time  `json:"last_at"`
}

// addWatchHistory připíše odkoukané sekundy k titulu a dni události (UTC).
// Volá se uvnitř transakce SaveProgress, proto bere querier. Částečný
// unikátní index vyžaduje v ON CONFLICT i podmínku WHERE, takže film a díl
// mají každý vlastní dotaz.
func addWatchHistory(ctx context.Context, q querier, userID uuid.UUID, movieID, episodeID *uuid.UUID, seconds int, at time.Time) error {
	const upsert = `
		INSERT INTO sc_watch_history (user_id, movie_id, episode_id, day, seconds, first_at, last_at)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?6)
		ON CONFLICT (user_id, %s, day) WHERE %s IS NOT NULL DO UPDATE SET
		  seconds  = seconds + excluded.seconds,
		  first_at = MIN(first_at, excluded.first_at),
		  last_at  = MAX(last_at, excluded.last_at)`
	col := "movie_id"
	if movieID == nil {
		col = "episode_id"
	}
	_, err := q.ExecContext(ctx, fmt.Sprintf(upsert, col, col),
		userID, movieID, episodeID, at.UTC().Format("2006-01-02"), seconds, corestorage.SQLiteTime(at))
	if err != nil {
		if corestorage.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("watch history: %w", err)
	}
	return nil
}

// WatchSummaries vrátí součty historie po uživatelích, kteří něco sledovali.
// Titulem se rozumí film nebo seriál (díly jednoho seriálu jsou jeden titul).
func (s *Store) WatchSummaries(ctx context.Context) ([]WatchSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.user_id, SUM(h.seconds),
		  COUNT(DISTINCT h.movie_id), COUNT(DISTINCT se.show_id), COUNT(DISTINCT h.episode_id),
		  MAX(h.last_at)
		FROM sc_watch_history h
		LEFT JOIN sc_episodes e ON e.id = h.episode_id
		LEFT JOIN sc_seasons se ON se.id = e.season_id
		GROUP BY h.user_id`)
	if err != nil {
		return nil, fmt.Errorf("watch summaries: %w", err)
	}
	defer rows.Close()
	out := []WatchSummary{}
	for rows.Next() {
		var (
			sum  WatchSummary
			last string
		)
		if err := rows.Scan(&sum.UserID, &sum.Seconds, &sum.Movies, &sum.Shows, &sum.Episodes, &last); err != nil {
			return nil, fmt.Errorf("watch summaries: %w", err)
		}
		if sum.LastAt, err = parseSQLiteTime(last); err != nil {
			return nil, fmt.Errorf("watch summaries: last_at: %w", err)
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// WatchHistory vrátí historii uživatele za posledních `days` dní včetně
// dneška (UTC), od nejnovějšího dne a v něm od naposledy sledovaného titulu.
// Tituly se připojují v jednom dotazu (sloupce podle episodeCols/episodeFrom).
func (s *Store) WatchHistory(ctx context.Context, userID uuid.UUID, days int) ([]WatchHistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT h.day, h.movie_id, m.title, m.year,
		  sh.id, sh.title, se.number, h.episode_id, e.number, e.title,
		  h.seconds, h.first_at, h.last_at
		FROM sc_watch_history h
		LEFT JOIN sc_movies m ON m.id = h.movie_id
		LEFT JOIN sc_episodes e ON e.id = h.episode_id
		LEFT JOIN sc_seasons se ON se.id = e.season_id
		LEFT JOIN sc_shows sh ON sh.id = se.show_id
		WHERE h.user_id = ?1 AND h.day >= date('now', ?2)
		ORDER BY h.day DESC, h.last_at DESC`,
		userID, fmt.Sprintf("-%d days", max(days-1, 0)))
	if err != nil {
		return nil, fmt.Errorf("watch history: %w", err)
	}
	defer rows.Close()
	out := []WatchHistoryEntry{}
	for rows.Next() {
		var (
			e                              WatchHistoryEntry
			movieTitle, showTitle, epTitle sql.NullString
			movieYear, season, episode     sql.NullInt64
			first, last                    any
		)
		if err := rows.Scan(&e.Day, &e.MovieID, &movieTitle, &movieYear,
			&e.ShowID, &showTitle, &season, &e.EpisodeID, &episode, &epTitle,
			&e.Seconds, &first, &last); err != nil {
			return nil, fmt.Errorf("watch history: %w", err)
		}
		e.Kind = model.KindMovie
		if e.EpisodeID != nil {
			e.Kind = model.KindEpisode
		}
		e.MovieTitle, e.MovieYear = ptrString(movieTitle), ptrInt(movieYear)
		e.ShowTitle, e.Season, e.Episode, e.EpisodeTitle = ptrString(showTitle), ptrInt(season), ptrInt(episode), ptrString(epTitle)
		if e.FirstAt, err = sqliteTimeValue(first); err != nil {
			return nil, fmt.Errorf("watch history: first_at: %w", err)
		}
		if e.LastAt, err = sqliteTimeValue(last); err != nil {
			return nil, fmt.Errorf("watch history: last_at: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Formáty, ve kterých SQLite vrací DATETIME z výrazu: CURRENT_TIMESTAMP bez
// zóny a hodnoty zapsané driverem i se zlomkem sekundy a zónou.
var sqliteTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.999999999-07:00",
	time.RFC3339Nano,
}

// parseSQLiteTime čte čas z výrazu jako MAX(last_at). Driver převádí na
// time.Time jen sloupce deklarované jako DATETIME; výraz deklaraci nemá, a
// tak přijde jako text.
func parseSQLiteTime(value string) (time.Time, error) {
	for _, layout := range sqliteTimeLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("neznámý formát času %q", value)
}

// sqliteTimeValue přijme sloupec DATETIME jako time.Time (převedl ho driver)
// i jako text.
func sqliteTimeValue(v any) (time.Time, error) {
	switch t := v.(type) {
	case time.Time:
		return t.UTC(), nil
	case string:
		return parseSQLiteTime(t)
	case []byte:
		return parseSQLiteTime(string(t))
	}
	return time.Time{}, fmt.Errorf("neočekávaný typ času %T", v)
}
