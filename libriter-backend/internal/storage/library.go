package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"libriter/internal/model"

	"github.com/google/uuid"
)

// --- Authors ---

// AuthorInput jsou zapisovatelná pole autora. Celé jméno (sloupec name)
// se dopočítává z částí, nezadává se.
type AuthorInput struct {
	Name      model.AuthorName
	Bio       *string
	ImagePath *string
}

const authorColumns = `id, first_name, middle_name, last_name, name, bio, image_path, created_at`

func (s *Store) CreateAuthor(ctx context.Context, in AuthorInput) (*model.Author, error) {
	const q = `
		INSERT INTO authors (id, first_name, middle_name, last_name, name, bio, image_path)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)
		RETURNING ` + authorColumns

	row := s.db.QueryRowContext(ctx, q,
		uuid.New(), in.Name.First, in.Name.Middle, in.Name.Last, in.Name.Full(),
		in.Bio, in.ImagePath,
	)
	a, err := scanAuthor(row)
	if isUniqueViolation(err) {
		return nil, ErrConflict
	}
	return a, err
}

func (s *Store) GetAuthor(ctx context.Context, id uuid.UUID) (*model.Author, error) {
	const q = `SELECT ` + authorColumns + ` FROM authors WHERE id = ?1`
	row := s.db.QueryRowContext(ctx, q, id)
	a, err := scanAuthor(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// ListAuthors vrací autory seřazené jako v knihovně – podle příjmení.
func (s *Store) ListAuthors(ctx context.Context) ([]model.Author, error) {
	const q = `SELECT ` + authorColumns + ` FROM authors ORDER BY last_name, first_name, middle_name`
	rows, err := s.db.QueryContext(ctx, q)
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

func (s *Store) UpdateAuthor(ctx context.Context, id uuid.UUID, in AuthorInput) (*model.Author, error) {
	const q = `
		UPDATE authors
		SET first_name = ?2, middle_name = ?3, last_name = ?4, name = ?5,
		    bio = ?6, image_path = ?7
		WHERE id = ?1
		RETURNING ` + authorColumns

	row := s.db.QueryRowContext(ctx, q,
		id, in.Name.First, in.Name.Middle, in.Name.Last, in.Name.Full(),
		in.Bio, in.ImagePath,
	)
	a, err := scanAuthor(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case isUniqueViolation(err):
		return nil, ErrConflict
	}
	return a, err
}

func (s *Store) DeleteAuthor(ctx context.Context, id uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM authors WHERE id = ?1`, id)
	if isForeignKeyViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("delete author: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOrCreateAuthor najde autora podle rozdělených částí jména nebo ho vytvoří.
// Volající musí serializovat přístupy (scanner používá ingestMu).
func (s *Store) GetOrCreateAuthor(ctx context.Context, name model.AuthorName) (*model.Author, error) {
	const q = `SELECT ` + authorColumns + ` FROM authors
		WHERE first_name = ?1 AND middle_name = ?2 AND last_name = ?3 LIMIT 1`

	a, err := scanAuthor(s.db.QueryRowContext(ctx, q, name.First, name.Middle, name.Last))
	if err == nil {
		return a, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get author by name: %w", err)
	}
	return s.CreateAuthor(ctx, AuthorInput{Name: name})
}

// GetOrCreateAuthors zpracuje celý seznam jmen a zachová jeho pořadí.
func (s *Store) GetOrCreateAuthors(ctx context.Context, names []model.AuthorName) ([]model.Author, error) {
	authors := make([]model.Author, 0, len(names))
	for _, name := range names {
		a, err := s.GetOrCreateAuthor(ctx, name)
		if err != nil {
			return nil, err
		}
		authors = append(authors, *a)
	}
	return authors, nil
}

func scanAuthor(row scanner) (*model.Author, error) {
	var a model.Author
	err := row.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.Name,
		&a.Bio, &a.ImagePath, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// --- Series ---

func (s *Store) CreateSeries(ctx context.Context, title string, description *string) (*model.Series, error) {
	const q = `
		INSERT INTO series (id, title, description)
		VALUES (?1, ?2, ?3)
		RETURNING id, title, description, created_at`

	row := s.db.QueryRowContext(ctx, q, uuid.New(), title, description)
	return scanSeries(row)
}

func (s *Store) GetSeries(ctx context.Context, id uuid.UUID) (*model.Series, error) {
	const q = `SELECT id, title, description, created_at FROM series WHERE id = ?1`
	row := s.db.QueryRowContext(ctx, q, id)
	sr, err := scanSeries(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sr, err
}

func (s *Store) ListSeries(ctx context.Context) ([]model.Series, error) {
	const q = `SELECT id, title, description, created_at FROM series ORDER BY title`
	rows, err := s.db.QueryContext(ctx, q)
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
		UPDATE series SET title = ?2, description = ?3
		WHERE id = ?1
		RETURNING id, title, description, created_at`

	row := s.db.QueryRowContext(ctx, q, id, title, description)
	sr, err := scanSeries(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return sr, err
}

func (s *Store) DeleteSeries(ctx context.Context, id uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM series WHERE id = ?1`, id)
	if err != nil {
		return fmt.Errorf("delete series: %w", err)
	}
	if rowsAffected(res) == 0 {
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
