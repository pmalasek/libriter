package storage

import (
	"context"
	"database/sql"
	"fmt"

	"libriter/internal/model"

	"github.com/google/uuid"
)

// touchBookProgress založí knihu jako rozposlouchanou, případně ji označí za
// doposlechnutou. Doposlechnutí se nikdy neruší – další poslech knihy z ní
// zase rozposlouchanou nedělá. Volá se uvnitř transakce zápisu pozice.
func touchBookProgress(ctx context.Context, q querier, userID, bookID uuid.UUID, finished bool) error {
	const query = `
		INSERT INTO book_progress (user_id, book_id, finished_at)
		VALUES (?1, ?2, CASE WHEN ?3 THEN CURRENT_TIMESTAMP END)
		ON CONFLICT (user_id, book_id) DO UPDATE SET
		  finished_at = CASE WHEN ?3 THEN COALESCE(finished_at, CURRENT_TIMESTAMP) ELSE finished_at END,
		  updated_at  = CURRENT_TIMESTAMP`

	if _, err := q.ExecContext(ctx, query, userID, bookID, finished); err != nil {
		if isForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("touch book progress: %w", err)
	}
	return nil
}

// ListBookProgress vrátí stav všech knih uživatele, od naposledy změněné.
func (s *Store) ListBookProgress(ctx context.Context, userID uuid.UUID) ([]model.BookProgress, error) {
	const q = `
		SELECT user_id, book_id, started_at, finished_at, updated_at
		FROM book_progress
		WHERE user_id = ?1
		ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list book progress: %w", err)
	}
	defer rows.Close()

	progress := []model.BookProgress{}
	for rows.Next() {
		var (
			p        model.BookProgress
			finished sql.NullTime
		)
		if err := rows.Scan(&p.UserID, &p.BookID, &p.StartedAt, &finished, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("list book progress: %w", err)
		}
		if finished.Valid {
			t := finished.Time
			p.FinishedAt = &t
		}
		progress = append(progress, p)
	}
	return progress, rows.Err()
}

// SetBookFinished označí knihu za doposlechnutou ručně – třeba proto, že ji
// uživatel slyšel jinde. Neexistující kniha vrací ErrNotFound.
func (s *Store) SetBookFinished(ctx context.Context, userID, bookID uuid.UUID) error {
	return touchBookProgress(ctx, s.db, userID, bookID, true)
}

// DeleteBookProgress vrátí knihu mezi neposlechnuté. Příští zápis pozice ji
// zase založí jako rozposlouchanou. Chybějící řádek není chyba – výsledek
// je stejný.
func (s *Store) DeleteBookProgress(ctx context.Context, userID, bookID uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM book_progress WHERE user_id = ?1 AND book_id = ?2`, userID, bookID); err != nil {
		return fmt.Errorf("delete book progress: %w", err)
	}
	return nil
}
