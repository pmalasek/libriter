package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"libriter/internal/model"

	"github.com/google/uuid"
)

type ChapterInput struct {
	BookID          uuid.UUID
	Position        int
	Title           string
	FilePath        string
	DurationSeconds int
	// StartOffsetSeconds se neukládá – počítá se dynamicky při čtení (window funkce)
}

// UpsertChapter vloží nebo aktualizuje kapitolu (idempotentní – bezpečné při opakovaném scanu).
func (s *Store) UpsertChapter(ctx context.Context, in ChapterInput) (*model.Chapter, error) {
	const q = `
		INSERT INTO chapters
			(id, book_id, position, title, file_path, start_offset_seconds, duration_seconds)
		VALUES (?1, ?2, ?3, ?4, ?5, 0, ?6)
		ON CONFLICT (book_id, position) DO UPDATE SET
			title            = excluded.title,
			file_path        = excluded.file_path,
			duration_seconds = excluded.duration_seconds
		RETURNING id, book_id, position, title, file_path, 0, duration_seconds`

	row := s.db.QueryRowContext(ctx, q,
		uuid.New(), in.BookID, in.Position, in.Title, in.FilePath, in.DurationSeconds,
	)
	return scanChapter(row)
}

// CreateChapter přidá kapitolu ke knize.
func (s *Store) CreateChapter(ctx context.Context, in ChapterInput) (*model.Chapter, error) {
	const q = `
		INSERT INTO chapters
			(id, book_id, position, title, file_path, start_offset_seconds, duration_seconds)
		VALUES (?1, ?2, ?3, ?4, ?5, 0, ?6)
		RETURNING id, book_id, position, title, file_path, 0, duration_seconds`

	row := s.db.QueryRowContext(ctx, q,
		uuid.New(), in.BookID, in.Position, in.Title, in.FilePath, in.DurationSeconds,
	)
	return scanChapter(row)
}

// GetChaptersByBookID vrátí všechny kapitoly knihy seřazené podle pořadí.
// start_offset_seconds se počítá dynamicky jako součet delék předešlých kapitol.
func (s *Store) GetChaptersByBookID(ctx context.Context, bookID uuid.UUID) ([]model.Chapter, error) {
	const q = `
		SELECT
			id, book_id, position, title, file_path,
			COALESCE(
				SUM(duration_seconds) OVER (
					PARTITION BY book_id
					ORDER BY position
					ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
				), 0
			) AS start_offset_seconds,
			duration_seconds
		FROM chapters
		WHERE book_id = ?1
		ORDER BY position`

	rows, err := s.db.QueryContext(ctx, q, bookID)
	if err != nil {
		return nil, fmt.Errorf("get chapters: %w", err)
	}
	defer rows.Close()

	var chapters []model.Chapter
	for rows.Next() {
		c, err := scanChapter(rows)
		if err != nil {
			return nil, err
		}
		chapters = append(chapters, *c)
	}
	return chapters, rows.Err()
}

// ChapterExistsByFilePath vrátí true, pokud v DB existuje kapitola s danou cestou souboru.
func (s *Store) ChapterExistsByFilePath(ctx context.Context, filePath string) (bool, error) {
	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM chapters WHERE file_path = ?1)`
	if err := s.db.QueryRowContext(ctx, q, filePath).Scan(&exists); err != nil {
		return false, fmt.Errorf("check chapter exists: %w", err)
	}
	return exists, nil
}

// GetBookChapterStats vrátí počet kapitol a celkovou délku knihy.
func (s *Store) GetBookChapterStats(ctx context.Context, bookID uuid.UUID) (count int, totalDuration int, err error) {
	const q = `
		SELECT COUNT(*), COALESCE(SUM(duration_seconds), 0)
		FROM chapters
		WHERE book_id = ?1`
	err = s.db.QueryRowContext(ctx, q, bookID).Scan(&count, &totalDuration)
	return
}

func scanChapter(row scanner) (*model.Chapter, error) {
	var c model.Chapter
	err := row.Scan(
		&c.ID, &c.BookID, &c.Position, &c.Title,
		&c.FilePath, &c.StartOffsetSeconds, &c.DurationSeconds,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
