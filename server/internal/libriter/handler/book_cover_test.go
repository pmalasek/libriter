package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/core/safehttp"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/migrations"
	"mediater/internal/libriter/scanner"
	"mediater/internal/libriter/service"
	"mediater/internal/libriter/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// coverStub je zdroj, jehož obálky leží na adrese coverHost („ip:port“
// testovacího serveru). metadata.HostMatches nestandardní port odmítá,
// proto vlastní porovnání.
type coverStub struct {
	stubProvider
	coverHost string
}

func (p *coverStub) SupportsCoverURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host == p.coverHost
}

// pngBytes je nejmenší platné PNG (1×1 px).
var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82")

func TestBookSetCoverFromURL(t *testing.T) {
	// Vnitřní služba, kam se zdroj pokusí server přesměrovat (SSRF).
	var internalHits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		internalHits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Write(pngBytes)
	}))
	defer internal.Close()

	// Server se „zdrojem“ obálek: /cover.png je obrázek, /page.html není,
	// /fake.png tvrdí, že je PNG, ale posílá HTML, /redirect přesměruje
	// na vnitřní službu.
	images := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cover.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(pngBytes)
		case "/fake.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("<!doctype html><script>alert(1)</script>"))
		case "/redirect":
			http.Redirect(w, r, internal.URL+"/cover.png", http.StatusFound)
		default:
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("Connection failed"))
		}
	}))
	defer images.Close()
	imagesURL, _ := url.Parse(images.URL)

	conn, err := db.Open(context.Background(), config.DBConfig{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := migrations.Apply(context.Background(), conn); err != nil {
		t.Fatalf("migrace knihovny: %v", err)
	}
	store := storage.New(conn)
	coverRoot := t.TempDir()

	chain := metadata.NewChain(&coverStub{stubProvider: stubProvider{name: "zdroj"}, coverHost: imagesURL.Host})
	// Stahuje se přes safehttp; povolená je jen adresa „zdroje“, vnitřní
	// služba na jiném portu zůstává zakázaná.
	fetcher := metadata.NewFetcher(0).WithHTTPOptions(safehttp.AllowAddr(images.Listener.Addr().String()))
	h := NewBook(service.NewBook(store, scanner.New(t.TempDir(), coverRoot, store)), coverRoot, nil).
		WithCovers(service.NewBookCover(store, chain, coverRoot).WithFetcher(fetcher))

	r := chi.NewRouter()
	r.Put("/books/{id}/cover", h.SetCover)
	r.Get("/books/{id}/cover", h.Cover)

	book := createTestBook(t, store)
	put := func(bookID uuid.UUID, coverURL string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		body := strings.NewReader(`{"url":"` + coverURL + `"}`)
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/books/"+bookID.String()+"/cover", body))
		return rec
	}

	tests := []struct {
		name   string
		bookID uuid.UUID
		url    string
		want   int
	}{
		{"cizí hostitel", book.ID, "https://evil.example/cover.png", http.StatusBadRequest},
		{"odpověď není obrázek", book.ID, images.URL + "/page.html", http.StatusBadGateway},
		{"HTML vydávající se za PNG", book.ID, images.URL + "/fake.png", http.StatusBadGateway},
		{"přesměrování na vnitřní adresu", book.ID, images.URL + "/redirect", http.StatusBadGateway},
		{"neznámá kniha", uuid.New(), images.URL + "/cover.png", http.StatusNotFound},
	}
	for _, tt := range tests {
		if rec := put(tt.bookID, tt.url); rec.Code != tt.want {
			t.Errorf("%s: status %d, chtěno %d (%s)", tt.name, rec.Code, tt.want, rec.Body)
		}
	}

	if internalHits.Load() != 0 {
		t.Fatal("přesměrování dovedlo server na vnitřní adresu")
	}
	if entries, _ := os.ReadDir(coverRoot); len(entries) != 0 {
		t.Fatalf("neplatná obálka se uložila: %v", entries)
	}

	rec := put(book.ID, images.URL+"/cover.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("stažení obálky: status %d: %s", rec.Code, rec.Body)
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

	// Uložená obálka jde rovnou servírovat.
	get := httptest.NewRecorder()
	r.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/books/"+book.ID.String()+"/cover", nil))
	if get.Code != http.StatusOK || !bytes.Equal(get.Body.Bytes(), pngBytes) {
		t.Errorf("GET /cover: status %d, %d B", get.Code, get.Body.Len())
	}
}
