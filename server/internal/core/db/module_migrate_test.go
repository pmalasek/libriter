package db

import (
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"mediater/internal/core/config"
)

func TestMigrateModule(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, config.DBConfig{Path: filepath.Join(t.TempDir(), "m.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	var baseBefore int
	if err := conn.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&baseBefore); err != nil {
		t.Fatal(err)
	}

	x := fstest.MapFS{
		"001_init.sql": {Data: []byte(`CREATE TABLE x_items (id TEXT PRIMARY KEY, user_id TEXT REFERENCES users (id) ON DELETE CASCADE);`)},
		"002_more.sql": {Data: []byte(`ALTER TABLE x_items ADD COLUMN title TEXT;`)},
		"README.md":    {Data: []byte(`ignorováno`)},
	}
	y := fstest.MapFS{
		"001_init.sql": {Data: []byte(`CREATE TABLE y_items (id TEXT PRIMARY KEY);`)},
	}

	for range 2 { // podruhé se nesmí nic aplikovat znovu
		if err := MigrateModule(ctx, conn, "x", x); err != nil {
			t.Fatalf("MigrateModule x: %v", err)
		}
		if err := MigrateModule(ctx, conn, "y", y); err != nil {
			t.Fatalf("MigrateModule y: %v", err)
		}
	}
	if err := MigrateModule(ctx, conn, "z", nil); err != nil {
		t.Fatalf("MigrateModule bez migrací: %v", err)
	}

	rows, err := conn.Query(`SELECT module, version FROM module_migrations ORDER BY module, version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var m string
		var v int
		if err := rows.Scan(&m, &v); err != nil {
			t.Fatal(err)
		}
		got = append(got, m+"/"+string(rune('0'+v)))
	}
	want := []string{"x/1", "x/2", "y/1"}
	if len(got) != len(want) {
		t.Fatalf("module_migrations = %v, chci %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("module_migrations = %v, chci %v", got, want)
		}
	}

	if _, err := conn.Exec(`INSERT INTO x_items (id, title) VALUES ('a', 'b')`); err != nil {
		t.Fatalf("tabulka modulu: %v", err)
	}

	var baseAfter int
	if err := conn.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&baseAfter); err != nil {
		t.Fatal(err)
	}
	if baseAfter != baseBefore {
		t.Fatalf("schema_migrations se změnila: %d → %d", baseBefore, baseAfter)
	}
}

func TestMigrateModuleFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, config.DBConfig{Path: filepath.Join(t.TempDir(), "m.db")})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	bad := fstest.MapFS{
		"001_ok.sql":  {Data: []byte(`CREATE TABLE ok_items (id TEXT);`)},
		"002_bad.sql": {Data: []byte(`CREATE TABLE broken (;`)},
	}
	if err := MigrateModule(ctx, conn, "bad", bad); err == nil {
		t.Fatal("chybná migrace prošla")
	}
	var n int
	if err := conn.QueryRow(`SELECT count(*) FROM module_migrations WHERE module = 'bad'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("zaznamenáno %d migrací, chci 1 (002 se musí vrátit)", n)
	}
}
