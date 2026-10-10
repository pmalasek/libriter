package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EpisodeFileRow je soubor dílu seriálu pro kontrolu číslování.
type EpisodeFileRow struct {
	ShowID     uuid.UUID
	ShowTitle  string
	SourceKey  string
	PosterPath *string
	UpdatedAt  time.Time
	Season     int
	Episode    int
	FilePath   string
}

// EpisodeFiles vrátí všechny soubory dílů seřazené podle seriálu, řady,
// dílu a cesty. Díly bez souboru se nevracejí.
func (s *Store) EpisodeFiles(ctx context.Context) ([]EpisodeFileRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sh.id, sh.title, sh.source_key, sh.poster_path, sh.updated_at, se.number, e.number, m.file_path
		FROM sc_media_files m
		JOIN sc_episodes e ON e.id = m.episode_id
		JOIN sc_seasons se ON se.id = e.season_id
		JOIN sc_shows sh ON sh.id = se.show_id
		ORDER BY sh.title COLLATE NOCASE, sh.id, se.number, e.number, m.file_path`)
	if err != nil {
		return nil, fmt.Errorf("list episode files: %w", err)
	}
	defer rows.Close()
	var out []EpisodeFileRow
	for rows.Next() {
		var r EpisodeFileRow
		if err := rows.Scan(&r.ShowID, &r.ShowTitle, &r.SourceKey, &r.PosterPath, &r.UpdatedAt, &r.Season, &r.Episode, &r.FilePath); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
