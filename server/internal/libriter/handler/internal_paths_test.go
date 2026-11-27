package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/libriter/migrations"
	"mediater/internal/libriter/scanner"
	"mediater/internal/libriter/service"
	"mediater/internal/libriter/storage"
)

// Interní cesty (file_path, cover_path, image_path) spravuje scanner a služby
// obrázků; podle nich se při mazání mažou soubory. Editor je přes API měnit
// nesmí – jinak by smazáním knihy nebo nahráním nové fotky smazal libovolný
// soubor, ke kterému má server přístup.
func newInternalPathsEnv(t *testing.T) (*storage.Store, http.Handler, string) {
	t.Helper()
	conn, err := db.Open(context.Background(), config.DBConfig{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := migrations.Apply(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	store := storage.New(conn)
	audioRoot := t.TempDir()
	books := NewBook(service.NewBook(store, scanner.New(audioRoot, t.TempDir(), store)), t.TempDir(), nil)
	authors := NewAuthor(service.NewAuthor(store), t.TempDir(), nil, nil)

	r := chi.NewRouter()
	r.Post("/books", books.Create)
	r.Put("/books/{id}", books.Update)
	r.Patch("/books/{id}", books.Patch)
	r.Post("/authors", authors.Create)
	r.Put("/authors/{id}", authors.Update)
	return store, r, audioRoot
}

func sendJSON(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(data)))
	return rec
}

func TestBookUpdateKeepsInternalPaths(t *testing.T) {
	ctx := context.Background()
	store, h, _ := newInternalPathsEnv(t)
	book := createTestBook(t, store)
	if err := store.UpdateBookCoverPath(ctx, book.ID, book.ID.String()+".jpg"); err != nil {
		t.Fatal(err)
	}
	authorID := book.Authors[0].ID.String()

	put := map[string]any{
		"author_ids": []string{authorID}, "title": "Nový název", "duration_seconds": 100,
		"file_path": "../../etc", "cover_path": "../../etc/passwd", "language": "cs",
	}
	if rec := sendJSON(t, h, http.MethodPut, "/books/"+book.ID.String(), put); rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	patch := map[string]any{"cover_path": "../../etc/shadow", "title": "Ještě novější"}
	if rec := sendJSON(t, h, http.MethodPatch, "/books/"+book.ID.String(), patch); rec.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", rec.Code, rec.Body)
	}

	got, err := store.GetBook(ctx, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Ještě novější" {
		t.Errorf("název se neuložil: %q", got.Title)
	}
	if got.FilePath != "capek/ze-zivota-hmyzu" {
		t.Errorf("file_path se změnil: %q", got.FilePath)
	}
	if got.CoverPath == nil || *got.CoverPath != book.ID.String()+".jpg" {
		t.Errorf("cover_path se změnil: %v", got.CoverPath)
	}
}

func TestBookCreateValidatesFilePath(t *testing.T) {
	ctx := context.Background()
	store, h, audioRoot := newInternalPathsEnv(t)
	author := createTestBook(t, store).Authors[0].ID.String()

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(audioRoot, "ven")); err != nil {
		t.Fatal(err)
	}
	for _, fp := range []string{"../mimo", "/etc", "a/../../b", "ven/kniha", "", "."} {
		body := map[string]any{"author_ids": []string{author}, "title": "X", "duration_seconds": 10, "file_path": fp}
		if rec := sendJSON(t, h, http.MethodPost, "/books", body); rec.Code != http.StatusBadRequest {
			t.Errorf("file_path %q: %d, chtěno 400 (%s)", fp, rec.Code, rec.Body)
		}
	}

	body := map[string]any{
		"author_ids": []string{author}, "title": "Válka s mloky", "duration_seconds": 10,
		"file_path": "capek/./valka-s-mloky", "cover_path": "../../etc/passwd",
	}
	rec := sendJSON(t, h, http.MethodPost, "/books", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		ID string `json:"id"`
	}
	decodeJSON(t, rec.Body.Bytes(), &created)
	books, err := store.ListBooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range books {
		if b.ID.String() != created.ID {
			continue
		}
		if b.FilePath != "capek/valka-s-mloky" {
			t.Errorf("file_path = %q", b.FilePath)
		}
		if b.CoverPath != nil {
			t.Errorf("cover_path z API se uložil: %q", *b.CoverPath)
		}
		return
	}
	t.Fatal("založená kniha chybí")
}

func TestAuthorUpdateKeepsImagePath(t *testing.T) {
	ctx := context.Background()
	store, h, _ := newInternalPathsEnv(t)

	rec := sendJSON(t, h, http.MethodPost, "/authors", map[string]any{
		"first_name": "Karel", "last_name": "Poláček", "image_path": "../../etc/passwd",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST: %d %s", rec.Code, rec.Body)
	}
	var created struct {
		ID        string  `json:"id"`
		ImagePath *string `json:"image_path"`
	}
	decodeJSON(t, rec.Body.Bytes(), &created)
	if created.ImagePath != nil {
		t.Fatalf("image_path z POST se uložil: %q", *created.ImagePath)
	}

	stored := "autor-ulozeny.jpg"
	authors, err := store.ListAuthors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range authors {
		if a.ID.String() != created.ID {
			continue
		}
		if _, err := store.PatchAuthor(ctx, a.ID, func(in *storage.AuthorInput) { in.ImagePath = &stored }); err != nil {
			t.Fatal(err)
		}
	}

	// Klient posílá image_path dál (DisallowUnknownFields) – přijme se, ale
	// uložená cesta zůstane.
	rec = sendJSON(t, h, http.MethodPut, "/authors/"+created.ID, map[string]any{
		"first_name": "Karel", "last_name": "Poláček", "bio": "Spisovatel", "image_path": "jiny-soubor.jpg",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	var updated struct {
		Bio       *string `json:"bio"`
		ImagePath *string `json:"image_path"`
	}
	decodeJSON(t, rec.Body.Bytes(), &updated)
	if updated.Bio == nil || *updated.Bio != "Spisovatel" {
		t.Errorf("bio se neuložilo: %v", updated.Bio)
	}
	if updated.ImagePath == nil || *updated.ImagePath != stored {
		t.Errorf("image_path = %v, chtěno %q", updated.ImagePath, stored)
	}
}
