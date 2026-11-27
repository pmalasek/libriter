package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	corestorage "mediater/internal/core/storage"
)

// SetAspectRatio nastaví uživateli ruční poměr stran filmu (movieID) nebo
// seriálu (showID); nil jeho volbu smaže a platí zase sdílená výchozí
// hodnota titulu. Ostatních uživatelů se volba netýká. updated_at titulu se
// nemění – slouží jako cache-buster obrázků.
func (s *Store) SetAspectRatio(ctx context.Context, userID uuid.UUID, movieID, showID *uuid.UUID, ratio *string) error {
	table, id := "sc_movies", movieID
	if showID != nil {
		table, id = "sc_shows", showID
	}
	// Neexistující titul je 404 i při mazání volby (DELETE by prošel naprázdno).
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = ?1)`, id).Scan(&exists); err != nil {
		return fmt.Errorf("aspect ratio: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	_, err := s.db.ExecContext(ctx, `DELETE FROM sc_user_aspect
		WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND show_id = ?3))`,
		userID, movieID, showID)
	if err == nil && ratio != nil {
		_, err = s.db.ExecContext(ctx, `INSERT INTO sc_user_aspect (user_id, movie_id, show_id, aspect_ratio)
			VALUES (?1, ?2, ?3, ?4)`, userID, movieID, showID, *ratio)
	}
	if err != nil {
		if corestorage.IsForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("aspect ratio: %w", err)
	}
	return nil
}

// UserAspectRatio vrátí poměr stran, který si uživatel zvolil pro film nebo
// seriál; nil = nic nezvolil (platí sdílená hodnota titulu).
func (s *Store) UserAspectRatio(ctx context.Context, userID uuid.UUID, movieID, showID *uuid.UUID) (*string, error) {
	var ratio sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT aspect_ratio FROM sc_user_aspect
		WHERE user_id = ?1 AND ((?2 IS NOT NULL AND movie_id = ?2) OR (?3 IS NOT NULL AND show_id = ?3))`,
		userID, movieID, showID).Scan(&ratio)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("user aspect ratio: %w", err)
	}
	return ptrString(ratio), nil
}

// MediaAspectRatio vrátí ruční poměr stran titulu, ke kterému soubor patří
// (film, nebo seriál dílu): volbu uživatele, jinak sdílenou výchozí hodnotu
// titulu; nil = podle souboru.
func (s *Store) MediaAspectRatio(ctx context.Context, userID, mediaID uuid.UUID) (*string, error) {
	var ratio sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(ua.aspect_ratio, m.aspect_ratio, sh.aspect_ratio) FROM sc_media_files f
		LEFT JOIN sc_movies m ON m.id = f.movie_id
		LEFT JOIN sc_episodes e ON e.id = f.episode_id
		LEFT JOIN sc_seasons se ON se.id = e.season_id
		LEFT JOIN sc_shows sh ON sh.id = se.show_id
		LEFT JOIN sc_user_aspect ua ON ua.user_id = ?2
		     AND ((m.id IS NOT NULL AND ua.movie_id = m.id) OR (sh.id IS NOT NULL AND ua.show_id = sh.id))
		WHERE f.id = ?1`, mediaID, userID).Scan(&ratio)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("media aspect ratio: %w", err)
	}
	return ptrString(ratio), nil
}
