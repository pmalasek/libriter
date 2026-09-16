package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"libriter/internal/model"

	"github.com/google/uuid"
)

// mergeTestBook založí knihu v daném adresáři; album tag odlišuje rozdělené
// knihy stejně jako v ostrém provozu.
func mergeTestBook(t *testing.T, store *Store, title, dir, albumTag string, authors ...model.AuthorName) *model.Book {
	t.Helper()
	ctx := context.Background()

	if len(authors) == 0 {
		authors = []model.AuthorName{{First: "Karel", Last: "Čapek"}}
	}
	created, err := store.GetOrCreateAuthors(ctx, authors)
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}
	ids := make([]uuid.UUID, 0, len(created))
	for _, a := range created {
		ids = append(ids, a.ID)
	}

	book, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       ids,
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

func mergeTestChapter(t *testing.T, store *Store, bookID uuid.UUID, position int, filePath string) {
	t.Helper()

	if _, err := store.UpsertChapter(context.Background(), ChapterInput{
		BookID:          bookID,
		Position:        position,
		Title:           fmt.Sprintf("Kapitola %d", position),
		FilePath:        filePath,
		DurationSeconds: 60,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}
}

func mergeTestUser(t *testing.T, store *Store, email string) *model.User {
	t.Helper()

	user, err := store.CreateUser(context.Background(), "Čtenář", email, "hash", "user")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user
}

// Navazující pozice (reálný případ rozdělené knihy) se sloučením nemění.
func TestMergeBooksMovesChaptersWithoutCollision(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	target := mergeTestBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "PÍSEÈNÁ BOUØE")
	source := mergeTestBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "Písečná bouře")
	mergeTestChapter(t, store, target.ID, 1, "rollins/pisecna-boure/01.mp3")
	mergeTestChapter(t, store, target.ID, 2, "rollins/pisecna-boure/02.mp3")
	mergeTestChapter(t, store, source.ID, 3, "rollins/pisecna-boure/03.mp3")

	moved, err := store.MergeBooks(ctx, target.ID, []uuid.UUID{source.ID})
	if err != nil {
		t.Fatalf("MergeBooks: %v", err)
	}
	if moved != 1 {
		t.Errorf("přesunuto kapitol = %d, chtěno 1", moved)
	}

	chapters, err := store.GetChaptersByBookID(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if len(chapters) != 3 {
		t.Fatalf("kapitol = %d, chtěny 3", len(chapters))
	}
	for i, c := range chapters {
		if c.Position != i+1 {
			t.Errorf("kapitola %d má pozici %d, chtěno %d", i, c.Position, i+1)
		}
	}

	got, err := store.GetBook(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if got.DurationSeconds != 180 {
		t.Errorf("délka = %d, chtěno 180 (součet kapitol)", got.DurationSeconds)
	}
	if got.ChapterCount != 3 {
		t.Errorf("počet kapitol = %d, chtěny 3", got.ChapterCount)
	}
	if _, err := store.GetBook(ctx, source.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("zdrojová kniha zůstala: %v", err)
	}
}

// Kolidující pozice se posunou za pozice cíle, pořadí kapitol zdroje zůstává.
func TestMergeBooksOffsetsCollidingPositions(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	target := mergeTestBook(t, store, "Hobit", "tolkien/hobit", "Hobit")
	source := mergeTestBook(t, store, "Hobit", "tolkien/hobit", "HOBIT")
	mergeTestChapter(t, store, target.ID, 1, "tolkien/hobit/a1.mp3")
	mergeTestChapter(t, store, target.ID, 2, "tolkien/hobit/a2.mp3")
	mergeTestChapter(t, store, source.ID, 1, "tolkien/hobit/b1.mp3")
	mergeTestChapter(t, store, source.ID, 2, "tolkien/hobit/b2.mp3")

	if _, err := store.MergeBooks(ctx, target.ID, []uuid.UUID{source.ID}); err != nil {
		t.Fatalf("MergeBooks: %v", err)
	}

	chapters, err := store.GetChaptersByBookID(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if len(chapters) != 4 {
		t.Fatalf("kapitol = %d, chtěny 4", len(chapters))
	}
	want := []string{"tolkien/hobit/a1.mp3", "tolkien/hobit/a2.mp3", "tolkien/hobit/b1.mp3", "tolkien/hobit/b2.mp3"}
	for i, c := range chapters {
		if c.Position != i+1 {
			t.Errorf("kapitola %d má pozici %d, chtěno %d", i, c.Position, i+1)
		}
		if c.FilePath != want[i] {
			t.Errorf("na pozici %d je %q, chtěno %q", i+1, c.FilePath, want[i])
		}
	}
}

// Uživatelská data se přenesou, prázdná pole cíle se doplní a autoři se spojí
// bez duplicit.
func TestMergeBooksCarriesUserDataAndFillsEmptyFields(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	target := mergeTestBook(t, store, "Bílá nemoc", "capek/bila-nemoc", "Bílá nemoc",
		model.AuthorName{First: "Karel", Last: "Čapek"})
	source := mergeTestBook(t, store, "Bílá nemoc", "capek/bila-nemoc", "BÍLÁ NEMOC",
		model.AuthorName{First: "Karel", Last: "Čapek"}, model.AuthorName{First: "Josef", Last: "Čapek"})

	// Metadata má jen zdroj – cíl je musí převzít.
	narrator, description, year := "Jan Novák", "Popis knihy", 1937
	if _, err := store.PatchBook(ctx, source.ID, func(in *BookInput) {
		in.Narrator = &narrator
		in.Description = &description
		in.PublishedYear = &year
	}); err != nil {
		t.Fatalf("PatchBook zdroje: %v", err)
	}

	// Poslech: uživatel A má rozečteno u obou knih (vyhrává cíl), uživatel B
	// jen u zdroje (musí se přesunout).
	userA := mergeTestUser(t, store, "a@example.com")
	userB := mergeTestUser(t, store, "b@example.com")
	insertPlayback(t, store, userA.ID, target.ID, 10)
	insertPlayback(t, store, userA.ID, source.ID, 20)
	insertPlayback(t, store, userB.ID, source.ID, 30)

	if _, err := store.MergeBooks(ctx, target.ID, []uuid.UUID{source.ID}); err != nil {
		t.Fatalf("MergeBooks: %v", err)
	}

	got, err := store.GetBook(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if got.Narrator == nil || *got.Narrator != narrator {
		t.Errorf("narrator = %v, chtěno %q", got.Narrator, narrator)
	}
	if got.Description == nil || *got.Description != description {
		t.Errorf("description = %v, chtěno %q", got.Description, description)
	}
	if got.PublishedYear == nil || *got.PublishedYear != year {
		t.Errorf("published_year = %v, chtěno %d", got.PublishedYear, year)
	}
	if len(got.Authors) != 2 {
		t.Errorf("autorů = %d, chtěni 2 (bez duplicit)", len(got.Authors))
	}

	if pos := playbackPosition(t, store, userA.ID, target.ID); pos != 10 {
		t.Errorf("poslech uživatele A = %d, chtěno 10 (záznam cíle zůstává)", pos)
	}
	if pos := playbackPosition(t, store, userB.ID, target.ID); pos != 30 {
		t.Errorf("poslech uživatele B = %d, chtěno 30 (přesunuto ze zdroje)", pos)
	}
}

// Cíl mezi zdroji ani neexistující kniha sloučení neprojdou a nic nezmění.
func TestMergeBooksRejectsSelfAndUnknown(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	target := mergeTestBook(t, store, "Krakatit", "capek/krakatit", "Krakatit")
	source := mergeTestBook(t, store, "Krakatit", "capek/krakatit", "KRAKATIT")
	mergeTestChapter(t, store, source.ID, 1, "capek/krakatit/01.mp3")

	if _, err := store.MergeBooks(ctx, target.ID, []uuid.UUID{target.ID}); !errors.Is(err, ErrSameBook) {
		t.Errorf("sloučení knihy se sebou: %v, chtěno ErrSameBook", err)
	}
	if _, err := store.MergeBooks(ctx, uuid.New(), []uuid.UUID{source.ID}); !errors.Is(err, ErrNotFound) {
		t.Errorf("neznámý cíl: %v, chtěno ErrNotFound", err)
	}
	if _, err := store.MergeBooks(ctx, target.ID, []uuid.UUID{uuid.New()}); !errors.Is(err, ErrNotFound) {
		t.Errorf("neznámý zdroj: %v, chtěno ErrNotFound", err)
	}

	// Transakce se vrátila zpět: zdroj i jeho kapitola jsou netknuté.
	if _, err := store.GetBook(ctx, source.ID); err != nil {
		t.Errorf("zdroj po neúspěšném sloučení zmizel: %v", err)
	}
	chapters, err := store.GetChaptersByBookID(ctx, source.ID)
	if err != nil || len(chapters) != 1 {
		t.Errorf("kapitoly zdroje = %d (err %v), chtěna 1", len(chapters), err)
	}
}

// playback_positions nemá API ve storage – testy si řádky vkládají přímo.
func insertPlayback(t *testing.T, store *Store, userID, bookID uuid.UUID, seconds int) {
	t.Helper()

	const q = `
		INSERT INTO playback_positions (id, user_id, book_id, position_seconds)
		VALUES (?1, ?2, ?3, ?4)`
	if _, err := store.db.ExecContext(context.Background(), q, uuid.New(), userID, bookID, seconds); err != nil {
		t.Fatalf("vložení poslechu: %v", err)
	}
}

func playbackPosition(t *testing.T, store *Store, userID, bookID uuid.UUID) int {
	t.Helper()

	const q = `SELECT position_seconds FROM playback_positions WHERE user_id = ?1 AND book_id = ?2`
	var seconds int
	if err := store.db.QueryRowContext(context.Background(), q, userID, bookID).Scan(&seconds); err != nil {
		t.Fatalf("čtení poslechu: %v", err)
	}
	return seconds
}

// Rozdělená kniha se v rozposlouchané session nahradí cílovou, aby poslech
// po opravě knihovny nespadl na neexistující knihu.
func TestMergeBooksKeepsPlaySessions(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	target := mergeTestBook(t, store, "Krakatit", "capek/krakatit", "Krakatit")
	source := mergeTestBook(t, store, "Krakatit", "capek/krakatit", "KRAKATIT")
	other := mergeTestBook(t, store, "Matka", "capek/matka", "Matka")
	user := mergeTestUser(t, store, "session@example.com")

	// Session, která zná jen zdrojovou knihu – ta se musí překlopit na cíl.
	moved, err := store.CreatePlaySession(ctx, PlaySessionInput{
		UserID: user.ID, Kind: model.PlaySessionBook,
		SourceID: &source.ID, BookIDs: []uuid.UUID{source.ID},
	})
	if err != nil {
		t.Fatalf("CreatePlaySession: %v", err)
	}
	if _, err := store.UpdatePlaySessionPosition(ctx, user.ID, moved.ID, PlaySessionPosition{
		BookID: source.ID, PositionSeconds: 42, PlaybackSpeed: 1.0,
	}); err != nil {
		t.Fatalf("UpdatePlaySessionPosition: %v", err)
	}

	// Seznam, který obsahuje obě knihy – duplicita nesmí porušit klíč a
	// vlastní pozice cílové knihy má zůstat.
	both, err := store.CreatePlaySession(ctx, PlaySessionInput{
		UserID: user.ID, Kind: model.PlaySessionList,
		BookIDs: []uuid.UUID{target.ID, source.ID, other.ID},
	})
	if err != nil {
		t.Fatalf("CreatePlaySession seznamu: %v", err)
	}
	if _, err := store.UpdatePlaySessionPosition(ctx, user.ID, both.ID, PlaySessionPosition{
		BookID: target.ID, PositionSeconds: 7, PlaybackSpeed: 1.0,
	}); err != nil {
		t.Fatalf("UpdatePlaySessionPosition seznamu: %v", err)
	}

	if _, err := store.MergeBooks(ctx, target.ID, []uuid.UUID{source.ID}); err != nil {
		t.Fatalf("MergeBooks: %v", err)
	}

	got, err := store.GetPlaySession(ctx, user.ID, moved.ID)
	if err != nil {
		t.Fatalf("GetPlaySession: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].BookID != target.ID || got.Items[0].PositionSeconds != 42 {
		t.Fatalf("přesunutá session: %+v", got.Items)
	}
	if got.CurrentBookID == nil || *got.CurrentBookID != target.ID {
		t.Errorf("aktuální kniha: %v", got.CurrentBookID)
	}
	if got.SourceID == nil || *got.SourceID != target.ID {
		t.Errorf("zdroj session: %v", got.SourceID)
	}

	list, err := store.GetPlaySession(ctx, user.ID, both.ID)
	if err != nil {
		t.Fatalf("GetPlaySession seznamu: %v", err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("seznam po sloučení: %+v", list.Items)
	}
	if list.Items[0].BookID != target.ID || list.Items[0].PositionSeconds != 7 {
		t.Errorf("pozice cílové knihy se ztratila: %+v", list.Items[0])
	}
	if list.Items[1].BookID != other.ID {
		t.Errorf("zbytek seznamu: %+v", list.Items[1])
	}
}
