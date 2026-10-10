package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EpisodeFileRow je soubor dílu seriálu pro kontrolu číslování a editor složky.
type EpisodeFileRow struct {
	ShowID     uuid.UUID
	ShowTitle  string
	SourceKey  string
	PosterPath *string
	UpdatedAt  time.Time
	Season     int
	Episode    int
	EpisodeID  uuid.UUID
	MediaID    uuid.UUID
	FilePath   string
	// EmbeddedTitle je název vložený v souboru ("" = žádný nebo nezjištěný).
	EmbeddedTitle string
}

const episodeFilesSelect = `
	SELECT sh.id, sh.title, sh.source_key, sh.poster_path, sh.updated_at, se.number, e.number, e.id, m.id, m.file_path, COALESCE(m.embedded_title, '')
	FROM sc_media_files m
	JOIN sc_episodes e ON e.id = m.episode_id
	JOIN sc_seasons se ON se.id = e.season_id
	JOIN sc_shows sh ON sh.id = se.show_id`

const episodeFilesOrder = ` ORDER BY sh.title COLLATE NOCASE, sh.id, se.number, e.number, m.file_path`

// EpisodeFiles vrátí všechny soubory dílů seřazené podle seriálu, řady,
// dílu a cesty. Díly bez souboru se nevracejí.
func (s *Store) EpisodeFiles(ctx context.Context) ([]EpisodeFileRow, error) {
	rows, err := s.db.QueryContext(ctx, episodeFilesSelect+episodeFilesOrder)
	if err != nil {
		return nil, fmt.Errorf("list episode files: %w", err)
	}
	return scanEpisodeFiles(rows)
}

// ShowFiles vrátí soubory dílů jednoho seriálu seřazené podle řady, dílu a cesty.
func (s *Store) ShowFiles(ctx context.Context, showID uuid.UUID) ([]EpisodeFileRow, error) {
	rows, err := s.db.QueryContext(ctx, episodeFilesSelect+` WHERE sh.id = ?1`+episodeFilesOrder, showID)
	if err != nil {
		return nil, fmt.Errorf("list show files: %w", err)
	}
	return scanEpisodeFiles(rows)
}

func scanEpisodeFiles(rows *sql.Rows) ([]EpisodeFileRow, error) {
	defer rows.Close()
	var out []EpisodeFileRow
	for rows.Next() {
		var r EpisodeFileRow
		if err := rows.Scan(&r.ShowID, &r.ShowTitle, &r.SourceKey, &r.PosterPath, &r.UpdatedAt, &r.Season, &r.Episode,
			&r.EpisodeID, &r.MediaID, &r.FilePath, &r.EmbeddedTitle); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RenameMedia přejmenuje soubor (a jeho externí titulky, subs je stará →
// nová cesta) a přeřadí ho k dílu. Volá se před přejmenováním na disku, aby
// sledování změn starou cestu už nenašlo a soubor nesmazalo.
func (s *Store) RenameMedia(ctx context.Context, mediaID uuid.UUID, newPath string, episodeID uuid.UUID, subs map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `UPDATE sc_media_files SET file_path = ?2, episode_id = ?3 WHERE id = ?1`, mediaID, newPath, episodeID); err != nil {
		return fmt.Errorf("rename media: %w", err)
	}
	for old, new := range subs {
		if _, err := tx.ExecContext(ctx, `UPDATE sc_external_subtitles SET file_path = ?3 WHERE media_file_id = ?1 AND file_path = ?2`, mediaID, old, new); err != nil {
			return fmt.Errorf("rename subtitle: %w", err)
		}
	}
	return tx.Commit()
}

// MovieFileRow je soubor filmu, který má souborů víc (kontrola sloučených filmů).
type MovieFileRow struct {
	MovieID    uuid.UUID
	Title      string
	SourceKey  string
	PosterPath *string
	UpdatedAt  time.Time
	MediaID    uuid.UUID
	FilePath   string
}

// MovieFiles vrátí soubory filmů s více než jedním souborem, seřazené
// podle filmu a cesty.
func (s *Store) MovieFiles(ctx context.Context) ([]MovieFileRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT mv.id, mv.title, mv.source_key, mv.poster_path, mv.updated_at, m.id, m.file_path
		FROM sc_media_files m
		JOIN sc_movies mv ON mv.id = m.movie_id
		WHERE mv.id IN (SELECT movie_id FROM sc_media_files WHERE movie_id IS NOT NULL GROUP BY movie_id HAVING COUNT(*) > 1)
		ORDER BY mv.title COLLATE NOCASE, mv.id, m.file_path`)
	if err != nil {
		return nil, fmt.Errorf("list movie files: %w", err)
	}
	defer rows.Close()
	var out []MovieFileRow
	for rows.Next() {
		var r MovieFileRow
		if err := rows.Scan(&r.MovieID, &r.Title, &r.SourceKey, &r.PosterPath, &r.UpdatedAt, &r.MediaID, &r.FilePath); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
