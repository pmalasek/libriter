package storage

import (
	"context"
	"errors"
	"fmt"

	"libriter/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// --- Authors ---

func (s *Store) CreateAuthor(ctx context.Context, name string, bio, imagePath *string) (*model.Author, error) {
	const q = `
		INSERT INTO library.authors (name, bio, image_path)
		VALUES ($1, $2, $3)
		RETURNING id, name, bio, image_path, created_at`

	row := s.db.QueryRow(ctx, q, name, bio, imagePath)
	return scanAuthor(row)
}

func (s *Store) GetAuthor(ctx context.Context, id uuid.UUID) (*model.Author, error) {
	const q = `SELECT id, name, bio, image_path, created_at FROM library.authors WHERE id = $1`
	row := s.db.QueryRow(ctx, q, id)
	a, err := scanAuthor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (s *Store) ListAuthors(ctx context.Context) ([]model.Author, error) {
	const q = `SELECT id, name, bio, image_path, created_at FROM library.authors ORDER BY name`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list authors: %w", err)
	}
	defer rows.Close()

	var authors []model.Author
	for rows.Next() {
		a, err := scanAuthor(rows)
		if err != nil {
			return nil, err
		}
		authors = append(authors, *a)
	}
	return authors, rows.Err()
}

func (s *Store) UpdateAuthor(ctx context.Context, id uuid.UUID, name string, bio, imagePath *string) (*model.Author, error) {
	const q = `
		UPDATE library.authors
		SET name = $2, bio = $3, image_path = $4
		WHERE id = $1
		RETURNING id, name, bio, image_path, created_at`

	row := s.db.QueryRow(ctx, q, id, name, bio, imagePath)
	a, err := scanAuthor(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (s *Store) DeleteAuthor(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM library.authors WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete author: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOrCreateAuthor najde autora podle jména nebo ho vytvoří.
// Volající musí serializovat přístupy (scanner používá ingestMu).
func (s *Store) GetOrCreateAuthor(ctx context.Context, name string) (*model.Author, error) {
	const q = `SELECT id, name, bio, image_path, created_at FROM library.authors WHERE name = $1 LIMIT 1`
	a, err := scanAuthor(s.db.QueryRow(ctx, q, name))
	if err == nil {
		return a, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get author by name: %w", err)
	}
	return s.CreateAuthor(ctx, name, nil, nil)
}

func scanAuthor(row scanner) (*model.Author, error) {
	var a model.Author
	err := row.Scan(&a.ID, &a.Name, &a.Bio, &a.ImagePath, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// --- Series ---

func (s *Store) CreateSeries(ctx context.Context, title string, description *string) (*model.Series, error) {
	const q = `
		INSERT INTO library.series (title, description)
		VALUES ($1, $2)
		RETURNING id, title, description, created_at`

	row := s.db.QueryRow(ctx, q, title, description)
	return scanSeries(row)
}

func (s *Store) GetSeries(ctx context.Context, id uuid.UUID) (*model.Series, error) {
	const q = `SELECT id, title, description, created_at FROM library.series WHERE id = $1`
	row := s.db.QueryRow(ctx, q, id)
	sr, err := scanSeries(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sr, err
}

func (s *Store) ListSeries(ctx context.Context) ([]model.Series, error) {
	const q = `SELECT id, title, description, created_at FROM library.series ORDER BY title`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	defer rows.Close()

	var list []model.Series
	for rows.Next() {
		sr, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *sr)
	}
	return list, rows.Err()
}

func (s *Store) UpdateSeries(ctx context.Context, id uuid.UUID, title string, description *string) (*model.Series, error) {
	const q = `
		UPDATE library.series SET title = $2, description = $3
		WHERE id = $1
		RETURNING id, title, description, created_at`

	row := s.db.QueryRow(ctx, q, id, title, description)
	sr, err := scanSeries(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sr, err
}

func (s *Store) DeleteSeries(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM library.series WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete series: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSeries(row scanner) (*model.Series, error) {
	var s model.Series
	err := row.Scan(&s.ID, &s.Title, &s.Description, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
