package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"libriter/internal/model"

	"github.com/google/uuid"
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

func TestNormalizeImageExt(t *testing.T) {
	cases := map[string]string{
		".JPEG": ".jpeg", "png": ".png", ".xyz": ".jpg", "": ".jpg", ".webp": ".webp",
	}
	for in, want := range cases {
		if got := normalizeImageExt(in); got != want {
			t.Errorf("normalizeImageExt(%q) = %q, chtěl %q", in, got, want)
		}
	}
}

func TestWriteCoverAndHasCover(t *testing.T) {
	root := filepath.Join(t.TempDir(), "covers") // ještě neexistuje – musí se vytvořit
	s := &Scanner{coverRoot: root}

	src := filepath.Join(t.TempDir(), "folder.JPG")
	if err := os.WriteFile(src, []byte("obrazek"), 0o644); err != nil {
		t.Fatal(err)
	}

	id := uuid.New()
	rel, err := s.copyCoverFile(id, src)
	if err != nil {
		t.Fatal(err)
	}
	if rel != id.String()+".jpg" {
		t.Fatalf("cesta %q", rel)
	}
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil || string(data) != "obrazek" {
		t.Fatalf("obsah %q err %v", data, err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatalf("zůstal .tmp soubor: %v", entries)
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
