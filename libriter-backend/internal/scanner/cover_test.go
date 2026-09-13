package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"libriter/internal/model"
)

func TestLargestImageInDir(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, n int) {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("thumb.jpg", 100)
	write("cover.png", 5000)
	write("notes.txt", 90000)
	write("empty.jpg", 0)

	got := largestImageInDir(dir)
	if filepath.Base(got) != "cover.png" {
		t.Fatalf("chtěl cover.png, dostal %q", got)
	}

	if got := largestImageInDir(t.TempDir()); got != "" {
		t.Fatalf("prázdný adresář má vrátit \"\", dostal %q", got)
	}
}

func TestHasCover(t *testing.T) {
	root := t.TempDir()
	s := &Scanner{coverRoot: root}

	if s.hasCover(&model.Book{}) {
		t.Error("kniha bez cover_path nemá mít obálku")
	}

	missing := "chybi.jpg"
	if s.hasCover(&model.Book{CoverPath: &missing}) {
		t.Error("cover_path na neexistující soubor se má brát jako chybějící")
	}

	name := "ok.jpg"
	if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.hasCover(&model.Book{CoverPath: &name}) {
		t.Error("existující obálka nebyla rozpoznána")
	}
}
