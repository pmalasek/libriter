package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// SetAspectRatio nastaví ruční poměr stran filmu (movieID) nebo seriálu
// (showID); nil vrátí poměr podle souboru. updated_at se nemění – slouží
// jako cache-buster obrázků titulu.
func (s *Store) SetAspectRatio(ctx context.Context, movieID, showID *uuid.UUID, ratio *string) error {
	table, id := "sc_movies", movieID
	if showID != nil {
		table, id = "sc_shows", showID
	}
	res, err := s.db.ExecContext(ctx, `UPDATE `+table+` SET aspect_ratio = ?2 WHERE id = ?1`, id, ratio)
	if err != nil {
		return fmt.Errorf("aspect ratio: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MediaAspectRatio vrátí ruční poměr stran titulu, ke kterému soubor patří
// (film, nebo seriál dílu); nil = podle souboru.
func (s *Store) MediaAspectRatio(ctx context.Context, mediaID uuid.UUID) (*string, error) {
	var ratio sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(m.aspect_ratio, sh.aspect_ratio) FROM sc_media_files f
		LEFT JOIN sc_movies m ON m.id = f.movie_id
		LEFT JOIN sc_episodes e ON e.id = f.episode_id
		LEFT JOIN sc_seasons se ON se.id = e.season_id
		LEFT JOIN sc_shows sh ON sh.id = se.show_id
		WHERE f.id = ?1`, mediaID).Scan(&ratio)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("media aspect ratio: %w", err)
	}
	return ptrString(ratio), nil
}
