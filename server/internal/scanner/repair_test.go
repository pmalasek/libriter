package scanner

import (
	"context"
	"errors"
	"fmt"
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

	result, err := ApplyRepair(ctx, store, "", plan, false)
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
	result, err := s.Repair(ctx, false)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	if result.DeletedChapters != 3 {
		t.Errorf("smazáno kapitol = %d, chtěny 3", result.DeletedChapters)
	}
	waitForIdle(t, s)

	assertOrder(t, chapterFiles(t, store, book.ID), []string{"03.mp3", "01.mp3", "02.mp3", "04.mp3"})
}

// --- kontrola chybějících souborů (DB → disk) ---

// Kniha, které na disku nezbyl ani jeden soubor, se smaže celá. Tohle je ten
// případ, kdy uživatel knihu smazal z disku a v knihovně zůstal prázdný záznam.
func TestPlanRepairFindsOrphanBook(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	// Druhá kniha drží knihovnu „živou“, aby nesepnula pojistka.
	keep := createRepairBook(t, store, "Bílá nemoc", "capek/nemoc", "Bílá nemoc")
	for i, name := range []string{"01.mp3", "02.mp3", "03.mp3", "04.mp3"} {
		writeAudio(t, audioRoot, "capek/nemoc/"+name)
		addChapter(t, store, keep.ID, i+1, "capek/nemoc/"+name)
	}

	gone := createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	addChapter(t, store, gone.ID, 1, "smolik/osamely/01.mp3")
	// Soubory se na disk vůbec nezapisují – kniha je od začátku bez nich.

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(plan.Orphans) != 1 || plan.Orphans[0].ID != gone.ID {
		t.Fatalf("osiřelé = %+v, chtěna kniha %s", plan.Orphans, gone.ID)
	}
	if plan.Orphans[0].Total != 1 || len(plan.Orphans[0].Chapters) != 1 {
		t.Errorf("osiřelá kniha = %+v, chtěna 1 kapitola z 1", plan.Orphans[0])
	}
	if len(plan.Missing) != 0 || len(plan.Rescan) != 0 {
		t.Errorf("chybějící = %+v, k načtení = %+v; chtěno prázdno", plan.Missing, plan.Rescan)
	}
	if plan.Guard.Tripped {
		t.Errorf("pojistka sepnula: %s", plan.Guard.Reason)
	}

	result, err := ApplyRepair(ctx, store, "", plan, false)
	if err != nil {
		t.Fatalf("ApplyRepair: %v", err)
	}
	if result.DeletedOrphans != 1 || result.DeletedBooks != 1 {
		t.Errorf("smazáno osiřelých = %d, knih = %d; chtěno 1 a 1",
			result.DeletedOrphans, result.DeletedBooks)
	}
	if _, err := store.GetBook(ctx, gone.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("kniha po opravě = %v, chtěno ErrNotFound", err)
	}
	if _, err := store.GetBook(ctx, keep.ID); err != nil {
		t.Errorf("zdravá kniha zmizela: %v", err)
	}
}

// Knize, které chybí jen část souborů, se smažou jen ty kapitoly a délka se
// srovná se zbytkem. Kniha samotná zůstane.
func TestPlanRepairFindsPartiallyMissingChapters(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	for i, name := range []string{"01.mp3", "02.mp3", "03.mp3", "04.mp3"} {
		addChapter(t, store, book.ID, i+1, "capek/hmyz/"+name)
	}
	// Na disk jen tři ze čtyř.
	for _, name := range []string{"01.mp3", "02.mp3", "03.mp3"} {
		writeAudio(t, audioRoot, "capek/hmyz/"+name)
	}

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(plan.Missing) != 1 || plan.Missing[0].ID != book.ID {
		t.Fatalf("chybějící = %+v, chtěna kniha %s", plan.Missing, book.ID)
	}
	if got := plan.Missing[0]; got.Total != 4 || len(got.Chapters) != 1 ||
		got.Chapters[0].FilePath != "capek/hmyz/04.mp3" {
		t.Fatalf("chybějící kniha = %+v, chtěna 1 kapitola (04.mp3) ze 4", got)
	}
	if len(plan.Orphans) != 0 {
		t.Errorf("osiřelé = %+v, chtěno prázdno", plan.Orphans)
	}

	if _, err := ApplyRepair(ctx, store, "", plan, false); err != nil {
		t.Fatalf("ApplyRepair: %v", err)
	}

	count, duration, err := store.GetBookChapterStats(ctx, book.ID)
	if err != nil {
		t.Fatalf("GetBookChapterStats: %v", err)
	}
	if count != 3 {
		t.Errorf("kapitol po opravě = %d, chtěny 3", count)
	}
	after, err := store.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if after.DurationSeconds != duration {
		t.Errorf("délka knihy = %d, chtěno %d", after.DurationSeconds, duration)
	}
}

// Kniha bez jediné kapitoly (pád mezi založením knihy a vložením první
// kapitoly) osiřelá není – jinak by ji oprava mlčky smazala.
func TestPlanRepairKeepsBookWithoutChapters(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	empty := createRepairBook(t, store, "Krakatit", "capek/krakatit", "Krakatit")

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(plan.Orphans) != 0 || len(plan.Missing) != 0 {
		t.Fatalf("osiřelé = %+v, chybějící = %+v; chtěno prázdno", plan.Orphans, plan.Missing)
	}
	if _, err := store.GetBook(ctx, empty.ID); err != nil {
		t.Errorf("prázdná kniha zmizela: %v", err)
	}
}

// Kniha, která má na disku soubor navíc, se celá načte znovu – přenačtení
// spraví i chybějící kapitoly, takže je zbytečné mazat je zvlášť.
func TestPlanRepairPrefersRescanOverMissing(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	addChapter(t, store, book.ID, 1, "capek/hmyz/01.mp3")
	addChapter(t, store, book.ID, 2, "capek/hmyz/02.mp3")
	writeAudio(t, audioRoot, "capek/hmyz/01.mp3")
	writeAudio(t, audioRoot, "capek/hmyz/03.mp3") // na disku navíc

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if len(plan.Rescan) != 1 || plan.Rescan[0].ID != book.ID {
		t.Fatalf("k načtení znovu = %+v, chtěna kniha %s", plan.Rescan, book.ID)
	}
	if len(plan.Missing) != 0 || len(plan.Orphans) != 0 {
		t.Errorf("chybějící = %+v, osiřelé = %+v; chtěno prázdno", plan.Missing, plan.Orphans)
	}
}

// Kniha, jejíž soubory na disku jsou, v plánu nefiguruje (regrese).
func TestPlanRepairIgnoresHealthyBook(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	for i, name := range []string{"01.mp3", "02.mp3"} {
		writeAudio(t, audioRoot, "capek/hmyz/"+name)
		addChapter(t, store, book.ID, i+1, "capek/hmyz/"+name)
	}

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if !plan.IsEmpty() {
		t.Errorf("plán = %+v, chtěno prázdno", plan)
	}
}

// Nedostupný AUDIO_ROOT nesmí vypadat jako prázdná knihovna – plán se vůbec
// nesestaví. Tohle je nejdůležitější pojistka celé kontroly.
func TestPlanRepairFailsOnMissingAudioRoot(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	addChapter(t, store, book.ID, 1, "capek/hmyz/01.mp3")

	if _, err := PlanRepair(ctx, store, filepath.Join(audioRoot, "neexistuje")); !errors.Is(err, ErrAudioRootUnavailable) {
		t.Fatalf("PlanRepair nad neexistujícím rootem = %v, chtěno ErrAudioRootUnavailable", err)
	}

	// Existující, ale prázdný adresář je typicky nepřipojený disk.
	if _, err := PlanRepair(ctx, store, audioRoot); !errors.Is(err, ErrAudioRootUnavailable) {
		t.Fatalf("PlanRepair nad prázdným rootem = %v, chtěno ErrAudioRootUnavailable", err)
	}
	if _, err := store.GetBook(ctx, book.ID); err != nil {
		t.Errorf("kniha zmizela, ačkoli se nic neopravovalo: %v", err)
	}
}

// Když chybí velká část knihovny, pojistka sepne: plán se vrátí i s výpisem,
// ale ApplyRepair chybějící soubory přeskočí. Teprve force je smaže.
func TestApplyRepairGuardSkipsMissingUntilForced(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	// Osm kapitol, z toho šest chybí → 75 %, tedy nad prahem.
	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("capek/hmyz/%02d.mp3", i)
		addChapter(t, store, book.ID, i, name)
		if i <= 2 {
			writeAudio(t, audioRoot, name)
		}
	}

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if !plan.Guard.Tripped {
		t.Fatalf("pojistka nesepnula: %+v", plan.Guard)
	}
	if plan.Guard.Reason == "" {
		t.Error("pojistka nemá důvod")
	}
	if len(plan.Missing) != 1 || len(plan.Missing[0].Chapters) != 6 {
		t.Fatalf("chybějící = %+v, chtěno 6 kapitol jedné knihy", plan.Missing)
	}

	result, err := ApplyRepair(ctx, store, "", plan, false)
	if err != nil {
		t.Fatalf("ApplyRepair: %v", err)
	}
	if !result.Skipped || result.DeletedMissingChapters != 0 {
		t.Errorf("výsledek = %+v, chtěno přeskočeno bez mazání", result)
	}
	if count, _, _ := store.GetBookChapterStats(ctx, book.ID); count != 8 {
		t.Errorf("kapitol po přeskočené opravě = %d, chtěno 8", count)
	}

	forced, err := ApplyRepair(ctx, store, "", plan, true)
	if err != nil {
		t.Fatalf("ApplyRepair(force): %v", err)
	}
	if forced.DeletedMissingChapters != 6 {
		t.Errorf("smazáno kapitol = %d, chtěno 6", forced.DeletedMissingChapters)
	}
	if count, _, _ := store.GetBookChapterStats(ctx, book.ID); count != 2 {
		t.Errorf("kapitol po vynucené opravě = %d, chtěny 2", count)
	}
}

// Ruční pořadí kapitol je klíčované cestou k souboru a mazání chybějících
// kapitol ho nesmí zahodit – když se soubory vrátí, pořadí se obnoví.
func TestApplyRepairKeepsOrderOverrides(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	book := createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze života hmyzu")
	for i, name := range []string{"01.mp3", "02.mp3", "03.mp3", "04.mp3"} {
		addChapter(t, store, book.ID, i+1, "capek/hmyz/"+name)
	}
	for _, name := range []string{"01.mp3", "02.mp3", "03.mp3"} {
		writeAudio(t, audioRoot, "capek/hmyz/"+name)
	}

	chapters, err := store.GetChaptersByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	ids := []uuid.UUID{chapters[3].ID, chapters[0].ID, chapters[1].ID, chapters[2].ID}
	if _, err := store.ReorderChapters(ctx, book.ID, ids); err != nil {
		t.Fatalf("ReorderChapters: %v", err)
	}

	plan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if _, err := ApplyRepair(ctx, store, "", plan, false); err != nil {
		t.Fatalf("ApplyRepair: %v", err)
	}

	// Ruční pozice chybějícího souboru zůstává v chapter_order_overrides.
	if _, ok, err := store.ChapterOrderOverride(ctx, book.ID, "capek/hmyz/04.mp3"); err != nil {
		t.Fatalf("ChapterOrderOverride: %v", err)
	} else if !ok {
		t.Error("ruční pořadí smazané kapitoly zmizelo")
	}
}
