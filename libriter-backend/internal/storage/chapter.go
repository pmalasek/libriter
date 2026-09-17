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
	// SizeBytes je velikost souboru na disku; 0 = neznámá.
	SizeBytes int64
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
			(id, book_id, position, title, file_path, start_offset_seconds, duration_seconds, size_bytes)
		VALUES (?1, ?2, ?3, ?4, ?5, 0, ?6, ?7)
		ON CONFLICT (file_path) DO UPDATE SET
			book_id          = excluded.book_id,
			position         = excluded.position,
			title            = excluded.title,
			duration_seconds = excluded.duration_seconds,
			size_bytes       = excluded.size_bytes
		RETURNING id, book_id, position, title, file_path, 0, duration_seconds, size_bytes`

	row := s.db.QueryRowContext(ctx, q,
		uuid.New(), in.BookID, in.Position, in.Title, in.FilePath, in.DurationSeconds, in.SizeBytes,
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
			(id, book_id, position, title, file_path, start_offset_seconds, duration_seconds, size_bytes)
		VALUES (?1, ?2, ?3, ?4, ?5, 0, ?6, ?7)
		RETURNING id, book_id, position, title, file_path, 0, duration_seconds, size_bytes`

	row := s.db.QueryRowContext(ctx, q,
		uuid.New(), in.BookID, in.Position, in.Title, in.FilePath, in.DurationSeconds, in.SizeBytes,
	)
	return scanChapter(row)
}

// GetChaptersByBookID vrátí všechny kapitoly knihy seřazené podle pořadí.
// start_offset_seconds se počítá dynamicky jako součet delék předešlých kapitol.
func (s *Store) GetChaptersByBookID(ctx context.Context, bookID uuid.UUID) ([]model.Chapter, error) {
	return getChaptersByBookID(ctx, s.db, bookID)
}

// GetChapterByID vrátí jednu kapitolu i s cestou k audio souboru.
// Offset v rámci knihy se počítá stejně jako v seznamu kapitol, jen pro
// jediný řádek – poddotaz sečte délky kapitol před ní.
func (s *Store) GetChapterByID(ctx context.Context, id uuid.UUID) (*model.Chapter, error) {
	const q = `
		SELECT
			c.id, c.book_id, c.position, c.title, c.file_path,
			(SELECT COALESCE(SUM(p.duration_seconds), 0) FROM chapters p
			 WHERE p.book_id = c.book_id AND p.position < c.position) AS start_offset_seconds,
			c.duration_seconds, c.size_bytes
		FROM chapters c
		WHERE c.id = ?1`

	return scanChapter(s.db.QueryRowContext(ctx, q, id))
}

func getChaptersByBookID(ctx context.Context, q querier, bookID uuid.UUID) ([]model.Chapter, error) {
	const query = `
		SELECT
			id, book_id, position, title, file_path,
			COALESCE(
				SUM(duration_seconds) OVER (
					PARTITION BY book_id
					ORDER BY position
					ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING
				), 0
			) AS start_offset_seconds,
			duration_seconds, size_bytes
		FROM chapters
		WHERE book_id = ?1
		ORDER BY position`

	rows, err := q.QueryContext(ctx, query, bookID)
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

// ReorderChapters nastaví kapitolám knihy pořadí podle seznamu ID a přečísluje
// je hustě 1..N. Seznam musí obsahovat všechny kapitoly knihy, každou právě
// jednou – jinak vrátí ErrChapterSetMismatch a nic nezmění.
//
// Nové pořadí se zároveň uloží do chapter_order_overrides podle cesty
// k souboru, aby přežilo opravu kapitol (ta řádky maže a scanner je načítá
// znovu). Po přečíslování na 1..N se nově přidané soubory zařadí na konec:
// jejich pozice z tagů nebo názvu už bývá obsazená a NextFreeChapterPosition
// je posune za poslední kapitolu.
func (s *Store) ReorderChapters(ctx context.Context, bookID uuid.UUID, ids []uuid.UUID) ([]model.Chapter, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reorder chapters: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := bookExists(ctx, tx, bookID); err != nil {
		return nil, err
	}

	existing, err := chapterIDs(ctx, tx, bookID)
	if err != nil {
		return nil, err
	}
	if len(ids) != len(existing) {
		return nil, ErrChapterSetMismatch
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !existing[id] || seen[id] {
			return nil, ErrChapterSetMismatch
		}
		seen[id] = true
	}

	// UNIQUE (book_id, position) SQLite kontroluje po jednotlivých řádcích,
	// takže přímé přepsání by cestou kolidovalo. Pozice se nejdřív překlopí
	// do záporných čísel (tam nic není) a pak se přidělí 1..N.
	if _, err := tx.ExecContext(ctx,
		`UPDATE chapters SET position = -position WHERE book_id = ?1`, bookID); err != nil {
		return nil, fmt.Errorf("reorder chapters: uvolnění pozic: %w", err)
	}
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx,
			`UPDATE chapters SET position = ?1 WHERE id = ?2 AND book_id = ?3`,
			i+1, id, bookID); err != nil {
			return nil, fmt.Errorf("reorder chapters: pozice %d: %w", i+1, err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM chapter_order_overrides WHERE book_id = ?1`, bookID); err != nil {
		return nil, fmt.Errorf("reorder chapters: smazání ručního pořadí: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO chapter_order_overrides (book_id, file_path, position)
		SELECT book_id, file_path, position FROM chapters WHERE book_id = ?1`, bookID); err != nil {
		return nil, fmt.Errorf("reorder chapters: uložení ručního pořadí: %w", err)
	}

	// Pořadí kapitol je součástí knihy, jen leží v jiné tabulce. Mobilní
	// aplikace se ptá, jestli má znovu stáhnout kapitoly, právě podle
	// books.updated_at – bez tohohle dotyku by jí zůstalo staré pořadí.
	if _, err := tx.ExecContext(ctx,
		`UPDATE books SET updated_at = CURRENT_TIMESTAMP WHERE id = ?1`, bookID); err != nil {
		return nil, fmt.Errorf("reorder chapters: dotyk knihy: %w", err)
	}

	chapters, err := getChaptersByBookID(ctx, tx, bookID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reorder chapters: commit: %w", err)
	}
	return chapters, nil
}

// chapterIDs vrátí množinu ID kapitol knihy.
func chapterIDs(ctx context.Context, q querier, bookID uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := q.QueryContext(ctx, `SELECT id FROM chapters WHERE book_id = ?1`, bookID)
	if err != nil {
		return nil, fmt.Errorf("chapter ids: %w", err)
	}
	defer rows.Close()

	ids := make(map[uuid.UUID]bool)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("chapter ids: %w", err)
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// ChapterOrderOverride vrátí ručně nastavenou pozici souboru v knize, pokud
// ji editor někdy určil. ok=false znamená, že pořadí určují tagy a název.
func (s *Store) ChapterOrderOverride(ctx context.Context, bookID uuid.UUID, filePath string) (int, bool, error) {
	var position int
	err := s.db.QueryRowContext(ctx,
		`SELECT position FROM chapter_order_overrides WHERE book_id = ?1 AND file_path = ?2`,
		bookID, filePath).Scan(&position)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("chapter order override: %w", err)
	}
	return position, true, nil
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
// Ruční pořadí zůstává v chapter_order_overrides a při novém načtení se obnoví.
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
		&c.FilePath, &c.StartOffsetSeconds, &c.DurationSeconds, &c.SizeBytes,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}
