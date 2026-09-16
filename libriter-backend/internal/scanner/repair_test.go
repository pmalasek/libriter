package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// newRepairEnv připraví prázdnou databázi a adresář pro audio soubory.
func newRepairEnv(t *testing.T) (*storage.Store, string) {
	t.Helper()

	conn, err := db.Open(context.Background(), config.DBConfig{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return storage.New(conn), t.TempDir()
}

// writeAudio vytvoří prázdný audio soubor – plán kapitoly čte jen z DB
// a názvů souborů, do obsahu nesahá.
func writeAudio(t *testing.T, audioRoot, rel string) {
	t.Helper()

	abs := filepath.Join(audioRoot, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(abs, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func createRepairBook(t *testing.T, store *storage.Store, title, dir, albumTag string) *model.Book {
	t.Helper()
	ctx := context.Background()

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}

	book, err := store.CreateBook(ctx, storage.BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           title,
		DurationSeconds: 3600,
		FilePath:        dir,
		AlbumTag:        &albumTag,
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	return book
}

func addChapter(t *testing.T, store *storage.Store, bookID uuid.UUID, position int, filePath string) {
	t.Helper()

	if _, err := store.UpsertChapter(context.Background(), storage.ChapterInput{
		BookID:          bookID,
		Position:        position,
		Title:           filepath.Base(filePath),
		FilePath:        filePath,
		DurationSeconds: 60,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}
}

// Soubor, který leží na disku, ale v DB kapitolu nemá, znamená, že se kniha
// musí načíst znovu.
func TestPlanRepairFindsMissingChapter(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	writeAudio(t, audioRoot, "capek/hmyz/01.mp3")
	writeAudio(t, audioRoot, "capek/hmyz/02.mp3")
	addChapter(t, store, book.ID, 1, "capek/hmyz/01.mp3")

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(plan.Rescan) != 1 || plan.Rescan[0].ID != book.ID {
		t.Fatalf("k načtení znovu = %+v, chtěna kniha %s", plan.Rescan, book.ID)
	}
	if plan.Rescan[0].FilePath != "capek/hmyz" {
		t.Errorf("file_path = %q, chtěno capek/hmyz", plan.Rescan[0].FilePath)
	}
	if len(plan.Duplicates) != 0 {
		t.Errorf("duplikáty = %+v, chtěno prázdno", plan.Duplicates)
	}
}

// Kapitola z adresáře, který s adresářem knihy nesouvisí, znamená dvě vydání
// slepená do jedné knihy.
func TestPlanRepairFindsForeignDirectory(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	writeAudio(t, audioRoot, "capek/hmyz/01.mp3")
	writeAudio(t, audioRoot, "jine-vydani/01.mp3")
	addChapter(t, store, book.ID, 1, "capek/hmyz/01.mp3")
	addChapter(t, store, book.ID, 2, "jine-vydani/01.mp3")

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(plan.Rescan) != 1 || plan.Rescan[0].ID != book.ID {
		t.Errorf("k načtení znovu = %+v, chtěna kniha %s", plan.Rescan, book.ID)
	}
}

// Dvě knihy se stejným album tagem v jednom adresáři: starší zůstává, novější
// je duplikát ke smazání.
func TestPlanAndApplyRepairRemovesDuplicate(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	older := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	// created_at má vteřinovou přesnost, druhá kniha proto musí vzniknout
	// v jiné vteřině, jinak není pořadí jednoznačné.
	time.Sleep(1100 * time.Millisecond)
	newer := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")

	writeAudio(t, audioRoot, "capek/hmyz/01.mp3")
	writeAudio(t, audioRoot, "capek/hmyz/02.mp3")
	addChapter(t, store, older.ID, 1, "capek/hmyz/01.mp3")
	addChapter(t, store, newer.ID, 1, "capek/hmyz/02.mp3")

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(plan.Duplicates) != 1 || plan.Duplicates[0].ID != newer.ID {
		t.Fatalf("duplikáty = %+v, chtěna novější kniha %s", plan.Duplicates, newer.ID)
	}
	if len(plan.Rescan) != 1 || plan.Rescan[0].ID != older.ID {
		t.Fatalf("k načtení znovu = %+v, chtěna starší kniha %s", plan.Rescan, older.ID)
	}

	result, err := ApplyRepair(ctx, store, plan)
	if err != nil {
		t.Fatalf("ApplyRepair: %v", err)
	}
	if result.DeletedChapters != 2 || result.DeletedBooks != 1 {
		t.Errorf("smazáno kapitol = %d, knih = %d; chtěno 2 a 1",
			result.DeletedChapters, result.DeletedBooks)
	}

	if _, err := store.GetBook(ctx, newer.ID); err == nil {
		t.Error("duplikát v databázi zůstal")
	}
	if _, err := store.GetBook(ctx, older.ID); err != nil {
		t.Errorf("starší kniha zmizela: %v", err)
	}

	chapters, err := store.GetChaptersByBookID(ctx, older.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if len(chapters) != 0 {
		t.Errorf("kapitol po opravě = %d, chtěno 0 (scanner je načte znovu)", len(chapters))
	}
}

// Když kapitoly souborům odpovídají, plán je prázdný.
func TestPlanRepairEmptyWhenConsistent(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	writeAudio(t, audioRoot, "capek/hmyz/01.mp3")
	addChapter(t, store, book.ID, 1, "capek/hmyz/01.mp3")

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if !plan.IsEmpty() {
		t.Errorf("plán = %+v, chtěno prázdno", plan)
	}
}

// chapterFiles vrátí názvy souborů kapitol knihy v pořadí přehrávání.
func chapterFiles(t *testing.T, store *storage.Store, bookID uuid.UUID) []string {
	t.Helper()

	chapters, err := store.GetChaptersByBookID(context.Background(), bookID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	names := make([]string, 0, len(chapters))
	for _, c := range chapters {
		names = append(names, filepath.Base(c.FilePath))
	}
	return names
}

func assertOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("kapitoly = %v, chtěno %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kapitoly = %v, chtěno %v", got, want)
		}
	}
}

// Soubory bez track tagu se řadí podle názvu tak, jak ho čte člověk – "2"
// před "10". Prázdné soubory tagy nemají, takže scanner sáhne po názvu.
func TestScanUsesNaturalOrderWithoutTags(t *testing.T) {
	store, audioRoot := newRepairEnv(t)
	for _, name := range []string{"1.mp3", "10.mp3", "2.mp3"} {
		writeAudio(t, audioRoot, "capek/hmyz/"+name)
	}

	s := New(audioRoot, "", store)
	if err := s.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	waitForIdle(t, s)

	book, err := store.GetBookByDirPath(context.Background(), "capek/hmyz")
	if err != nil {
		t.Fatalf("GetBookByDirPath: %v", err)
	}
	assertOrder(t, chapterFiles(t, store, book.ID), []string{"1.mp3", "2.mp3", "10.mp3"})
}

// Ruční pořadí má přednost před vším ostatním a přežije smazání kapitol.
func TestScanHonoursManualOrder(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "hmyz")
	files := []string{"capek/hmyz/01.mp3", "capek/hmyz/02.mp3", "capek/hmyz/03.mp3"}
	for i, f := range files {
		writeAudio(t, audioRoot, f)
		addChapter(t, store, book.ID, i+1, f)
	}
	chapters, err := store.GetChaptersByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if _, err := store.ReorderChapters(ctx, book.ID,
		[]uuid.UUID{chapters[2].ID, chapters[0].ID, chapters[1].ID}); err != nil {
		t.Fatalf("ReorderChapters: %v", err)
	}
	if _, err := store.DeleteChaptersByBookID(ctx, book.ID); err != nil {
		t.Fatalf("DeleteChaptersByBookID: %v", err)
	}

	s := New(audioRoot, "", store)
	if err := s.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	waitForIdle(t, s)

	assertOrder(t, chapterFiles(t, store, book.ID), []string{"03.mp3", "01.mp3", "02.mp3"})
}

// Oprava kapitol knihu načte znovu, ruční pořadí ale zůstane; nový soubor
// bez ručně určené pozice se zařadí na konec.
func TestRepairKeepsManualOrder(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "hmyz")
	files := []string{"capek/hmyz/01.mp3", "capek/hmyz/02.mp3", "capek/hmyz/03.mp3"}
	for i, f := range files {
		writeAudio(t, audioRoot, f)
		addChapter(t, store, book.ID, i+1, f)
	}
	chapters, err := store.GetChaptersByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if _, err := store.ReorderChapters(ctx, book.ID,
		[]uuid.UUID{chapters[2].ID, chapters[0].ID, chapters[1].ID}); err != nil {
		t.Fatalf("ReorderChapters: %v", err)
	}

	// Soubor na disku bez kapitoly → kniha je v plánu opravy.
	writeAudio(t, audioRoot, "capek/hmyz/04.mp3")

	s := New(audioRoot, "", store)
	result, err := s.Repair(ctx)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.DeletedChapters != 3 {
		t.Errorf("smazáno kapitol = %d, chtěny 3", result.DeletedChapters)
	}
	waitForIdle(t, s)

	assertOrder(t, chapterFiles(t, store, book.ID), []string{"03.mp3", "01.mp3", "02.mp3", "04.mp3"})
}
