package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	corestorage "mediater/internal/core/storage"
	"mediater/internal/sceniter/model"
)

// MaxWatchedSecondsPerSave omezuje sekundy připsané jedním zápisem, aby
// chybný klient nenafoukl statistiky (stejná pojistka jako v Libriteru).
const MaxWatchedSecondsPerSave = 600

// ErrStale znamená, že na serveru je novější pozice než ta poslaná –
// pozice se nezapsala, odkoukané sekundy ano.
var ErrStale = errors.New("stale position")

// ErrDuplicateEvent znamená, že událost se stejným ID už byla zpracovaná.
var ErrDuplicateEvent = corestorage.ErrDuplicateEvent

// ProgressInput je zápis pozice z přehrávače.
type ProgressInput struct {
	MovieID        *uuid.UUID
	EpisodeID      *uuid.UUID
	MediaID        *uuid.UUID
	PositionSec    float64
	DurationSec    float64
	Completed      bool
	DeviceID       string
	RecordedAt     time.Time
	WatchedSeconds int
	// EventID zapne idempotenci (dávky z offline zařízení); nil u webu.
	EventID *uuid.UUID
}

// SaveProgress zapíše pozici, pokud je novější než uložená (recorded_at),
// a připíše odkoukané sekundy k UTC dni recorded_at. Vrací ErrStale, když
// se pozice kvůli novějšímu záznamu nezapsala, a ErrDuplicateEvent u
// opakované události.
func (s *Store) SaveProgress(ctx context.Context, userID uuid.UUID, in ProgressInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if in.EventID != nil {
		if err := corestorage.InsertSyncEvent(ctx, tx, *in.EventID, userID, in.DeviceID); err != nil {
			return err
		}
	}

	stale := false
	var existing time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT recorded_at FROM sc_watch_progress
		WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND episode_id = ?3))`,
		userID, in.MovieID, in.EpisodeID).Scan(&existing)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sc_watch_progress (user_id, movie_id, episode_id, media_file_id, position_sec,
			  duration_sec, completed, device_id, recorded_at)
			VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)`,
			userID, in.MovieID, in.EpisodeID, in.MediaID, in.PositionSec, nullFloat(in.DurationSec),
			in.Completed, nullStr(in.DeviceID), in.RecordedAt.UTC())
	case err == nil && in.RecordedAt.After(existing):
		_, err = tx.ExecContext(ctx, `
			UPDATE sc_watch_progress SET media_file_id = COALESCE(?4, media_file_id), position_sec = ?5,
			  duration_sec = COALESCE(?6, duration_sec), completed = ?7, device_id = ?8,
			  recorded_at = ?9, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND episode_id = ?3))`,
			userID, in.MovieID, in.EpisodeID, in.MediaID, in.PositionSec, nullFloat(in.DurationSec),
			in.Completed, nullStr(in.DeviceID), in.RecordedAt.UTC())
	case err == nil:
		stale = true
	}
	if err != nil {
		if corestorage.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("save progress: %w", err)
	}

	if secs := min(max(in.WatchedSeconds, 0), MaxWatchedSecondsPerSave); secs > 0 {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sc_watch_log (user_id, day, seconds) VALUES (?1, ?2, ?3)
			ON CONFLICT (user_id, day) DO UPDATE SET seconds = seconds + excluded.seconds`,
			userID, in.RecordedAt.UTC().Format("2006-01-02"), secs)
		if err != nil {
			return fmt.Errorf("watch log: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if stale {
		return ErrStale
	}
	return nil
}

const progressCols = `movie_id, episode_id, media_file_id, position_sec, duration_sec, completed, recorded_at, updated_at`

func scanProgress(sc interface{ Scan(...any) error }) (model.WatchProgress, error) {
	var (
		p        model.WatchProgress
		duration sql.NullFloat64
	)
	err := sc.Scan(&p.MovieID, &p.EpisodeID, &p.MediaID, &p.PositionSec, &duration, &p.Completed, &p.RecordedAt, &p.UpdatedAt)
	p.DurationSec = ptrFloat(duration)
	return p, err
}

// GetProgress vrátí rozkoukanost filmu nebo dílu, nebo ErrNotFound.
func (s *Store) GetProgress(ctx context.Context, userID uuid.UUID, movieID, episodeID *uuid.UUID) (*model.WatchProgress, error) {
	p, err := scanProgress(s.db.QueryRowContext(ctx, `SELECT `+progressCols+` FROM sc_watch_progress
		WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND episode_id = ?3))`,
		userID, movieID, episodeID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get progress: %w", err)
	}
	return &p, nil
}

// ProgressFilter omezuje výpis rozkoukanosti.
type ProgressFilter struct {
	ShowID     *uuid.UUID // jen díly seriálu
	InProgress bool       // jen rozkoukané (ne dokoukané)
	Completed  bool       // jen dokoukané
	Limit      int
}

// ListProgress vrátí rozkoukanost uživatele, od naposledy změněné.
func (s *Store) ListProgress(ctx context.Context, userID uuid.UUID, f ProgressFilter) ([]model.WatchProgress, error) {
	q := `SELECT ` + progressCols + ` FROM sc_watch_progress p WHERE user_id = ?1`
	args := []any{userID}
	if f.ShowID != nil {
		args = append(args, *f.ShowID)
		q += fmt.Sprintf(` AND episode_id IN (SELECT e.id FROM sc_episodes e JOIN sc_seasons se ON se.id = e.season_id
			WHERE se.show_id = ?%d)`, len(args))
	}
	if f.InProgress {
		q += ` AND completed = 0 AND position_sec > 0`
	}
	if f.Completed {
		q += ` AND completed = 1`
	}
	q += ` ORDER BY updated_at DESC`
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list progress: %w", err)
	}
	defer rows.Close()
	out := []model.WatchProgress{}
	for rows.Next() {
		p, err := scanProgress(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DeleteProgress smaže rozkoukanost („označit jako neviděné“).
func (s *Store) DeleteProgress(ctx context.Context, userID uuid.UUID, movieID, episodeID *uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sc_watch_progress
		WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND episode_id = ?3))`,
		userID, movieID, episodeID)
	return err
}

// WatchlistEntry je položka seznamu „chci vidět“.
type WatchlistEntry struct {
	MovieID *uuid.UUID
	ShowID  *uuid.UUID
	AddedAt time.Time
}

// ListWatchlist vrátí seznam „chci vidět“, nejnovější první.
func (s *Store) ListWatchlist(ctx context.Context, userID uuid.UUID) ([]WatchlistEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT movie_id, show_id, added_at FROM sc_watchlist
		WHERE user_id = ?1 ORDER BY added_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}
	defer rows.Close()
	out := []WatchlistEntry{}
	for rows.Next() {
		var e WatchlistEntry
		if err := rows.Scan(&e.MovieID, &e.ShowID, &e.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetWatchlist přidá titul na seznam (on=true) nebo ho z něj odebere.
func (s *Store) SetWatchlist(ctx context.Context, userID uuid.UUID, movieID, showID *uuid.UUID, on bool) error {
	var err error
	if on {
		_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO sc_watchlist (user_id, movie_id, show_id) VALUES (?1, ?2, ?3)`,
			userID, movieID, showID)
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM sc_watchlist
			WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND show_id = ?3))`,
			userID, movieID, showID)
	}
	if err != nil {
		if corestorage.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("watchlist: %w", err)
	}
	return nil
}

// SetRating uloží hodnocení 1–10; nil hodnocení smaže.
func (s *Store) SetRating(ctx context.Context, userID uuid.UUID, movieID, showID *uuid.UUID, rating *int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sc_ratings
		WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND show_id = ?3))`,
		userID, movieID, showID)
	if err == nil && rating != nil {
		_, err = s.db.ExecContext(ctx, `INSERT INTO sc_ratings (user_id, movie_id, show_id, rating) VALUES (?1, ?2, ?3, ?4)`,
			userID, movieID, showID, *rating)
	}
	if err != nil {
		if corestorage.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("rating: %w", err)
	}
	return nil
}

// UserState vrátí vztah uživatele k filmu nebo seriálu.
func (s *Store) UserState(ctx context.Context, userID uuid.UUID, movieID, showID *uuid.UUID) (model.UserState, error) {
	var (
		st     model.UserState
		rating sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT
		  EXISTS (SELECT 1 FROM sc_watchlist WHERE user_id = ?1
		          AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND show_id = ?3))),
		  (SELECT rating FROM sc_ratings WHERE user_id = ?1
		          AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND show_id = ?3)))`,
		userID, movieID, showID).Scan(&st.InWatchlist, &rating)
	if err != nil {
		return st, fmt.Errorf("user state: %w", err)
	}
	st.Rating = ptrInt(rating)
	if movieID != nil {
		p, err := s.GetProgress(ctx, userID, movieID, nil)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return st, err
		}
		st.Progress = p
	}
	return st, nil
}

// WatchLogDay jsou odkoukané sekundy za den.
type WatchLogDay struct {
	Day     string `json:"day"`
	Seconds int    `json:"seconds"`
}

// WatchLog vrátí odkoukané sekundy uživatele po dnech (nejnovější první).
func (s *Store) WatchLog(ctx context.Context, userID uuid.UUID, days int) ([]WatchLogDay, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT day, seconds FROM sc_watch_log WHERE user_id = ?1
		ORDER BY day DESC LIMIT ?2`, userID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WatchLogDay{}
	for rows.Next() {
		var d WatchLogDay
		if err := rows.Scan(&d.Day, &d.Seconds); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
