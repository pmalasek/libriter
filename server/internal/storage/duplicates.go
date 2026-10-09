package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

// DismissedDuplicate je jedna odmítnutá skupina („není to duplicita“).
type DismissedDuplicate struct {
	GroupKey string
	BookIDs  []uuid.UUID
}

// ListDismissedDuplicates vrátí odmítnuté skupiny i s knihami, které v nich
// byly. Neúplné klíče (část knih se mezitím smazala, zbytek odnesla kaskáda)
// se vracejí taky – že už nesedí na žádnou skupinu, pozná volající.
func (s *Store) ListDismissedDuplicates(ctx context.Context) ([]DismissedDuplicate, error) {
	const q = `SELECT group_key, book_id FROM duplicate_dismissals
		ORDER BY group_key, book_id`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list dismissed duplicates: %w", err)
	}
	defer rows.Close()

	byKey := map[string][]uuid.UUID{}
	order := []string{}
	for rows.Next() {
		var key string
		var bookID uuid.UUID
		if err := rows.Scan(&key, &bookID); err != nil {
			return nil, err
		}
		if _, seen := byKey[key]; !seen {
			order = append(order, key)
		}
		byKey[key] = append(byKey[key], bookID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]DismissedDuplicate, 0, len(order))
	for _, key := range order {
		out = append(out, DismissedDuplicate{GroupKey: key, BookIDs: byKey[key]})
	}
	return out, nil
}

// DismissDuplicate označí skupinu za „není to duplicita“. Opakované volání
// nic nerozbije – zapisuje se idempotentně.
func (s *Store) DismissDuplicate(
	ctx context.Context, groupKey string, bookIDs []uuid.UUID, actorID *uuid.UUID,
) error {
	if groupKey == "" || len(bookIDs) == 0 {
		return fmt.Errorf("prázdná skupina")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("dismiss duplicate: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // commit níž; rollback po něm nic nedělá

	const q = `INSERT INTO duplicate_dismissals (group_key, book_id, dismissed_by)
		VALUES (?1, ?2, ?3)
		ON CONFLICT (group_key, book_id) DO UPDATE SET
		  dismissed_by = excluded.dismissed_by,
		  dismissed_at = CURRENT_TIMESTAMP`

	for _, id := range bookIDs {
		if _, err := tx.ExecContext(ctx, q, groupKey, id, actorID); err != nil {
			return fmt.Errorf("dismiss duplicate: %w", err)
		}
	}
	return tx.Commit()
}

// RestoreDuplicate zruší odmítnutí, takže se skupina zase začne hlásit.
func (s *Store) RestoreDuplicate(ctx context.Context, groupKey string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM duplicate_dismissals WHERE group_key = ?1`, groupKey)
	if err != nil {
		return fmt.Errorf("restore duplicate: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

// DuplicateGroupKey sestaví klíč skupiny: ID knih seřazená a spojená čárkou.
// Musí být nezávislý na pořadí knih na vstupu, aby odmítnutí drželo i po
// přeskládání skupiny, a musí se změnit, jakmile do skupiny přibude další
// kniha – ta je nový nález, o kterém se rozhoduje zvlášť.
func DuplicateGroupKey(bookIDs []uuid.UUID) string {
	parts := make([]string, 0, len(bookIDs))
	for _, id := range bookIDs {
		parts = append(parts, id.String())
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
