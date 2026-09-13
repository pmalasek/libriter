package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// newCoverTestEnv připraví store, handler a adresář s obálkami.
func newCoverTestEnv(t *testing.T) (*storage.Store, http.Handler, string) {
	t.Helper()

	conn, err := db.Open(context.Background(), config.DBConfig{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := storage.New(conn)
	coverRoot := t.TempDir()
	h := NewBook(service.NewBook(store), coverRoot)

	r := chi.NewRouter()
	r.Get("/books/{id}/cover", h.Cover)
	r.Head("/books/{id}/cover", h.Cover)

	return store, r, coverRoot
}

// createTestBook založí knihu s jedním autorem.
func createTestBook(t *testing.T, store *storage.Store) *model.Book {
	t.Helper()
	ctx := context.Background()

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}

	book, err := store.CreateBook(ctx, storage.BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           "Ze života hmyzu",
		DurationSeconds: 7200,
		FilePath:        "capek/ze-zivota-hmyzu",
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	return book
}

func TestBookHandlerCoverErrors(t *testing.T) {
	ctx := context.Background()
	store, router, coverRoot := newCoverTestEnv(t)
	book := createTestBook(t, store)

	// Soubor nad COVER_ROOT - nesmí být dostupný přes ../
	secret := filepath.Join(filepath.Dir(coverRoot), "secret.txt")
	if err := os.WriteFile(secret, []byte("tajné"), 0o644); err != nil {
		t.Fatalf("zápis tajného souboru: %v", err)
	}

	tests := []struct {
		name      string
		id        string
		coverPath string // "" = cover_path se nenastavuje
		want      int
	}{
		{"neplatné UUID", "not-a-uuid", "", http.StatusBadRequest},
		{"neexistující kniha", uuid.NewString(), "", http.StatusNotFound},
		{"kniha bez obálky", book.ID.String(), "", http.StatusNotFound},
		{"chybějící soubor", book.ID.String(), "chybi.jpg", http.StatusNotFound},
		{"path traversal", book.ID.String(), "../secret.txt", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.coverPath != "" {
				if err := store.UpdateBookCoverPath(ctx, book.ID, tt.coverPath); err != nil {
					t.Fatalf("UpdateBookCoverPath: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodGet, "/books/"+tt.id+"/cover", nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("status = %d, chtěno %d (tělo: %s)", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestBookHandlerCoverServesFile(t *testing.T) {
	ctx := context.Background()
	store, router, coverRoot := newCoverTestEnv(t)
	book := createTestBook(t, store)

	// Minimální JPEG - stačí magické číslo, http.ServeFile určuje typ dle přípony.
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	name := book.ID.String() + ".jpg"
	if err := os.WriteFile(filepath.Join(coverRoot, name), data, 0o644); err != nil {
		t.Fatalf("zápis obálky: %v", err)
	}
	if err := store.UpdateBookCoverPath(ctx, book.ID, name); err != nil {
		t.Fatalf("UpdateBookCoverPath: %v", err)
	}

	t.Run("GET vrátí obálku", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/books/"+book.ID.String()+"/cover", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, chtěno 200 (tělo: %s)", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("Content-Type = %q, chtěno %q", ct, "image/jpeg")
		}
		if cc := rec.Header().Get("Cache-Control"); cc == "" {
			t.Error("chybí hlavička Cache-Control")
		}
		if rec.Header().Get("Last-Modified") == "" {
			t.Error("chybí hlavička Last-Modified")
		}
		if got := rec.Body.Bytes(); string(got) != string(data) {
			t.Errorf("tělo má %d B, chtěno %d B", len(got), len(data))
		}
	})

	t.Run("HEAD vrátí jen hlavičky", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/books/"+book.ID.String()+"/cover", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, chtěno 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
			t.Errorf("Content-Type = %q, chtěno %q", ct, "image/jpeg")
		}
	})
}

func TestBookPatchRequestValidation(t *testing.T) {
	// Základ, proti kterému se patch aplikuje – odpovídá řádku v databázi.
	narrator := "Viktor Preiss"
	description := "Původní popis."
	rating := int16(4)
	base := func() storage.BookInput {
		return storage.BookInput{
			AuthorIDs:       []uuid.UUID{uuid.MustParse("11111111-1111-1111-1111-111111111111")},
			Title:           "Ze života hmyzu",
			Narrator:        &narrator,
			DurationSeconds: 7200,
			FilePath:        "capek/ze-zivota-hmyzu",
			Language:        "cs",
			Description:     &description,
			InternalRating:  &rating,
		}
	}

	invalid := []struct {
		name string
		body string
	}{
		{"prázdný title", `{"title":"   "}`},
		{"nulová délka", `{"duration_seconds":0}`},
		{"záporná délka", `{"duration_seconds":-1}`},
		{"hodnocení mimo rozsah", `{"internal_rating":9}`},
		{"prázdní autoři", `{"author_ids":[]}`},
		{"neplatné UUID autora", `{"author_ids":["nope"]}`},
		{"neplatné UUID série", `{"series_id":"nope"}`},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			var req bookPatchRequest
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if _, err := req.toPatch(); err == nil {
				t.Errorf("toPatch(%s) prošlo, chtěna chyba", tt.body)
			}
		})
	}

	t.Run("prázdné tělo nic nemění", func(t *testing.T) {
		var req bookPatchRequest
		if err := json.Unmarshal([]byte(`{}`), &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		apply, err := req.toPatch()
		if err != nil {
			t.Fatalf("toPatch: %v", err)
		}

		in := base()
		apply(&in)
		if !reflect.DeepEqual(in, base()) {
			t.Errorf("vstup se změnil: %+v", in)
		}
	})

	t.Run("null vyprázdní nullable pole", func(t *testing.T) {
		var req bookPatchRequest
		body := `{"description":null,"narrator":null,"internal_rating":null,"series_id":null}`
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		apply, err := req.toPatch()
		if err != nil {
			t.Fatalf("toPatch: %v", err)
		}

		in := base()
		apply(&in)
		if in.Description != nil || in.Narrator != nil || in.InternalRating != nil || in.SeriesID != nil {
			t.Errorf("nullable pole se nevyprázdnila: %+v", in)
		}
		// Ostatní pole zůstávají nedotčená.
		if in.Title != base().Title || in.FilePath != base().FilePath {
			t.Errorf("změnila se i jiná pole: %+v", in)
		}
	})

	t.Run("poslaná pole se zapíšou", func(t *testing.T) {
		var req bookPatchRequest
		body := `{"title":"  Nový název  ","duration_seconds":60,"internal_rating":5,
		          "author_ids":["22222222-2222-2222-2222-222222222222"]}`
		if err := json.Unmarshal([]byte(body), &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		apply, err := req.toPatch()
		if err != nil {
			t.Fatalf("toPatch: %v", err)
		}

		in := base()
		apply(&in)
		if in.Title != "Nový název" {
			t.Errorf("title = %q, chtěno %q (oříznuté)", in.Title, "Nový název")
		}
		if in.DurationSeconds != 60 {
			t.Errorf("duration_seconds = %d", in.DurationSeconds)
		}
		if in.InternalRating == nil || *in.InternalRating != 5 {
			t.Errorf("internal_rating = %v", in.InternalRating)
		}
		if len(in.AuthorIDs) != 1 || in.AuthorIDs[0].String() != "22222222-2222-2222-2222-222222222222" {
			t.Errorf("author_ids = %v", in.AuthorIDs)
		}
		// file_path v požadavku vůbec není a zůstává netknutý.
		if in.FilePath != base().FilePath {
			t.Errorf("file_path = %q, chtěno %q", in.FilePath, base().FilePath)
		}
	})
}

// file_path je neznámé pole – readJSON ho odmítne, takže ho klient nemá jak přepsat.
func TestBookPatchRejectsFilePath(t *testing.T) {
	r := httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(`{"file_path":"/etc/passwd"}`))
	var req bookPatchRequest
	if err := readJSON(r, &req); err == nil {
		t.Error("readJSON přijal file_path, chtěna chyba")
	}
}
