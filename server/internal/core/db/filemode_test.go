//go:build unix

package db

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"mediater/internal/core/config"
)

// withUmask nastaví umask procesu po dobu testu (kontejner běží s 0000).
// Umask je globální pro celý proces, proto testy neběží paralelně.
func withUmask(t *testing.T, mask int) {
	t.Helper()
	old := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(old) })
}

func fileMode(t *testing.T, p string) os.FileMode {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}

// openAndWrite otevře databázi a zapíše do ní, aby vznikly -wal a -shm.
// Spojení zůstane otevřené – jinak by SQLite -wal/-shm při zavření smazal.
func openAndWrite(t *testing.T, path string) {
	t.Helper()
	conn, err := Open(context.Background(), config.DBConfig{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS t_mode (x INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO t_mode VALUES (1)`); err != nil {
		t.Fatal(err)
	}
}

func TestOpenNewDatabaseIsPrivate(t *testing.T) {
	withUmask(t, 0)

	dir := filepath.Join(t.TempDir(), "data")
	path := filepath.Join(dir, "libriter.db")
	openAndWrite(t, path)

	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if got := fileMode(t, p); got != 0o600 {
			t.Errorf("%s: práva %o, chci 600", filepath.Base(p), got)
		}
	}
	if got := fileMode(t, dir); got != 0o700 {
		t.Errorf("nový adresář: práva %o, chci 700", got)
	}
}

func TestOpenTightensExistingDatabase(t *testing.T) {
	withUmask(t, 0)

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "libriter.db")

	// Databáze ze starší verze: soubor i -wal/-shm čitelné pro všechny.
	openAndWrite(t, path)
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o666); err != nil {
			t.Fatal(err)
		}
	}

	openAndWrite(t, path)

	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if got := fileMode(t, p); got != 0o600 {
			t.Errorf("%s: práva %o, chci 600", filepath.Base(p), got)
		}
	}
	// Existující adresář se nemění – může ho sdílet víc věcí (obálky, importy).
	if got := fileMode(t, dir); got != 0o755 {
		t.Errorf("existující adresář: práva %o, chci beze změny 755", got)
	}
}
