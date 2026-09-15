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
//
// Identitou kapitoly je file_path, ne pozice: pozice se u multi-disk vydání
// opakuje (track 1/24 na CD1 i CD2) a kolize v (book_id, position) by přepsala
// cestu jiného souboru – ten by pak při dalším scanu chyběl a ingestoval se
// znovu dokola. Volající proto musí pozici předem uvolnit
// (NextFreeChapterPosition).
func (s *Store) UpsertChapter(ctx context.Context, in ChapterInput) (*model.Chapter, error) {
	const q = `
		INSERT INTO chapters
			(id, book_id, position, title, file_path, start_offset_seconds, duration_seconds)
		VALUES (?1, ?2, ?3, ?4, ?5, 0, ?6)
		ON CONFLICT (file_path) DO UPDATE SET
			book_id          = excluded.book_id,
			position         = excluded.position,
			title            = excluded.title,
			duration_seconds = excluded.duration_seconds
		RETURNING id, book_id, position, title, file_path, 0, duration_seconds`

	row := s.db.QueryRowContext(ctx, q,
		uuid.New(), in.BookID, in.Position, in.Title, in.FilePath, in.DurationSeconds,
	)
	return scanChapter(row)
}

// NextFreeChapterPosition vrátí nejbližší pozici >= desired, kterou v dané knize
// neobsazuje jiný soubor. Vlastní řádek (stejná file_path) se nepočítá jako
// kolize, takže opakovaný scan téhož souboru pozici neposouvá.
func (s *Store) NextFreeChapterPosition(
	ctx context.Context,
	bookID uuid.UUID,
	desired int,
	filePath string,
) (int, error) {
	if desired < 1 {
		desired = 1
	}

	const q = `
		SELECT position FROM chapters
		WHERE book_id = ?1 AND position >= ?2 AND file_path <> ?3
		ORDER BY position`

	rows, err := s.db.QueryContext(ctx, q, bookID, desired, filePath)
	if err != nil {
		return 0, fmt.Errorf("next free chapter position: %w", err)
	}
	defer rows.Close()

	// Obsazené pozice jdou vzestupně; první mezera v řadě je hledaná pozice.
	free := desired
	for rows.Next() {
		var taken int
		if err := rows.Scan(&taken); err != nil {
			return 0, fmt.Errorf("next free chapter position: %w", err)
		}
		if taken > free {
			break
		}
		free = taken + 1
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("next free chapter position: %w", err)
	}
	return free, nil
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

// DeleteChaptersByBookID smaže všechny kapitoly knihy a vrátí jejich počet.
// Kapitoly jsou odvozená data – scanner je při dalším průchodu načte znovu.
func (s *Store) DeleteChaptersByBookID(ctx context.Context, bookID uuid.UUID) (int, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM chapters WHERE book_id = ?1`, bookID)
	if err != nil {
		return 0, fmt.Errorf("delete chapters: %w", err)
	}
	return int(rowsAffected(res)), nil
}

// ChapterFile spojuje kapitolu s knihou, ke které patří.
type ChapterFile struct {
	BookID       uuid.UUID
	BookFilePath string // adresář knihy
	FilePath     string // cesta k audio souboru
}

// ListChapterFiles vrátí cesty všech kapitol i s adresářem jejich knihy.
// Slouží ke kontrole, že kapitoly v DB odpovídají souborům na disku.
func (s *Store) ListChapterFiles(ctx context.Context) ([]ChapterFile, error) {
	const q = `SELECT c.book_id, b.file_path, c.file_path
		FROM chapters c JOIN books b ON b.id = c.book_id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list chapter files: %w", err)
	}
	defer rows.Close()

	var files []ChapterFile
	for rows.Next() {
		var f ChapterFile
		if err := rows.Scan(&f.BookID, &f.BookFilePath, &f.FilePath); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
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
