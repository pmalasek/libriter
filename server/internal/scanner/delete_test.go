package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"libriter/internal/storage"

	"github.com/google/uuid"
)

// Bez delete_files zmizí jen záznam; soubory na disku zůstanou a scanner
// knihu při dalším průchodu založí znovu.
func TestDeleteBookKeepsFilesByDefault(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	writeAudio(t, audioRoot, "smolik/osamely/01.mp3")
	addChapter(t, store, book.ID, 1, "smolik/osamely/01.mp3")

	s := New(audioRoot, "", store)
	result, err := s.DeleteBook(ctx, book.ID, false)
	if err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
	if result.DeletedFiles != 0 {
		t.Errorf("smazáno souborů = %d, chtěno 0", result.DeletedFiles)
	}
	assertExists(t, filepath.Join(audioRoot, "smolik/osamely/01.mp3"), true)
}

// S delete_files zmizí i soubory a prázdný adresář knihy.
func TestDeleteBookRemovesFilesAndEmptyDirs(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	for i, name := range []string{"CD1/01.mp3", "CD1/02.mp3", "CD2/01.mp3"} {
		writeAudio(t, audioRoot, "smolik/osamely/"+name)
		addChapter(t, store, book.ID, i+1, "smolik/osamely/"+name)
	}

	s := New(audioRoot, "", store)
	result, err := s.DeleteBook(ctx, book.ID, true)
	if err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
	if result.DeletedFiles != 3 {
		t.Errorf("smazáno souborů = %d, chtěny 3", result.DeletedFiles)
	}
	if result.FreedBytes <= 0 {
		t.Errorf("uvolněno bajtů = %d, chtěno víc než 0", result.FreedBytes)
	}

	assertExists(t, filepath.Join(audioRoot, "smolik/osamely"), false)
	// Adresář autora zůstává – nad adresář knihy se nesahá.
	assertExists(t, filepath.Join(audioRoot, "smolik"), true)
	assertExists(t, audioRoot, true)
}

// Dvě knihy v jednom adresáři (série): smazání jedné nesmí vzít soubory
// druhé ani společný adresář.
func TestDeleteBookKeepsSiblingBookInSameDir(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	first := createRepairBook(t, store, "Povídky z jedné kapsy", "capek/povidky", "Z jedné kapsy")
	writeAudio(t, audioRoot, "capek/povidky/kapsa1.mp3")
	addChapter(t, store, first.ID, 1, "capek/povidky/kapsa1.mp3")

	second := createRepairBook(t, store, "Povídky z druhé kapsy", "capek/povidky", "Z druhé kapsy")
	writeAudio(t, audioRoot, "capek/povidky/kapsa2.mp3")
	addChapter(t, store, second.ID, 1, "capek/povidky/kapsa2.mp3")

	s := New(audioRoot, "", store)
	if _, err := s.DeleteBook(ctx, first.ID, true); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}

	assertExists(t, filepath.Join(audioRoot, "capek/povidky/kapsa1.mp3"), false)
	assertExists(t, filepath.Join(audioRoot, "capek/povidky/kapsa2.mp3"), true)
	assertExists(t, filepath.Join(audioRoot, "capek/povidky"), true)

	if _, err := store.GetBook(ctx, second.ID); err != nil {
		t.Errorf("sousední kniha zmizela: %v", err)
	}
}

// Cesta mimo AUDIO_ROOT se nesmí smazat, ať se do databáze dostala jakkoli.
func TestDeleteBookRefusesPathOutsideAudioRoot(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	outside := filepath.Join(t.TempDir(), "cizi.mp3")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	book := createRepairBook(t, store, "Podvržená", "smolik/podvrzena", "Podvržená")
	// Cesta s `..` projde do DB jen ručně; audiostore.Resolve ji musí odmítnout.
	addChapter(t, store, book.ID, 1, "../"+filepath.Base(filepath.Dir(outside))+"/cizi.mp3")

	s := New(audioRoot, "", store)
	result, err := s.DeleteBook(ctx, book.ID, true)
	if err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
	if result.DeletedFiles != 0 {
		t.Errorf("smazáno souborů = %d, chtěno 0", result.DeletedFiles)
	}
	assertExists(t, outside, true)
}

// Chybějící soubor mazání neshodí – právě tahle kombinace nastane, když
// uživatel knihu nejdřív smaže z disku a teprve pak z rozhraní.
func TestDeleteBookToleratesAlreadyMissingFiles(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	addChapter(t, store, book.ID, 1, "smolik/osamely/01.mp3")

	s := New(audioRoot, "", store)
	result, err := s.DeleteBook(ctx, book.ID, true)
	if err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
	if result.DeletedFiles != 0 || result.Chapters != 1 {
		t.Errorf("výsledek = %+v, chtěno 0 smazaných souborů z 1 kapitoly", result)
	}
	if _, err := store.GetBook(ctx, book.ID); err == nil {
		t.Error("kniha zůstala v databázi")
	}
}

// Obálka knihy zmizí spolu s ní, ať se maže s soubory, nebo bez nich.
func TestDeleteBookRemovesCover(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)
	coverRoot := t.TempDir()

	book := createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	cover := book.ID.String() + ".jpg"
	if err := os.WriteFile(filepath.Join(coverRoot, cover), []byte("img"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := store.PatchBook(ctx, book.ID, func(in *storage.BookInput) {
		in.CoverPath = &cover
	}); err != nil {
		t.Fatalf("PatchBook: %v", err)
	}

	s := New(audioRoot, coverRoot, store)
	if _, err := s.DeleteBook(ctx, book.ID, false); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
	assertExists(t, filepath.Join(coverRoot, cover), false)
}

// Neexistující kniha vrátí ErrNotFound, ne prázdný úspěch.
func TestDeleteBookUnknownID(t *testing.T) {
	store, audioRoot := newRepairEnv(t)

	s := New(audioRoot, "", store)
	if _, err := s.DeleteBook(context.Background(), uuid.New(), false); err == nil {
		t.Fatal("mazání neexistující knihy prošlo bez chyby")
	}
}

func assertExists(t *testing.T, path string, want bool) {
	t.Helper()

	_, err := os.Stat(path)
	if got := err == nil; got != want {
		if want {
			t.Errorf("%s neexistuje, ale měl by", path)
		} else {
			t.Errorf("%s pořád existuje, ale neměl by", path)
		}
	}
}
