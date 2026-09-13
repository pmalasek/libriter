package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"libriter/internal/model"

	"github.com/google/uuid"
)

type BookInput struct {
	AuthorIDs       []uuid.UUID // pořadí určuje book_authors.position (první = hlavní autor)
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

const bookColumns = `id, series_id, series_position, title, narrator,
	       duration_seconds, file_path, cover_path, language, description,
	       internal_rating, created_at, updated_at`

func (s *Store) CreateBook(ctx context.Context, in BookInput) (*model.Book, error) {
	const q = `
		INSERT INTO books
			(id, series_id, series_position, title, narrator,
			 duration_seconds, file_path, cover_path, language, description, internal_rating)
		VALUES (?1,?2,?3,?4,?5,?6,?7,?8,?9,?10,?11)
		RETURNING ` + bookColumns

	return s.writeBook(ctx, in, func(tx *sql.Tx) (*model.Book, error) {
		row := tx.QueryRowContext(ctx, q,
			uuid.New(), in.SeriesID, in.SeriesPosition, in.Title, in.Narrator,
			in.DurationSeconds, in.FilePath, in.CoverPath, in.Language, in.Description,
			in.InternalRating,
		)
		return scanBook(row)
	})
}

func (s *Store) UpdateBook(ctx context.Context, id uuid.UUID, in BookInput) (*model.Book, error) {
	const q = `
		UPDATE books SET
			series_id = ?2, series_position = ?3, title = ?4,
			narrator = ?5, duration_seconds = ?6, file_path = ?7, cover_path = ?8,
			language = ?9, description = ?10, internal_rating = ?11,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?1
		RETURNING ` + bookColumns

	return s.writeBook(ctx, in, func(tx *sql.Tx) (*model.Book, error) {
		row := tx.QueryRowContext(ctx, q,
			id, in.SeriesID, in.SeriesPosition, in.Title, in.Narrator,
			in.DurationSeconds, in.FilePath, in.CoverPath, in.Language, in.Description,
			in.InternalRating,
		)
		b, err := scanBook(row)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return b, err
	})
}

// writeBook provede zápis knihy a přepis jejích autorů v jedné transakci.
func (s *Store) writeBook(ctx context.Context, in BookInput, write func(*sql.Tx) (*model.Book, error)) (*model.Book, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("write book: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	book, err := write(tx)
	if err != nil {
		return nil, err
	}

	if err := setBookAuthors(ctx, tx, book.ID, in.AuthorIDs); err != nil {
		return nil, err
	}
	if book.Authors, err = loadBookAuthors(ctx, tx, book.ID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("write book: commit: %w", err)
	}
	return book, nil
}

func (s *Store) GetBook(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	const q = `SELECT ` + bookColumns + ` FROM books WHERE id = ?1`

	row := s.db.QueryRowContext(ctx, q, id)
	b, err := scanBook(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if b.Authors, err = loadBookAuthors(ctx, s.db, b.ID); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) ListBooks(ctx context.Context) ([]model.Book, error) {
	const q = `SELECT ` + bookColumns + ` FROM books ORDER BY title`

	rows, err := s.db.QueryContext(ctx, q)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Autoři všech knih jedním dotazem – jinak by seznam dělal dotaz na knihu.
	byBook, err := loadAllBookAuthors(ctx, s.db)
	if err != nil {
		return nil, err
	}
	for i := range books {
		books[i].Authors = byBook[books[i].ID]
	}
	return books, nil
}

func (s *Store) DeleteBook(ctx context.Context, id uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM books WHERE id = ?1`, id)
	if err != nil {
		return fmt.Errorf("delete book: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

// BookExistsByFilePath vrátí true, pokud v DB existuje kniha s danou cestou (adresář nebo soubor).
func (s *Store) BookExistsByFilePath(ctx context.Context, filePath string) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM books WHERE file_path = ?1)`
	if err := s.db.QueryRowContext(ctx, q, filePath).Scan(&exists); err != nil {
		return false, fmt.Errorf("check book exists: %w", err)
	}
	return exists, nil
}

// GetBookByTitleAndAuthorID najde knihu podle názvu a ID jednoho z jejích autorů.
// Používá se pro seskupování souborů podle album tagu.
func (s *Store) GetBookByTitleAndAuthorID(ctx context.Context, title string, authorID uuid.UUID) (*model.Book, error) {
	const q = `SELECT ` + bookColumns + ` FROM books
		WHERE title = ?1
		  AND id IN (SELECT book_id FROM book_authors WHERE author_id = ?2)
		LIMIT 1`

	return s.getBookBy(ctx, q, title, authorID)
}

// GetBookByDirPath najde knihu podle relativní cesty k adresáři (books.file_path).
func (s *Store) GetBookByDirPath(ctx context.Context, dirPath string) (*model.Book, error) {
	const q = `SELECT ` + bookColumns + ` FROM books WHERE file_path = ?1 LIMIT 1`

	return s.getBookBy(ctx, q, dirPath)
}

func (s *Store) getBookBy(ctx context.Context, query string, args ...any) (*model.Book, error) {
	b, err := scanBook(s.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if b.Authors, err = loadBookAuthors(ctx, s.db, b.ID); err != nil {
		return nil, err
	}
	return b, nil
}

// UpdateBookCoverPath nastaví cestu k obálce (relativní ke COVER_ROOT).
func (s *Store) UpdateBookCoverPath(ctx context.Context, bookID uuid.UUID, coverPath string) error {
	const q = `UPDATE books SET cover_path = ?2, updated_at = CURRENT_TIMESTAMP WHERE id = ?1`
	res, err := s.db.ExecContext(ctx, q, bookID, coverPath)
	if err != nil {
		return fmt.Errorf("update book cover path: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateBookDuration nastaví celkovou délku knihy (v sekundách).
func (s *Store) UpdateBookDuration(ctx context.Context, bookID uuid.UUID, durationSeconds int) error {
	const q = `UPDATE books SET duration_seconds = ?2, updated_at = CURRENT_TIMESTAMP WHERE id = ?1`
	res, err := s.db.ExecContext(ctx, q, bookID, durationSeconds)
	if err != nil {
		return fmt.Errorf("update book duration: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

// --- book_authors ---

// setBookAuthors nahradí seznam autorů knihy; pořadí se uloží do position.
func setBookAuthors(ctx context.Context, q querier, bookID uuid.UUID, authorIDs []uuid.UUID) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM book_authors WHERE book_id = ?1`, bookID); err != nil {
		return fmt.Errorf("clear book authors: %w", err)
	}

	const ins = `INSERT INTO book_authors (book_id, author_id, position) VALUES (?1, ?2, ?3)`
	seen := make(map[uuid.UUID]bool, len(authorIDs))
	position := 0
	for _, id := range authorIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		position++
		if _, err := q.ExecContext(ctx, ins, bookID, id, position); err != nil {
			return fmt.Errorf("link book author: %w", err)
		}
	}
	return nil
}

func loadBookAuthors(ctx context.Context, q querier, bookID uuid.UUID) ([]model.Author, error) {
	const sel = `
		SELECT a.id, a.first_name, a.middle_name, a.last_name, a.name,
		       a.bio, a.image_path, a.created_at
		FROM   book_authors ba
		JOIN   authors a ON a.id = ba.author_id
		WHERE  ba.book_id = ?1
		ORDER BY ba.position`

	rows, err := q.QueryContext(ctx, sel, bookID)
	if err != nil {
		return nil, fmt.Errorf("load book authors: %w", err)
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

// loadAllBookAuthors načte autory všech knih do mapy podle ID knihy.
func loadAllBookAuthors(ctx context.Context, q querier) (map[uuid.UUID][]model.Author, error) {
	const sel = `
		SELECT ba.book_id, a.id, a.first_name, a.middle_name, a.last_name, a.name,
		       a.bio, a.image_path, a.created_at
		FROM   book_authors ba
		JOIN   authors a ON a.id = ba.author_id
		ORDER BY ba.book_id, ba.position`

	rows, err := q.QueryContext(ctx, sel)
	if err != nil {
		return nil, fmt.Errorf("load book authors: %w", err)
	}
	defer rows.Close()

	byBook := make(map[uuid.UUID][]model.Author)
	for rows.Next() {
		var (
			bookID uuid.UUID
			a      model.Author
		)
		if err := rows.Scan(&bookID, &a.ID, &a.FirstName, &a.MiddleName, &a.LastName,
			&a.Name, &a.Bio, &a.ImagePath, &a.CreatedAt); err != nil {
			return nil, err
		}
		byBook[bookID] = append(byBook[bookID], a)
	}
	return byBook, rows.Err()
}

func scanBook(row scanner) (*model.Book, error) {
	var b model.Book
	err := row.Scan(
		&b.ID, &b.SeriesID, &b.SeriesPosition, &b.Title, &b.Narrator,
		&b.DurationSeconds, &b.FilePath, &b.CoverPath, &b.Language, &b.Description,
		&b.InternalRating, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
