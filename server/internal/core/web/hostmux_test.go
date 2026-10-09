package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func named(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(name)) })
}

func TestHostMux(t *testing.T) {
	m := NewHostMux(named("libriter"))
	m.Handle([]string{"Video.Example.com", "sceniter.localhost"}, named("sceniter"))

	cases := map[string]string{
		"video.example.com":       "sceniter",
		"VIDEO.example.com:443":   "sceniter",
		"video.example.com.":      "sceniter",
		"sceniter.localhost:8080": "sceniter",
		"audiobooks.example.com":  "libriter",
		"localhost:8080":          "libriter",
		"127.0.0.1":               "libriter",
	}
	for host, want := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		m.ServeHTTP(rec, req)
		if got := rec.Body.String(); got != want {
			t.Errorf("host %q → %q, chci %q", host, got, want)
		}
	}
}

func TestNewServesIndexAndAssets(t *testing.T) {
	h := New(fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa</html>")},
		"assets/app.js": {Data: []byte("js")},
		"favicon.svg":   {Data: []byte("svg")},
	}, "sceniter")

	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		return rec
	}
	if rec := get("/movies/123"); rec.Code != 200 || rec.Body.String() != "<html>spa</html>" || rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("SPA fallback: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
	}
	if rec := get("/assets/app.js"); rec.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Errorf("assets cache: %q", rec.Header().Get("Cache-Control"))
	}
	if rec := get("/favicon.svg"); rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("favicon cache: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestNewNotBuilt(t *testing.T) {
	rec := httptest.NewRecorder()
	New(fstest.MapFS{".gitkeep": {}}, "sceniter").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("stav %d, chci 503", rec.Code)
	}
	if want := "Sceniter"; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("stránka neobsahuje %q", want)
	}
}
