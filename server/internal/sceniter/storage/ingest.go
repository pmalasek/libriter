package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
)

// MediaRecord je stav souboru v databázi, podle kterého scanner pozná,
// jestli soubor potřebuje znovu prozkoumat.
type MediaRecord struct {
	ID        uuid.UUID
	MovieID   *uuid.UUID
	EpisodeID *uuid.UUID
	SizeBytes int64
	ModTime   time.Time
	Probed    bool
}

// MediaByPath vrátí záznam souboru podle cesty, nebo ErrNotFound.
func (s *Store) MediaByPath(ctx context.Context, path string) (*MediaRecord, error) {
	const q = `
		SELECT id, movie_id, episode_id, size_bytes, mtime, probed_at IS NOT NULL
		FROM sc_media_files WHERE file_path = ?1`
	var (
		r        MediaRecord
		movie    *uuid.UUID
		episode  *uuid.UUID
		modified sql.NullTime
	)
	err := s.db.QueryRowContext(ctx, q, path).Scan(&r.ID, &movie, &episode, &r.SizeBytes, &modified, &r.Probed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("media by path: %w", err)
	}
	r.MovieID, r.EpisodeID = movie, episode
	if modified.Valid {
		r.ModTime = modified.Time
	}
	return &r, nil
}

// ListMediaPaths vrátí cesty všech souborů v knihovně a jejich ID.
func (s *Store) ListMediaPaths(ctx context.Context) (map[string]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT file_path, id FROM sc_media_files`)
	if err != nil {
		return nil, fmt.Errorf("list media paths: %w", err)
	}
	defer rows.Close()
	out := map[string]uuid.UUID{}
	for rows.Next() {
		var (
			p  string
			id uuid.UUID
		)
		if err := rows.Scan(&p, &id); err != nil {
			return nil, err
		}
		out[p] = id
	}
	return out, rows.Err()
}

// DeleteMedia smaže soubor z knihovny. Film nebo díl bez souborů zůstane
// (drží historii sledování), ve výpisech se ale nezobrazuje.
func (s *Store) DeleteMedia(ctx context.Context, id uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sc_media_files WHERE id = ?1`, id); err != nil {
		return fmt.Errorf("delete media: %w", err)
	}
	return nil
}

// MovieKey identifikuje film na disku a nese údaje z názvu souboru.
type MovieKey struct {
	LibraryID     uuid.UUID
	SourceKey     string
	Title         string
	OriginalTitle string
	Year          int
}

// EnsureMovie vrátí ID filmu se source_key, případně ho založí s údaji
// z názvu souboru. Existující film se nemění – mohl ho upravit editor nebo zdroj metadat.
func (s *Store) EnsureMovie(ctx context.Context, k MovieKey) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.db.QueryRowContext(ctx, `SELECT id FROM sc_movies WHERE source_key = ?1`, k.SourceKey).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("ensure movie: %w", err)
	}
	id = uuid.New()
	const ins = `
		INSERT INTO sc_movies (id, library_id, source_key, title, original_title, year)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6)`
	if _, err := s.db.ExecContext(ctx, ins, id, k.LibraryID, k.SourceKey, k.Title, nullStr(k.OriginalTitle), nullInt(k.Year)); err != nil {
		return uuid.Nil, fmt.Errorf("ensure movie: %w", err)
	}
	return id, nil
}

// MovieSourceKey vrátí source_key filmu, nebo ErrNotFound.
func (s *Store) MovieSourceKey(ctx context.Context, id uuid.UUID) (string, error) {
	var key string
	err := s.db.QueryRowContext(ctx, `SELECT source_key FROM sc_movies WHERE id = ?1`, id).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("movie source key: %w", err)
	}
	return key, nil
}

// MovieFilePaths vrátí cesty souborů filmu.
func (s *Store) MovieFilePaths(ctx context.Context, id uuid.UUID) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT file_path FROM sc_media_files WHERE movie_id = ?1`, id)
	if err != nil {
		return nil, fmt.Errorf("movie file paths: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RekeyMovie změní source_key filmu. Spárovaný film si metadata nechá;
// nespárovanému se název a rok přepíšou podle souboru a smažou se
// zbytky dřívějších metadat. False = film s klíčem už existuje.
func (s *Store) RekeyMovie(ctx context.Context, id uuid.UUID, k MovieKey) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sc_movies WHERE source_key = ?1`, k.SourceKey).Scan(&n); err != nil {
		return false, fmt.Errorf("rekey movie: %w", err)
	}
	if n > 0 {
		return false, nil
	}
	const q = `
		UPDATE sc_movies SET source_key = ?2, updated_at = CURRENT_TIMESTAMP,
		  title          = CASE WHEN match_state = 'unmatched' THEN ?3 ELSE title END,
		  original_title = CASE WHEN match_state = 'unmatched' THEN ?4 ELSE original_title END,
		  year           = CASE WHEN match_state = 'unmatched' THEN ?5 ELSE year END,
		  overview       = CASE WHEN match_state = 'unmatched' THEN NULL ELSE overview END,
		  tagline        = CASE WHEN match_state = 'unmatched' THEN NULL ELSE tagline END,
		  poster_path    = CASE WHEN match_state = 'unmatched' THEN NULL ELSE poster_path END,
		  backdrop_path  = CASE WHEN match_state = 'unmatched' THEN NULL ELSE backdrop_path END,
		  match_attempted_at = CASE WHEN match_state = 'unmatched' THEN NULL ELSE match_attempted_at END
		WHERE id = ?1`
	if _, err := s.db.ExecContext(ctx, q, id, k.SourceKey, k.Title, nullStr(k.OriginalTitle), nullInt(k.Year)); err != nil {
		return false, fmt.Errorf("rekey movie: %w", err)
	}
	return true, nil
}

// EpisodeKey identifikuje díl na disku.
type EpisodeKey struct {
	LibraryID uuid.UUID
	ShowKey   string // adresář seriálu (source_key)
	ShowTitle string
	ShowYear  int
	Season    int
	Episode   int
	Title     string
}

// EnsureEpisode vrátí ID dílu, případně založí seriál, řadu i díl.
// Existující záznamy se nemění, jen chybějící název dílu se doplní z názvu souboru.
func (s *Store) EnsureEpisode(ctx context.Context, k EpisodeKey) (uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var showID uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM sc_shows WHERE source_key = ?1`, k.ShowKey).Scan(&showID)
	if errors.Is(err, sql.ErrNoRows) {
		showID = uuid.New()
		_, err = tx.ExecContext(ctx, `INSERT INTO sc_shows (id, library_id, source_key, title, year) VALUES (?1, ?2, ?3, ?4, ?5)`,
			showID, k.LibraryID, k.ShowKey, k.ShowTitle, nullInt(k.ShowYear))
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure show: %w", err)
	}

	var seasonID uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM sc_seasons WHERE show_id = ?1 AND number = ?2`, showID, k.Season).Scan(&seasonID)
	if errors.Is(err, sql.ErrNoRows) {
		seasonID = uuid.New()
		_, err = tx.ExecContext(ctx, `INSERT INTO sc_seasons (id, show_id, number) VALUES (?1, ?2, ?3)`, seasonID, showID, k.Season)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure season: %w", err)
	}

	var episodeID uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM sc_episodes WHERE season_id = ?1 AND number = ?2`, seasonID, k.Episode).Scan(&episodeID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		episodeID = uuid.New()
		_, err = tx.ExecContext(ctx, `INSERT INTO sc_episodes (id, season_id, number, title) VALUES (?1, ?2, ?3, ?4)`,
			episodeID, seasonID, k.Episode, nullStr(k.Title))
	case err == nil && k.Title != "":
		_, err = tx.ExecContext(ctx, `UPDATE sc_episodes SET title = ?2 WHERE id = ?1 AND (title IS NULL OR title = '')`, episodeID, k.Title)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure episode: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sc_shows SET updated_at = CURRENT_TIMESTAMP WHERE id = ?1`, showID); err != nil {
		return uuid.Nil, err
	}
	return episodeID, tx.Commit()
}

// Probe je výsledek ffprobe pro soubor.
type Probe struct {
	Container    string
	DurationSec  float64
	Bitrate      int64
	VideoCodec   string
	VideoProfile string
	PixFmt       string
	Width        int
	Height       int
	Streams      []model.Stream
	Error        string // ffprobe selhal; soubor se přesto založí
}

// SubtitleFile je externí soubor titulků.
type SubtitleFile struct {
	FilePath string
	Language string
	Format   string
	Forced   bool
}

// SaveMediaInput je vše, co scanner o souboru ví.
type SaveMediaInput struct {
	FilePath  string
	MovieID   *uuid.UUID
	EpisodeID *uuid.UUID
	SizeBytes int64
	ModTime   time.Time
	// Probe je nil, když se soubor nezměnil a údaje z ffprobe se ponechávají.
	Probe     *Probe
	Subtitles []SubtitleFile
}

// SaveMedia založí nebo aktualizuje soubor (podle cesty), jeho stopy a
// externí titulky v jedné transakci. ffprobe běží dřív, mimo transakci –
// SQLite má jediné spojení a dlouhá transakce by blokovala API.
func (s *Store) SaveMedia(ctx context.Context, in SaveMediaInput) (uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var id uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT id FROM sc_media_files WHERE file_path = ?1`, in.FilePath).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		id = uuid.New()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sc_media_files (id, movie_id, episode_id, file_path, size_bytes, mtime)
			VALUES (?1, ?2, ?3, ?4, ?5, ?6)`,
			id, in.MovieID, in.EpisodeID, in.FilePath, in.SizeBytes, in.ModTime.UTC())
	case err == nil:
		_, err = tx.ExecContext(ctx, `
			UPDATE sc_media_files SET movie_id = ?2, episode_id = ?3, size_bytes = ?4, mtime = ?5
			WHERE id = ?1`,
			id, in.MovieID, in.EpisodeID, in.SizeBytes, in.ModTime.UTC())
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("save media: %w", err)
	}

	if p := in.Probe; p != nil {
		_, err = tx.ExecContext(ctx, `
			UPDATE sc_media_files SET
			  container = ?2, duration_sec = ?3, bitrate = ?4, video_codec = ?5, video_profile = ?6,
			  width = ?7, height = ?8, pix_fmt = ?9, probed_at = CURRENT_TIMESTAMP, probe_error = ?10
			WHERE id = ?1`,
			id, nullStr(p.Container), nullFloat(p.DurationSec), nullInt64(p.Bitrate), nullStr(p.VideoCodec),
			nullStr(p.VideoProfile), nullInt(p.Width), nullInt(p.Height), nullStr(p.PixFmt), nullStr(p.Error))
		if err != nil {
			return uuid.Nil, fmt.Errorf("save probe: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sc_streams WHERE media_file_id = ?1`, id); err != nil {
			return uuid.Nil, err
		}
		for _, st := range p.Streams {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO sc_streams (media_file_id, idx, kind, codec, language, title, channels, is_default, is_forced)
				VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)`,
				id, st.Index, st.Kind, st.Codec, st.Language, st.Title, st.Channels, st.IsDefault, st.IsForced)
			if err != nil {
				return uuid.Nil, fmt.Errorf("save stream: %w", err)
			}
		}
	}

	if err := replaceSubtitles(ctx, tx, id, in.Subtitles); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit()
}

// replaceSubtitles srovná externí titulky souboru se stavem na disku a
// ponechá ID titulků, které se nezměnily (přehrávač je může mít vybrané).
func replaceSubtitles(ctx context.Context, tx *sql.Tx, mediaID uuid.UUID, subs []SubtitleFile) error {
	keep := make(map[string]bool, len(subs))
	for _, sub := range subs {
		keep[sub.FilePath] = true
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sc_external_subtitles (id, media_file_id, file_path, language, format, is_forced)
			VALUES (?1, ?2, ?3, ?4, ?5, ?6)
			ON CONFLICT (file_path) DO UPDATE SET
			  media_file_id = excluded.media_file_id, language = excluded.language,
			  format = excluded.format, is_forced = excluded.is_forced`,
			uuid.New(), mediaID, sub.FilePath, nullStr(sub.Language), sub.Format, sub.Forced)
		if err != nil {
			return fmt.Errorf("save subtitle: %w", err)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, file_path FROM sc_external_subtitles WHERE media_file_id = ?1`, mediaID)
	if err != nil {
		return err
	}
	var stale []uuid.UUID
	for rows.Next() {
		var (
			id uuid.UUID
			p  string
		)
		if err := rows.Scan(&id, &p); err != nil {
			rows.Close()
			return err
		}
		if !keep[p] {
			stale = append(stale, id)
		}
	}
	rows.Close()
	for _, id := range stale {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sc_external_subtitles WHERE id = ?1`, id); err != nil {
			return err
		}
	}
	return rows.Err()
}

func nullFloat(f float64) any {
	if f == 0 {
		return nil
	}
	return f
}

func nullInt64(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
