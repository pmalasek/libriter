package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestResolveCoverPath(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name      string
		coverRoot string
		coverPath string
		want      bool
	}{
		{"holý název souboru", root, "abc.jpg", true},
		{"prázdný coverRoot", "", "abc.jpg", false},
		{"prázdná cesta", root, "", false},
		{"tečka", root, ".", false},
		{"dvě tečky", root, "..", false},
		{"únik nahoru", root, "../secret.txt", false},
		{"podadresář", root, "sub/x.jpg", false},
		{"absolutní cesta", root, "/etc/passwd", false},
		{"zpětné lomítko", root, `..\x.jpg`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abs, ok := resolveCoverPath(tt.coverRoot, tt.coverPath)
			if ok != tt.want {
				t.Fatalf("resolveCoverPath(%q, %q) ok = %v, chtěno %v", tt.coverRoot, tt.coverPath, ok, tt.want)
			}
			if !ok {
				return
			}
			want := filepath.Join(tt.coverRoot, tt.coverPath)
			if abs != want {
				t.Errorf("cesta = %q, chtěno %q", abs, want)
			}
		})
	}
}

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
