package storage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"

	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	conn, err := db.Open(context.Background(), config.DBConfig{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return New(conn)
}

func TestBookWithMultipleAuthors(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{
		{First: "Karel", Last: "Čapek"},
		{First: "Josef", Last: "Čapek"},
	})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}
	if len(authors) != 2 {
		t.Fatalf("autorů: %d, chtěni 2", len(authors))
	}
	if authors[0].Name != "Karel Čapek" {
		t.Errorf("celé jméno = %q, chtěno %q", authors[0].Name, "Karel Čapek")
	}

	book, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID, authors[1].ID},
		Title:           "Ze života hmyzu",
		DurationSeconds: 7200,
		FilePath:        "capek/ze-zivota-hmyzu",
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	if len(book.Authors) != 2 {
		t.Fatalf("kniha má %d autorů, chtěni 2", len(book.Authors))
	}
	// Pořadí autorů odpovídá pořadí na vstupu.
	if book.Authors[0].Name != "Karel Čapek" || book.Authors[1].Name != "Josef Čapek" {
		t.Errorf("pořadí autorů: %q, %q", book.Authors[0].Name, book.Authors[1].Name)
	}

	// Čtení knihy i seznamu vrací autory.
	got, err := store.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if len(got.Authors) != 2 {
		t.Errorf("GetBook: %d autorů, chtěni 2", len(got.Authors))
	}

	books, err := store.ListBooks(ctx)
	if err != nil {
		t.Fatalf("ListBooks: %v", err)
	}
	if len(books) != 1 || len(books[0].Authors) != 2 {
		t.Errorf("ListBooks: %d knih, %d autorů", len(books), len(books[0].Authors))
	}

	// Kniha je dohledatelná přes kteréhokoli ze svých autorů.
	for _, a := range authors {
		if _, err := store.GetBookByTitleAndAuthorID(ctx, book.Title, a.ID); err != nil {
			t.Errorf("GetBookByTitleAndAuthorID(%s): %v", a.Name, err)
		}
	}

	// Úprava knihy přepíše seznam autorů.
	updated, err := store.UpdateBook(ctx, book.ID, BookInput{
		AuthorIDs:       []uuid.UUID{authors[1].ID},
		Title:           book.Title,
		DurationSeconds: book.DurationSeconds,
		FilePath:        book.FilePath,
		Language:        book.Language,
	})
	if err != nil {
		t.Fatalf("UpdateBook: %v", err)
	}
	if len(updated.Authors) != 1 || updated.Authors[0].Name != "Josef Čapek" {
		t.Errorf("po úpravě: %v", updated.Authors)
	}
}

func TestGetOrCreateAuthorIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	name := model.AuthorName{First: "Jan", Middle: "Amos", Last: "Komenský"}
	first, err := store.GetOrCreateAuthor(ctx, name)
	if err != nil {
		t.Fatalf("GetOrCreateAuthor: %v", err)
	}
	second, err := store.GetOrCreateAuthor(ctx, name)
	if err != nil {
		t.Fatalf("GetOrCreateAuthor podruhé: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("vznikli dva autoři: %s a %s", first.ID, second.ID)
	}

	// Stejné jméno přes CreateAuthor musí narazit na unikátní index.
	if _, err := store.CreateAuthor(ctx, AuthorInput{Name: name}); !errors.Is(err, ErrConflict) {
		t.Errorf("CreateAuthor duplicitního autora: %v, chtěno ErrConflict", err)
	}

	// Autor bez knih jde smazat.
	if err := store.DeleteAuthor(ctx, first.ID); err != nil {
		t.Errorf("DeleteAuthor: %v", err)
	}
}

func TestPatchBookKeepsUntouchedFields(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{
		{First: "Karel", Last: "Čapek"},
		{First: "Josef", Last: "Čapek"},
	})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}

	narrator := "Viktor Preiss"
	cover := "obalka.jpg"
	book, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID, authors[1].ID},
		Title:           "Ze života hmyzu",
		Narrator:        &narrator,
		DurationSeconds: 7200,
		FilePath:        "capek/ze-zivota-hmyzu",
		CoverPath:       &cover,
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}

	// Změna jediného pole nesmí sáhnout na nic dalšího – hlavně ne na file_path,
	// který se přes API vůbec nevystavuje.
	patched, err := store.PatchBook(ctx, book.ID, func(in *BookInput) {
		in.Title = "Ze života hmyzu (rozhlasová hra)"
	})
	if err != nil {
		t.Fatalf("PatchBook: %v", err)
	}
	if patched.Title != "Ze života hmyzu (rozhlasová hra)" {
		t.Errorf("title = %q", patched.Title)
	}
	if patched.FilePath != book.FilePath {
		t.Errorf("file_path = %q, chtěno %q", patched.FilePath, book.FilePath)
	}
	if patched.DurationSeconds != 7200 {
		t.Errorf("duration_seconds = %d, chtěno 7200", patched.DurationSeconds)
	}
	if patched.Narrator == nil || *patched.Narrator != narrator {
		t.Errorf("narrator = %v, chtěno %q", patched.Narrator, narrator)
	}
	if patched.CoverPath == nil || *patched.CoverPath != cover {
		t.Errorf("cover_path = %v, chtěno %q", patched.CoverPath, cover)
	}
	if len(patched.Authors) != 2 || patched.Authors[0].Name != "Karel Čapek" {
		t.Errorf("autoři po patchi: %v", patched.Authors)
	}

	// Patch, který autory nastaví, je přepíše i s pořadím.
	patched, err = store.PatchBook(ctx, book.ID, func(in *BookInput) {
		in.AuthorIDs = []uuid.UUID{authors[1].ID, authors[0].ID}
	})
	if err != nil {
		t.Fatalf("PatchBook s autory: %v", err)
	}
	if len(patched.Authors) != 2 || patched.Authors[0].Name != "Josef Čapek" {
		t.Errorf("autoři po přeuspořádání: %v", patched.Authors)
	}

	// Vyprázdnění nullable pole projde.
	patched, err = store.PatchBook(ctx, book.ID, func(in *BookInput) {
		in.Narrator = nil
	})
	if err != nil {
		t.Fatalf("PatchBook s prázdným vypravěčem: %v", err)
	}
	if patched.Narrator != nil {
		t.Errorf("narrator = %v, chtěno nil", *patched.Narrator)
	}

	// Rok vydání se ukládá i čte; bez zásahu zůstává prázdný.
	if patched.PublishedYear != nil {
		t.Errorf("published_year = %v, chtěno nil", *patched.PublishedYear)
	}
	year := 1936
	patched, err = store.PatchBook(ctx, book.ID, func(in *BookInput) {
		in.PublishedYear = &year
	})
	if err != nil {
		t.Fatalf("PatchBook s rokem vydání: %v", err)
	}
	if patched.PublishedYear == nil || *patched.PublishedYear != year {
		t.Errorf("published_year = %v, chtěno %d", patched.PublishedYear, year)
	}
	if got, err := store.GetBook(ctx, book.ID); err != nil || got.PublishedYear == nil || *got.PublishedYear != year {
		t.Errorf("GetBook published_year = %v (err %v), chtěno %d", got, err, year)
	}

	// Neexistující kniha končí ErrNotFound a apply se nevolá.
	called := false
	if _, err := store.PatchBook(ctx, uuid.New(), func(*BookInput) { called = true }); !errors.Is(err, ErrNotFound) {
		t.Errorf("PatchBook neexistující knihy: %v, chtěno ErrNotFound", err)
	}
	if called {
		t.Error("apply se zavolalo i pro neexistující knihu")
	}
}

// Počet kapitol je odvozený sloupec – musí sedět ve všech cestách, kterými
// kniha z databáze vychází, včetně RETURNING po zápisu.
func TestBookChapterCount(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}
	albumTag := "Válka s mloky"

	book, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           "Válka s mloky",
		DurationSeconds: 3600,
		FilePath:        "capek/valka-s-mloky",
		AlbumTag:        &albumTag,
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	if book.ChapterCount != 0 {
		t.Errorf("nová kniha má %d kapitol, chtěno 0", book.ChapterCount)
	}

	for i := 1; i <= 2; i++ {
		if _, err := store.UpsertChapter(ctx, ChapterInput{
			BookID:          book.ID,
			Position:        i,
			Title:           fmt.Sprintf("Kapitola %d", i),
			FilePath:        fmt.Sprintf("capek/valka-s-mloky/%02d.mp3", i),
			DurationSeconds: 1800,
		}); err != nil {
			t.Fatalf("UpsertChapter: %v", err)
		}
	}

	got, err := store.GetBook(ctx, book.ID)
	if err != nil || got.ChapterCount != 2 {
		t.Errorf("GetBook: %d kapitol (err %v), chtěno 2", got.ChapterCount, err)
	}

	books, err := store.ListBooks(ctx)
	if err != nil || len(books) != 1 || books[0].ChapterCount != 2 {
		t.Errorf("ListBooks: %+v (err %v), chtěny 2 kapitoly", books, err)
	}

	// RETURNING po UPDATE musí počet vrátit taky – frontend ukládá odpověď
	// PATCH rovnou do cache, jinak by v ní kniha měla nula kapitol.
	patched, err := store.PatchBook(ctx, book.ID, func(in *BookInput) { in.Title = "Válka s mloky (2. vydání)" })
	if err != nil || patched.ChapterCount != 2 {
		t.Errorf("PatchBook: %d kapitol (err %v), chtěno 2", patched.ChapterCount, err)
	}

	byTag, err := store.GetBooksByAlbumTag(ctx, albumTag)
	if err != nil || len(byTag) != 1 || byTag[0].ChapterCount != 2 {
		t.Errorf("GetBooksByAlbumTag: %+v (err %v), chtěny 2 kapitoly", byTag, err)
	}
}
