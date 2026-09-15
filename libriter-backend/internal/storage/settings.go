package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// GetSetting vrátí hodnotu nastavení jako surový JSON. Chybějící klíč je
// ErrNotFound – volající si za něj dosadí výchozí hodnotu z konfigurace.
func (s *Store) GetSetting(ctx context.Context, key string) (json.RawMessage, error) {
	const q = `SELECT value FROM settings WHERE key = ?1`

	var value string
	err := s.db.QueryRowContext(ctx, q, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get setting %s: %w", key, err)
	}
	return json.RawMessage(value), nil
}

// SetSetting uloží hodnotu nastavení (vloží nebo přepíše).
func (s *Store) SetSetting(ctx context.Context, key string, value json.RawMessage) error {
	const q = `
		INSERT INTO settings (key, value)
		VALUES (?1, ?2)
		ON CONFLICT (key) DO UPDATE
		SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`

	if _, err := s.db.ExecContext(ctx, q, key, string(value)); err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}
