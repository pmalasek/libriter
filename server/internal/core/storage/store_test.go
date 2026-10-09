package storage

import (
	"context"
	"path/filepath"
	"testing"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	conn, err := db.Open(context.Background(), config.DBConfig{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return New(conn)
}
