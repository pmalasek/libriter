package storage

import (
	"context"
	"errors"
	"testing"

	"libriter/internal/model"

	"github.com/google/uuid"
)

// bookForChapters vytvoří knihu, ke které se dají přidávat kapitoly.
func bookForChapters(t *testing.T, store *Store) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Daniel", Last: "Cole"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}
	book, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           "Loutkář",
		DurationSeconds: 60,
		FilePath:        "cole/loutkar",
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	return book.ID
}

// Dva disky jednoho vydání mají obě track 1; kapitola prvního disku se nesmí
// přepsat kapitolou druhého – právě to vedlo k opakovanému scanu při každém startu.
func TestUpsertChapterKeepsBothFilesOnPositionClash(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)

	first, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: bookID, Position: 1, Title: "Prolog",
		FilePath: "cole/loutkar/cd1/01.mp3", DurationSeconds: 100,
	})
	if err != nil {
		t.Fatalf("UpsertChapter (CD1): %v", err)
	}

	position, err := store.NextFreeChapterPosition(ctx, bookID, 1, "cole/loutkar/cd2/01.mp3")
	if err != nil {
		t.Fatalf("NextFreeChapterPosition: %v", err)
	}
	if position != 2 {
		t.Fatalf("pozice = %d, chtěna 2", position)
	}

	if _, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: bookID, Position: position, Title: "Kapitola 23",
		FilePath: "cole/loutkar/cd2/01.mp3", DurationSeconds: 200,
	}); err != nil {
		t.Fatalf("UpsertChapter (CD2): %v", err)
	}

	chapters, err := store.GetChaptersByBookID(ctx, bookID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("kapitol: %d, chtěny 2", len(chapters))
	}
	if chapters[0].ID != first.ID || chapters[0].FilePath != "cole/loutkar/cd1/01.mp3" {
		t.Errorf("první kapitola přepsána: %+v", chapters[0])
	}

	// Oba soubory jsou v DB, takže je scanner při dalším běhu přeskočí.
	for _, path := range []string{"cole/loutkar/cd1/01.mp3", "cole/loutkar/cd2/01.mp3"} {
		exists, err := store.ChapterExistsByFilePath(ctx, path)
		if err != nil {
			t.Fatalf("ChapterExistsByFilePath(%s): %v", path, err)
		}
		if !exists {
			t.Errorf("soubor %s v DB chybí", path)
		}
	}
}

// Opakovaný scan téhož souboru jen aktualizuje jeho řádek a pozici nemění.
func TestUpsertChapterSameFileIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)

	const path = "cole/loutkar/01.mp3"
	created, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: bookID, Position: 3, Title: "Prolog", FilePath: path, DurationSeconds: 100,
	})
	if err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	position, err := store.NextFreeChapterPosition(ctx, bookID, 3, path)
	if err != nil {
		t.Fatalf("NextFreeChapterPosition: %v", err)
	}
	if position != 3 {
		t.Fatalf("pozice = %d, chtěna 3 (vlastní řádek není kolize)", position)
	}

	updated, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: bookID, Position: position, Title: "Prolog", FilePath: path, DurationSeconds: 120,
	})
	if err != nil {
		t.Fatalf("UpsertChapter (znovu): %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("vznikl nový řádek: %s → %s", created.ID, updated.ID)
	}
	if updated.DurationSeconds != 120 {
		t.Errorf("délka = %d, chtěno 120", updated.DurationSeconds)
	}

	count, total, err := store.GetBookChapterStats(ctx, bookID)
	if err != nil {
		t.Fatalf("GetBookChapterStats: %v", err)
	}
	if count != 1 || total != 120 {
		t.Errorf("kapitol = %d, délka = %d; chtěno 1 a 120", count, total)
	}
}

// NextFreeChapterPosition hledá první mezeru od požadované pozice.
func TestNextFreeChapterPosition(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)

	for i, position := range []int{1, 2, 3, 5} {
		if _, err := store.UpsertChapter(ctx, ChapterInput{
			BookID: bookID, Position: position, Title: "K",
			FilePath: "cole/loutkar/" + string(rune('a'+i)) + ".mp3", DurationSeconds: 10,
		}); err != nil {
			t.Fatalf("UpsertChapter(%d): %v", position, err)
		}
	}

	tests := []struct {
		desired int
		want    int
	}{
		{desired: 1, want: 4},
		{desired: 4, want: 4},
		{desired: 5, want: 6},
		{desired: 9, want: 9},
		{desired: 0, want: 4}, // nula se bere jako začátek knihy
	}
	for _, tc := range tests {
		got, err := store.NextFreeChapterPosition(ctx, bookID, tc.desired, "cole/loutkar/novy.mp3")
		if err != nil {
			t.Fatalf("NextFreeChapterPosition(%d): %v", tc.desired, err)
		}
		if got != tc.want {
			t.Errorf("NextFreeChapterPosition(%d) = %d, chtěno %d", tc.desired, got, tc.want)
		}
	}
}

// Stejné album od stejného autora může být v knihovně vícekrát (jiné vydání).
func TestGetBooksByAlbumTagReturnsAllEditions(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Arthur Conan", Last: "Doyle"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}

	album := "Podpis čtyř"
	dirs := []string{"doyle/podpis-ctyr", "doyle/sherlock-65x/podpis-ctyr"}
	for _, dir := range dirs {
		if _, err := store.CreateBook(ctx, BookInput{
			AuthorIDs:       []uuid.UUID{authors[0].ID},
			Title:           album,
			DurationSeconds: 60,
			FilePath:        dir,
			AlbumTag:        &album,
			Language:        "cs",
		}); err != nil {
			t.Fatalf("CreateBook(%s): %v", dir, err)
		}
	}

	books, err := store.GetBooksByAlbumTag(ctx, album)
	if err != nil {
		t.Fatalf("GetBooksByAlbumTag: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("knih: %d, chtěny 2", len(books))
	}
	for _, b := range books {
		if len(b.Authors) != 1 {
			t.Errorf("kniha %s má %d autorů, chtěn 1", b.Title, len(b.Authors))
		}
	}
}

// Kniha z dřívějších scanů nemá album tag; scanner ji v adresáři najde a tag
// jí doplní. Po přejmenování knihy (import metadat) se pak páruje dál.
func TestGetBookWithoutAlbumTagInDir(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Radek", Last: "John"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}
	newBook := func(title, dir string, albumTag *string) *model.Book {
		t.Helper()
		b, err := store.CreateBook(ctx, BookInput{
			AuthorIDs:       []uuid.UUID{authors[0].ID},
			Title:           title,
			DurationSeconds: 60,
			FilePath:        dir,
			AlbumTag:        albumTag,
			Language:        "cs",
		})
		if err != nil {
			t.Fatalf("CreateBook(%s): %v", title, err)
		}
		return b
	}

	legacy := newBook("Memento", "john/memento", nil)

	found, err := store.GetBookWithoutAlbumTagInDir(ctx, "john/memento")
	if err != nil {
		t.Fatalf("GetBookWithoutAlbumTagInDir: %v", err)
	}
	if found.ID != legacy.ID {
		t.Fatalf("nalezena kniha %s, chtěna %s", found.ID, legacy.ID)
	}

	if err := store.SetBookAlbumTag(ctx, legacy.ID, "Memento"); err != nil {
		t.Fatalf("SetBookAlbumTag: %v", err)
	}
	if _, err := store.GetBookWithoutAlbumTagInDir(ctx, "john/memento"); !errors.Is(err, ErrNotFound) {
		t.Errorf("kniha s tagem se stále nabízí k napojení: %v", err)
	}
	books, err := store.GetBooksByAlbumTag(ctx, "Memento")
	if err != nil || len(books) != 1 {
		t.Fatalf("GetBooksByAlbumTag = %d knih, %v", len(books), err)
	}

	// Import metadat knihu přejmenuje – album tag zůstává, párování drží.
	renamed, err := store.PatchBook(ctx, legacy.ID, func(in *BookInput) {
		in.Title = "Memento (1986)"
	})
	if err != nil {
		t.Fatalf("PatchBook: %v", err)
	}
	if renamed.AlbumTag == nil || *renamed.AlbumTag != "Memento" {
		t.Errorf("album tag po přejmenování = %v, chtěno Memento", renamed.AlbumTag)
	}

	// Dvě knihy bez tagu v jednom adresáři nelze rozlišit.
	dir := "lewis/letopisy-narnie"
	newBook("Čarodějův synovec", dir, nil)
	newBook("Lev, čarodějnice a skříň", dir, nil)
	if _, err := store.GetBookWithoutAlbumTagInDir(ctx, dir); !errors.Is(err, ErrNotFound) {
		t.Errorf("nejednoznačný adresář vrátil knihu: %v", err)
	}
}

// upsertChapters založí kapitoly v daném pořadí pozic a vrátí je v tom pořadí.
func upsertChapters(t *testing.T, store *Store, bookID uuid.UUID, positions []int) []model.Chapter {
	t.Helper()
	ctx := context.Background()

	chapters := make([]model.Chapter, 0, len(positions))
	for i, position := range positions {
		c, err := store.UpsertChapter(ctx, ChapterInput{
			BookID: bookID, Position: position, Title: "K",
			FilePath: "cole/loutkar/" + string(rune('a'+i)) + ".mp3", DurationSeconds: 10 * (i + 1),
		})
		if err != nil {
			t.Fatalf("UpsertChapter(%d): %v", position, err)
		}
		chapters = append(chapters, *c)
	}
	return chapters
}

// Ruční pořadí přečísluje kapitoly hustě od 1 i z řídkých pozic (multi-disk
// vydání) a přepočítá offsety.
func TestReorderChaptersRenumbersDensely(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)
	c := upsertChapters(t, store, bookID, []int{1, 1001, 1002})

	got, err := store.ReorderChapters(ctx, bookID, []uuid.UUID{c[2].ID, c[0].ID, c[1].ID})
	if err != nil {
		t.Fatalf("ReorderChapters: %v", err)
	}

	wantIDs := []uuid.UUID{c[2].ID, c[0].ID, c[1].ID}
	wantOffsets := []int{0, 30, 40}
	if len(got) != 3 {
		t.Fatalf("kapitol = %d, chtěny 3", len(got))
	}
	for i := range got {
		if got[i].ID != wantIDs[i] || got[i].Position != i+1 {
			t.Errorf("kapitola %d = %s na pozici %d, chtěna %s na %d",
				i, got[i].ID, got[i].Position, wantIDs[i], i+1)
		}
		if got[i].StartOffsetSeconds != wantOffsets[i] {
			t.Errorf("offset kapitoly %d = %d, chtěno %d", i, got[i].StartOffsetSeconds, wantOffsets[i])
		}
	}

	stored, err := store.GetChaptersByBookID(ctx, bookID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	for i := range stored {
		if stored[i].ID != wantIDs[i] {
			t.Errorf("uložené pořadí %d = %s, chtěno %s", i, stored[i].ID, wantIDs[i])
		}
	}
}

// Seznam musí přesně odpovídat kapitolám knihy; jinak zůstane vše beze změny.
func TestReorderChaptersRejectsWrongSet(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)
	c := upsertChapters(t, store, bookID, []int{1, 2, 3})

	otherBook := bookForChaptersAt(t, store, "cole/jina")
	other, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: otherBook, Position: 1, Title: "K", FilePath: "cole/jina/a.mp3", DurationSeconds: 5,
	})
	if err != nil {
		t.Fatalf("UpsertChapter (jiná kniha): %v", err)
	}

	tests := map[string][]uuid.UUID{
		"chybějící":  {c[0].ID, c[1].ID},
		"duplicitní": {c[0].ID, c[1].ID, c[1].ID},
		"cizí":       {c[0].ID, c[1].ID, other.ID},
		"prázdný":    {},
	}
	for name, ids := range tests {
		if _, err := store.ReorderChapters(ctx, bookID, ids); !errors.Is(err, ErrChapterSetMismatch) {
			t.Errorf("%s seznam: err = %v, chtěno ErrChapterSetMismatch", name, err)
		}
	}

	stored, err := store.GetChaptersByBookID(ctx, bookID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	for i := range stored {
		if stored[i].ID != c[i].ID || stored[i].Position != i+1 {
			t.Errorf("pozice po odmítnutí změněna: %+v", stored[i])
		}
	}

	if _, err := store.ReorderChapters(ctx, uuid.New(), nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("neznámá kniha: err = %v, chtěno ErrNotFound", err)
	}
}

// Ruční pořadí se pamatuje podle cesty k souboru, další seřazení ho nahradí
// a se smazáním knihy zmizí.
func TestReorderChaptersWritesOverrides(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)
	c := upsertChapters(t, store, bookID, []int{1, 2})

	if _, err := store.ReorderChapters(ctx, bookID, []uuid.UUID{c[1].ID, c[0].ID}); err != nil {
		t.Fatalf("ReorderChapters: %v", err)
	}
	assertOverride := func(path string, want int) {
		t.Helper()
		got, ok, err := store.ChapterOrderOverride(ctx, bookID, path)
		if err != nil {
			t.Fatalf("ChapterOrderOverride(%s): %v", path, err)
		}
		if !ok || got != want {
			t.Errorf("override %s = %d (ok=%v), chtěno %d", path, got, ok, want)
		}
	}
	assertOverride(c[1].FilePath, 1)
	assertOverride(c[0].FilePath, 2)

	if _, err := store.ReorderChapters(ctx, bookID, []uuid.UUID{c[0].ID, c[1].ID}); err != nil {
		t.Fatalf("ReorderChapters (znovu): %v", err)
	}
	assertOverride(c[0].FilePath, 1)
	assertOverride(c[1].FilePath, 2)

	if _, ok, err := store.ChapterOrderOverride(ctx, bookID, "cole/loutkar/nikdy.mp3"); err != nil || ok {
		t.Errorf("neznámý soubor: ok=%v, err=%v; chtěno ok=false", ok, err)
	}

	if err := store.DeleteBook(ctx, bookID); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}
	if _, ok, err := store.ChapterOrderOverride(ctx, bookID, c[0].FilePath); err != nil || ok {
		t.Errorf("override po smazání knihy: ok=%v, err=%v; chtěno ok=false", ok, err)
	}
}

// bookForChaptersAt je bookForChapters s vlastním adresářem (druhá kniha v testu).
func bookForChaptersAt(t *testing.T, store *Store, dir string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Daniel", Last: "Cole"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}
	book, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           "Jiná",
		DurationSeconds: 60,
		FilePath:        dir,
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	return book.ID
}

// Velikost souboru přežije opakovaný scan i změnu délky – mobil podle ní
// odhaduje místo na disku a průběh stahování.
func TestUpsertChapterStoresSizeBytes(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)

	created, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: bookID, Position: 1, Title: "Prolog",
		FilePath: "cole/loutkar/01.mp3", DurationSeconds: 100, SizeBytes: 1_234_567,
	})
	if err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}
	if created.SizeBytes != 1_234_567 {
		t.Errorf("size_bytes po vložení = %d, chtěno 1234567", created.SizeBytes)
	}

	updated, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: bookID, Position: 1, Title: "Prolog",
		FilePath: "cole/loutkar/01.mp3", DurationSeconds: 100, SizeBytes: 999,
	})
	if err != nil {
		t.Fatalf("UpsertChapter (znovu): %v", err)
	}
	if updated.SizeBytes != 999 {
		t.Errorf("size_bytes po přepsání = %d, chtěno 999", updated.SizeBytes)
	}

	stored, err := store.GetChapterByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetChapterByID: %v", err)
	}
	if stored.SizeBytes != 999 {
		t.Errorf("size_bytes z databáze = %d, chtěno 999", stored.SizeBytes)
	}

	list, err := store.GetChaptersByBookID(ctx, bookID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if len(list) != 1 || list[0].SizeBytes != 999 {
		t.Errorf("size_bytes v seznamu = %+v, chtěno 999", list)
	}
}

// Změna pořadí kapitol je změna knihy: mobil se ptá books.updated_at, jestli
// má kapitoly stáhnout znovu.
func TestReorderChaptersTouchesBook(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	bookID := bookForChapters(t, store)
	c := upsertChapters(t, store, bookID, []int{1, 2, 3})

	// CURRENT_TIMESTAMP má vteřinovou přesnost; bez posunu zpět by se razítko
	// v testu nemuselo vůbec lišit.
	if _, err := store.db.ExecContext(ctx,
		`UPDATE books SET updated_at = datetime('now', '-1 hour') WHERE id = ?1`, bookID); err != nil {
		t.Fatalf("posun updated_at: %v", err)
	}
	before, err := store.GetBook(ctx, bookID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}

	if _, err := store.ReorderChapters(ctx, bookID, []uuid.UUID{c[2].ID, c[1].ID, c[0].ID}); err != nil {
		t.Fatalf("ReorderChapters: %v", err)
	}

	after, err := store.GetBook(ctx, bookID)
	if err != nil {
		t.Fatalf("GetBook po přeřazení: %v", err)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Errorf("updated_at = %v, chtěno novější než %v", after.UpdatedAt, before.UpdatedAt)
	}
}
