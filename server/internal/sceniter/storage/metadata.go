package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
)

// Genre je žánr z TMDB.
type Genre struct {
	TMDBID int
	Name   string
}

// TitleMeta jsou metadata filmu nebo seriálu z TMDB. Obrázky jsou názvy
// souborů v SCENITER_IMAGE_ROOT (prázdné = beze změny).
type TitleMeta struct {
	TMDBID        int
	IMDBID        string
	Title         string
	OriginalTitle string
	Overview      string
	Tagline       string
	Status        string
	Year          int
	RuntimeMin    int
	Rating        float64
	PosterPath    string
	BackdropPath  string
	Genres        []Genre
	Credits       []model.Credit
}

// ApplyMovieMeta uloží metadata k filmu. Když už jiný film má stejné
// tmdb_id (druhá verze téhož filmu v jiném souboru), soubory se přesunou
// k němu a tento záznam zanikne. Vrací ID filmu, který metadata nese.
func (s *Store) ApplyMovieMeta(ctx context.Context, id uuid.UUID, m TitleMeta, state string) (uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var other uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM sc_movies WHERE tmdb_id = ?1 AND id <> ?2`, m.TMDBID, id).Scan(&other)
	switch {
	case err == nil:
		if _, err := tx.ExecContext(ctx, `UPDATE sc_media_files SET movie_id = ?1 WHERE movie_id = ?2`, other, id); err != nil {
			return uuid.Nil, fmt.Errorf("merge movie files: %w", err)
		}
		// Rozkoukanost a seznamy uživatelů přejdou na sloučený film, pokud tam ještě nejsou.
		for _, table := range []string{"sc_watch_progress", "sc_watchlist", "sc_ratings"} {
			if _, err := tx.ExecContext(ctx, `UPDATE OR IGNORE `+table+` SET movie_id = ?1 WHERE movie_id = ?2`, other, id); err != nil {
				return uuid.Nil, fmt.Errorf("merge %s: %w", table, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sc_movies WHERE id = ?1`, id); err != nil {
			return uuid.Nil, fmt.Errorf("merge movie: %w", err)
		}
		id = other
	case !errors.Is(err, sql.ErrNoRows):
		return uuid.Nil, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE sc_movies SET
		  tmdb_id = ?2, imdb_id = COALESCE(?3, imdb_id), title = COALESCE(?4, title),
		  original_title = COALESCE(?5, original_title), overview = COALESCE(?6, overview),
		  tagline = COALESCE(?7, tagline), year = COALESCE(?8, year), runtime_min = COALESCE(?9, runtime_min),
		  tmdb_rating = COALESCE(?10, tmdb_rating), poster_path = COALESCE(?11, poster_path),
		  backdrop_path = COALESCE(?12, backdrop_path), credits_json = COALESCE(?13, credits_json),
		  match_state = ?14, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?1`,
		id, m.TMDBID, nullStr(m.IMDBID), nullStr(m.Title), nullStr(m.OriginalTitle), nullStr(m.Overview),
		nullStr(m.Tagline), nullInt(m.Year), nullInt(m.RuntimeMin), nullFloat(m.Rating),
		nullStr(m.PosterPath), nullStr(m.BackdropPath), model.MarshalCredits(m.Credits), state)
	if err != nil {
		return uuid.Nil, fmt.Errorf("apply movie meta: %w", err)
	}
	if err := setGenres(ctx, tx, "movie", id, m.Genres); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit()
}

// ApplyShowMeta uloží metadata k seriálu.
func (s *Store) ApplyShowMeta(ctx context.Context, id uuid.UUID, m TitleMeta, state string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx, `
		UPDATE sc_shows SET
		  tmdb_id = ?2, title = COALESCE(?3, title), original_title = COALESCE(?4, original_title),
		  overview = COALESCE(?5, overview), status = COALESCE(?6, status), year = COALESCE(?7, year),
		  tmdb_rating = COALESCE(?8, tmdb_rating), poster_path = COALESCE(?9, poster_path),
		  backdrop_path = COALESCE(?10, backdrop_path), credits_json = COALESCE(?11, credits_json),
		  match_state = ?12, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?1`,
		id, m.TMDBID, nullStr(m.Title), nullStr(m.OriginalTitle), nullStr(m.Overview), nullStr(m.Status),
		nullInt(m.Year), nullFloat(m.Rating), nullStr(m.PosterPath), nullStr(m.BackdropPath),
		model.MarshalCredits(m.Credits), state)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return fmt.Errorf("apply show meta: %w", err)
	}
	if err := setGenres(ctx, tx, "show", id, m.Genres); err != nil {
		return err
	}
	return tx.Commit()
}

// EpisodeMeta jsou metadata dílu z TMDB.
type EpisodeMeta struct {
	Number     int
	Title      string
	Overview   string
	AirDate    string
	StillPath  string
	RuntimeMin int
}

// SeasonMeta jsou metadata řady z TMDB.
type SeasonMeta struct {
	Number     int
	Title      string
	Overview   string
	PosterPath string
	Episodes   []EpisodeMeta
}

// ApplySeasonMeta doplní metadata existující řady a jejích existujících
// dílů. Díly, které na disku nejsou, se nezakládají.
func (s *Store) ApplySeasonMeta(ctx context.Context, showID uuid.UUID, m SeasonMeta) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var seasonID uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM sc_seasons WHERE show_id = ?1 AND number = ?2`, showID, m.Number).Scan(&seasonID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sc_seasons SET title = COALESCE(?2, title), overview = COALESCE(?3, overview),
		  poster_path = COALESCE(?4, poster_path) WHERE id = ?1`,
		seasonID, nullStr(m.Title), nullStr(m.Overview), nullStr(m.PosterPath)); err != nil {
		return fmt.Errorf("apply season meta: %w", err)
	}
	for _, e := range m.Episodes {
		if _, err := tx.ExecContext(ctx, `
			UPDATE sc_episodes SET title = COALESCE(?3, title), overview = COALESCE(?4, overview),
			  air_date = COALESCE(?5, air_date), still_path = COALESCE(?6, still_path),
			  runtime_min = COALESCE(?7, runtime_min)
			WHERE season_id = ?1 AND number = ?2`,
			seasonID, e.Number, nullStr(e.Title), nullStr(e.Overview), nullStr(e.AirDate),
			nullStr(e.StillPath), nullInt(e.RuntimeMin)); err != nil {
			return fmt.Errorf("apply episode meta: %w", err)
		}
	}
	return tx.Commit()
}

func setGenres(ctx context.Context, tx *sql.Tx, kind string, id uuid.UUID, genres []Genre) error {
	if genres == nil {
		return nil
	}
	table, col := "sc_movie_genres", "movie_id"
	if kind == "show" {
		table, col = "sc_show_genres", "show_id"
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE `+col+` = ?1`, id); err != nil {
		return err
	}
	for _, g := range genres {
		var gid int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO sc_genres (tmdb_id, name) VALUES (?1, ?2)
			ON CONFLICT (tmdb_id) DO UPDATE SET name = excluded.name
			RETURNING id`, g.TMDBID, g.Name).Scan(&gid)
		if err != nil {
			return fmt.Errorf("genre: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO `+table+` (`+col+`, genre_id) VALUES (?1, ?2)`, id, gid); err != nil {
			return err
		}
	}
	return nil
}

// SetMatchState změní stav párování. Stav „unmatched“ zároveň zruší vazbu
// na TMDB (texty a obrázky zůstanou, dokud se titul nespáruje znovu).
func (s *Store) SetMatchState(ctx context.Context, kind string, id uuid.UUID, state string) error {
	table := "sc_movies"
	if kind == model.KindShow {
		table = "sc_shows"
	}
	q := `UPDATE ` + table + ` SET match_state = ?2, updated_at = CURRENT_TIMESTAMP`
	if state == model.MatchUnmatched {
		q += `, tmdb_id = NULL`
	}
	res, err := s.db.ExecContext(ctx, q+` WHERE id = ?1`, id, state)
	if err != nil {
		return fmt.Errorf("set match state: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TitlePatch je ruční úprava filmu nebo seriálu editorem. Nil = beze změny.
type TitlePatch struct {
	Title         *string `json:"title"`
	OriginalTitle *string `json:"original_title"`
	Year          *int    `json:"year"`
	Overview      *string `json:"overview"`
}

// PatchTitle uloží ruční úpravu.
func (s *Store) PatchTitle(ctx context.Context, kind string, id uuid.UUID, p TitlePatch) error {
	table := "sc_movies"
	if kind == model.KindShow {
		table = "sc_shows"
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE `+table+` SET
		  title = COALESCE(?2, title),
		  original_title = CASE WHEN ?3 IS NULL THEN original_title ELSE NULLIF(?3, '') END,
		  year = CASE WHEN ?4 IS NULL THEN year ELSE NULLIF(?4, 0) END,
		  overview = CASE WHEN ?5 IS NULL THEN overview ELSE NULLIF(?5, '') END,
		  updated_at = CURRENT_TIMESTAMP
		WHERE id = ?1`, id, p.Title, p.OriginalTitle, p.Year, p.Overview)
	if err != nil {
		return fmt.Errorf("patch title: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
