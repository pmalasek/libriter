package storage

import (
	"context"
	"errors"
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
