package storage

import (
	"context"
	"fmt"

	"libriter/internal/model"
)

// ListLanguages vrátí celý číselník jazyků seřazený podle českého názvu.
func (s *Store) ListLanguages(ctx context.Context) ([]model.Language, error) {
	const q = `SELECT code, name_cs, name_native FROM languages ORDER BY name_cs`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list languages: %w", err)
	}
	defer rows.Close()

	var list []model.Language
	for rows.Next() {
		var l model.Language
		if err := rows.Scan(&l.Code, &l.NameCs, &l.NameNative); err != nil {
			return nil, fmt.Errorf("list languages: %w", err)
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// LanguageExists ověří, že kód jazyka je v číselníku.
func (s *Store) LanguageExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM languages WHERE code = ?1)`, code,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("language exists: %w", err)
	}
	return exists, nil
}
