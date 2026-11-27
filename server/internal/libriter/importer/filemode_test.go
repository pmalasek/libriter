//go:build unix

package importer

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Kontejner běží s UMASK=0000; nahrané soubory se pak přesouvají do sdílené
// knihovny a nesmí být zapisovatelné pro všechny.
func TestUploadedFilesNotWorldWritable(t *testing.T) {
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })

	svc := newTestService(t, 1<<20, 0)
	sess, _ := svc.Create()
	if _, err := svc.AddFile(sess.ID, "Kniha/01.mp3", strings.NewReader("abc"), 3); err != nil {
		t.Fatal(err)
	}
	assertMode(t, filepath.Join(svc.filesDir(sess.ID), "Kniha", "01.mp3"), 0o644)

	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "kniha.zip"), map[string]string{"Kniha/02.mp3": "b"})
	if _, err := extractZip(dir, "kniha.zip", 1<<20); err != nil {
		t.Fatal(err)
	}
	assertMode(t, filepath.Join(dir, "Kniha", "02.mp3"), 0o644)
}

func assertMode(t *testing.T, p string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != want {
		t.Errorf("%s: práva %o, chci %o", filepath.Base(p), got, want)
	}
}
