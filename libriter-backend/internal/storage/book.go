package storage

import (
	"context"
	"errors"
	"fmt"

	"libriter/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type BookInput struct {
	AuthorID        uuid.UUID
	SeriesID        *uuid.UUID
	SeriesPosition  *int16
	Title           string
	Narrator        *string
	DurationSeconds int
	FilePath        string
	CoverPath       *string
	Language        string
	Description     *string
	InternalRating  *int16
}

func (s *Store) CreateBook(ctx context.Context, in BookInput) (*model.Book, error) {
	const q = `
		INSERT INTO library.books
			(author_id, series_id, series_position, title, narrator,
			 duration_seconds, file_path, cover_path, language, description, internal_rating)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, author_id, series_id, series_position, title, narrator,
		          duration_seconds, file_path, cover_path, language, description,
		          internal_rating, created_at, updated_at`

	row := s.db.QueryRow(ctx, q,
		in.AuthorID, in.SeriesID, in.SeriesPosition, in.Title, in.Narrator,
		in.DurationSeconds, in.FilePath, in.CoverPath, in.Language, in.Description, in.InternalRating,
	)
	return scanBook(row)
}

func (s *Store) GetBook(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	const q = `
		SELECT id, author_id, series_id, series_position, title, narrator,
		       duration_seconds, file_path, cover_path, language, description,
		       internal_rating, created_at, updated_at
		FROM library.books WHERE id = $1`

	row := s.db.QueryRow(ctx, q, id)
	b, err := scanBook(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (s *Store) ListBooks(ctx context.Context) ([]model.Book, error) {
	const q = `
		SELECT id, author_id, series_id, series_position, title, narrator,
		       duration_seconds, file_path, cover_path, language, description,
		       internal_rating, created_at, updated_at
		FROM library.books
		ORDER BY title`

	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	defer rows.Close()

	var books []model.Book
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, *b)
	}
	return books, rows.Err()
}

func (s *Store) UpdateBook(ctx context.Context, id uuid.UUID, in BookInput) (*model.Book, error) {
	const q = `
		UPDATE library.books SET
			author_id = $2, series_id = $3, series_position = $4, title = $5,
			narrator = $6, duration_seconds = $7, file_path = $8, cover_path = $9,
			language = $10, description = $11, internal_rating = $12
		WHERE id = $1
		RETURNING id, author_id, series_id, series_position, title, narrator,
		          duration_seconds, file_path, cover_path, language, description,
		          internal_rating, created_at, updated_at`

	row := s.db.QueryRow(ctx, q,
		id, in.AuthorID, in.SeriesID, in.SeriesPosition, in.Title, in.Narrator,
		in.DurationSeconds, in.FilePath, in.CoverPath, in.Language, in.Description, in.InternalRating,
	)
	b, err := scanBook(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (s *Store) DeleteBook(ctx context.Context, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM library.books WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete book: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// BookExistsByFilePath vrátí true, pokud v DB existuje kniha s danou cestou (adresář nebo soubor).
func (s *Store) BookExistsByFilePath(ctx context.Context, filePath string) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM library.books WHERE file_path = $1)`
	if err := s.db.QueryRow(ctx, q, filePath).Scan(&exists); err != nil {
		return false, fmt.Errorf("check book exists: %w", err)
	}
	return exists, nil
}

// GetBookByTitleAndAuthorID najde knihu podle názvu a ID autora.
// Používá se pro seskupování souborů podle album tagu.
func (s *Store) GetBookByTitleAndAuthorID(ctx context.Context, title string, authorID uuid.UUID) (*model.Book, error) {
	const q = `
		SELECT id, author_id, series_id, series_position, title, narrator,
		       duration_seconds, file_path, cover_path, language, description,
		       internal_rating, created_at, updated_at
		FROM library.books
		WHERE title = $1 AND author_id = $2
		LIMIT 1`

	row := s.db.QueryRow(ctx, q, title, authorID)
	b, err := scanBook(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

// GetBookByDirPath najde knihu podle relativní cesty k adresáři (books.file_path).
func (s *Store) GetBookByDirPath(ctx context.Context, dirPath string) (*model.Book, error) {
	const q = `
		SELECT id, author_id, series_id, series_position, title, narrator,
		       duration_seconds, file_path, cover_path, language, description,
		       internal_rating, created_at, updated_at
		FROM library.books WHERE file_path = $1 LIMIT 1`

	row := s.db.QueryRow(ctx, q, dirPath)
	b, err := scanBook(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

// UpdateBookDuration nastaví celkovou délku knihy (v sekundách).
func (s *Store) UpdateBookDuration(ctx context.Context, bookID uuid.UUID, durationSeconds int) error {
	const q = `UPDATE library.books SET duration_seconds = $2 WHERE id = $1`
	tag, err := s.db.Exec(ctx, q, bookID, durationSeconds)
	if err != nil {
		return fmt.Errorf("update book duration: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanBook(row scanner) (*model.Book, error) {
	var b model.Book
	err := row.Scan(
		&b.ID, &b.AuthorID, &b.SeriesID, &b.SeriesPosition, &b.Title, &b.Narrator,
		&b.DurationSeconds, &b.FilePath, &b.CoverPath, &b.Language, &b.Description,
		&b.InternalRating, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
