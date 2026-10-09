package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/imagestore"
	"libriter/internal/metadata"
	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func newUploadTestStore(t *testing.T) *storage.Store {
	t.Helper()
	conn, err := db.Open(context.Background(), config.DBConfig{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return storage.New(conn)
}

func postImage(r http.Handler, path string, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	return rec
}

func TestBookUploadCover(t *testing.T) {
	store := newUploadTestStore(t)
	coverRoot := t.TempDir()
	h := NewBook(service.NewBook(store, scanner.New(t.TempDir(), coverRoot, store)), coverRoot, nil).
		WithCovers(service.NewBookCover(store, metadata.NewChain(), coverRoot))

	r := chi.NewRouter()
	r.Post("/books/{id}/cover", h.UploadCover)

	book := createTestBook(t, store)
	path := "/books/" + book.ID.String() + "/cover"

	tests := []struct {
		name string
		path string
		body []byte
		want int
	}{
		{"prázdné tělo", path, nil, http.StatusBadRequest},
		{"není obrázek", path, []byte("<html>ahoj</html>"), http.StatusBadRequest},
		{"příliš velký", path, bytes.Repeat([]byte{0}, imagestore.MaxBytes+1), http.StatusRequestEntityTooLarge},
		{"neznámá kniha", "/books/" + uuid.NewString() + "/cover", pngBytes, http.StatusNotFound},
	}
	for _, tt := range tests {
		if rec := postImage(r, tt.path, tt.body); rec.Code != tt.want {
			t.Errorf("%s: status %d, chtěno %d (%s)", tt.name, rec.Code, tt.want, rec.Body)
		}
	}

	rec := postImage(r, path, pngBytes)
	if rec.Code != http.StatusOK {
		t.Fatalf("nahrání obálky: status %d: %s", rec.Code, rec.Body)
	}
	saved, err := store.GetBook(context.Background(), book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.CoverPath == nil || *saved.CoverPath != book.ID.String()+".png" {
		t.Fatalf("cover_path = %v, chtěno %s.png", saved.CoverPath, book.ID)
	}
	data, err := os.ReadFile(filepath.Join(coverRoot, *saved.CoverPath))
	if err != nil || !bytes.Equal(data, pngBytes) {
		t.Errorf("obálka na disku = %d B, %v", len(data), err)
	}
}

func TestAuthorUploadImage(t *testing.T) {
	ctx := context.Background()
	store := newUploadTestStore(t)
	imageRoot := t.TempDir()
	h := NewAuthor(service.NewAuthor(store), imageRoot,
		service.NewAuthorImage(store, metadata.NewChain(), imageRoot), nil)

	r := chi.NewRouter()
	r.Post("/authors/{id}/image", h.UploadImage)

	author, err := store.CreateAuthor(ctx, storage.AuthorInput{Name: model.AuthorName{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatal(err)
	}
	path := "/authors/" + author.ID.String() + "/image"

	if rec := postImage(r, path, []byte("není obrázek")); rec.Code != http.StatusBadRequest {
		t.Errorf("není obrázek: status %d, chtěno 400", rec.Code)
	}

	upload := func() string {
		t.Helper()
		if rec := postImage(r, path, pngBytes); rec.Code != http.StatusOK {
			t.Fatalf("nahrání fotky: status %d: %s", rec.Code, rec.Body)
		}
		saved, err := store.GetAuthor(ctx, author.ID)
		if err != nil || saved.ImagePath == nil {
			t.Fatalf("image_path chybí: %v", err)
		}
		return *saved.ImagePath
	}

	first := upload()
	if !strings.HasPrefix(first, author.ID.String()) || filepath.Ext(first) != ".png" {
		t.Errorf("image_path = %q", first)
	}

	// Nová fotka musí mít nové jméno (frontend podle něj obchází cache)
	// a ta stará se má smazat.
	second := upload()
	if second == first {
		t.Errorf("druhé nahrání ponechalo jméno %q", first)
	}
	if imagestore.Exists(imageRoot, first) {
		t.Errorf("stará fotka %q zůstala na disku", first)
	}
	if !imagestore.Exists(imageRoot, second) {
		t.Errorf("nová fotka %q chybí", second)
	}
}
