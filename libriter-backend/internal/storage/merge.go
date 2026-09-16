package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrSameBook vrací MergeBooks, když je cíl uvedený i mezi zdroji.
var ErrSameBook = errors.New("cílová kniha je zároveň zdrojem")

// MergeBooks sloučí zdrojové knihy do cílové: přesune kapitoly, autory, štítky
// i uživatelská data (poslech, hodnocení, záložky), doplní cíli prázdná pole ze
// zdroje, přepočítá délku podle kapitol a zdroje smaže. Vrací počet přesunutých
// kapitol.
//
// Všechno běží v jedné transakci – při chybě zůstane knihovna beze změny.
// Kapitoly se přesouvají, ne mažou: na rozdíl od opravy kapitol se kniha
// nenačítá znovu, takže scanner nemá šanci ji podle album tagu zase rozdělit.
// Ruční pořadí kapitol zdrojů (chapter_order_overrides) odejde kaskádou s jejich
// smazáním; sloučené kapitoly ho získají zpět až dalším ručním seřazením cíle.
func (s *Store) MergeBooks(ctx context.Context, targetID uuid.UUID, sourceIDs []uuid.UUID) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("merge books: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := bookExists(ctx, tx, targetID); err != nil {
		return 0, err
	}

	moved := 0
	for _, sourceID := range sourceIDs {
		if sourceID == targetID {
			return 0, ErrSameBook
		}
		n, err := mergeOneBook(ctx, tx, targetID, sourceID)
		if err != nil {
			return 0, err
		}
		moved += n
	}

	// Délka knihy je součet kapitol. COALESCE drží CHECK (duration_seconds > 0)
	// i u knihy, která zatím žádnou kapitolu nemá.
	const qDuration = `
		UPDATE books SET
			duration_seconds = COALESCE(
				(SELECT SUM(duration_seconds) FROM chapters WHERE book_id = ?1),
				duration_seconds),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?1`
	if _, err := tx.ExecContext(ctx, qDuration, targetID); err != nil {
		return 0, fmt.Errorf("merge books: přepočet délky: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("merge books: commit: %w", err)
	}
	return moved, nil
}

func bookExists(ctx context.Context, q querier, id uuid.UUID) error {
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM books WHERE id = ?1`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("kniha %s: %w", id, err)
	}
	return nil
}

// mergeOneBook přelije jednu zdrojovou knihu do cílové a smaže ji.
func mergeOneBook(ctx context.Context, tx *sql.Tx, targetID, sourceID uuid.UUID) (int, error) {
	offset, err := chapterOffset(ctx, tx, targetID, sourceID)
	if err != nil {
		return 0, err
	}

	// Pozice se posouvají jen při kolizi (UNIQUE (book_id, position)); běžný
	// rozdělený případ má pozice navazující (1–23 a 24–63), a ty zůstanou.
	const qChapters = `UPDATE chapters SET book_id = ?1, position = position + ?3 WHERE book_id = ?2`
	res, err := tx.ExecContext(ctx, qChapters, targetID, sourceID, offset)
	if err != nil {
		return 0, fmt.Errorf("merge books: přesun kapitol: %w", err)
	}
	moved := int(rowsAffected(res))

	// Autoři zdroje se připojí za autory cíle; společné autory drží stranou
	// primární klíč (book_id, author_id).
	const qAuthors = `
		INSERT OR IGNORE INTO book_authors (book_id, author_id, position)
		SELECT ?1, author_id,
		       position + (SELECT COALESCE(MAX(position), 0) FROM book_authors WHERE book_id = ?1)
		FROM book_authors WHERE book_id = ?2 ORDER BY position`
	if _, err := tx.ExecContext(ctx, qAuthors, targetID, sourceID); err != nil {
		return 0, fmt.Errorf("merge books: přesun autorů: %w", err)
	}

	// Uživatelská data: co se ke knize váže jen jednou na uživatele, se při
	// kolizi zahodí (u cíle už záznam je) – proto UPDATE OR IGNORE. Zbytek
	// smaže kaskáda při mazání zdroje.
	updates := []string{
		`UPDATE OR IGNORE book_tags          SET book_id = ?1 WHERE book_id = ?2`,
		`UPDATE OR IGNORE ratings            SET book_id = ?1 WHERE book_id = ?2`,
		`UPDATE OR IGNORE playback_positions SET book_id = ?1 WHERE book_id = ?2`,
		`UPDATE           bookmarks          SET book_id = ?1 WHERE book_id = ?2`,
		`UPDATE           listening_sessions SET book_id = ?1 WHERE book_id = ?2`,
		// Poslechové session: kniha se v seznamu nahradí cílovou. Když už tam
		// cílová je, zůstane její vlastní pozice a zdrojová položka odejde
		// kaskádou. source_id není cizí klíč, přepíše se ručně.
		`UPDATE OR IGNORE play_session_items SET book_id = ?1 WHERE book_id = ?2`,
		`UPDATE play_sessions SET current_book_id = ?1 WHERE current_book_id = ?2`,
		`UPDATE play_sessions SET source_id = ?1 WHERE source_id = ?2 AND kind = 'book'`,
	}
	for _, q := range updates {
		if _, err := tx.ExecContext(ctx, q, targetID, sourceID); err != nil {
			return 0, fmt.Errorf("merge books: přesun uživatelských dat: %w", err)
		}
	}

	// Prázdná pole cíle se doplní ze zdroje – metadata mohla být naimportovaná
	// jen u jedné z rozdělených knih. Série se přebírá jako dvojice, jinak by
	// padl CHECK books_series_position_check.
	const qFill = `
		UPDATE books SET
			narrator        = COALESCE(NULLIF(books.narrator, ''), src.narrator),
			description     = COALESCE(NULLIF(books.description, ''), src.description),
			cover_path      = COALESCE(NULLIF(books.cover_path, ''), src.cover_path),
			published_year  = COALESCE(books.published_year, src.published_year),
			internal_rating = COALESCE(books.internal_rating, src.internal_rating),
			series_id       = CASE WHEN books.series_id IS NULL THEN src.series_id ELSE books.series_id END,
			series_position = CASE WHEN books.series_id IS NULL THEN src.series_position ELSE books.series_position END
		FROM (SELECT * FROM books WHERE id = ?2) AS src
		WHERE books.id = ?1`
	if _, err := tx.ExecContext(ctx, qFill, targetID, sourceID); err != nil {
		return 0, fmt.Errorf("merge books: doplnění metadat: %w", err)
	}

	res, err = tx.ExecContext(ctx, `DELETE FROM books WHERE id = ?1`, sourceID)
	if err != nil {
		return 0, fmt.Errorf("merge books: smazání zdroje: %w", err)
	}
	if rowsAffected(res) == 0 {
		return 0, ErrNotFound
	}
	return moved, nil
}

// chapterOffset vrátí posun pozic kapitol zdroje. Bez kolize s pozicemi cíle je
// nulový, takže si kniha po sloučení zachová původní číslování.
func chapterOffset(ctx context.Context, tx *sql.Tx, targetID, sourceID uuid.UUID) (int, error) {
	const qCollision = `
		SELECT EXISTS(
			SELECT 1 FROM chapters s
			JOIN chapters t ON t.book_id = ?1 AND t.position = s.position
			WHERE s.book_id = ?2)`

	var collides bool
	if err := tx.QueryRowContext(ctx, qCollision, targetID, sourceID).Scan(&collides); err != nil {
		return 0, fmt.Errorf("merge books: kontrola pozic: %w", err)
	}
	if !collides {
		return 0, nil
	}

	var maxPosition int
	const qMax = `SELECT COALESCE(MAX(position), 0) FROM chapters WHERE book_id = ?1`
	if err := tx.QueryRowContext(ctx, qMax, targetID).Scan(&maxPosition); err != nil {
		return 0, fmt.Errorf("merge books: nejvyšší pozice: %w", err)
	}
	return maxPosition, nil
}
