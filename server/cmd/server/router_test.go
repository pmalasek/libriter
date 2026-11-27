package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/core/middleware"
	"mediater/internal/core/plugin"
	"mediater/internal/core/product"
	"mediater/internal/core/service"
	"mediater/internal/core/storage"
)

// testRouter sestaví router přesně jako runServe, nad dočasnou databází.
func testRouter(t *testing.T) http.Handler {
	t.Helper()
	r, _ := testRouterDeps(t)
	return r
}

// testRouterDeps je testRouter, který vrátí i závislosti (zakládání účtů).
func testRouterDeps(t *testing.T) (http.Handler, product.Deps) {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Server:         config.ServerConfig{Env: "test"},
		DB:             config.DBConfig{Path: filepath.Join(dir, "test.db")},
		JWT:            config.JWTConfig{Secret: "test-secret", ExpiryHours: time.Hour},
		DefaultProduct: "libriter",
		Storage: config.StorageConfig{
			AudioRoot:       filepath.Join(dir, "audio"),
			CoverRoot:       filepath.Join(dir, "covers"),
			AuthorImageRoot: filepath.Join(dir, "author-images"),
			ImportRoot:      filepath.Join(dir, "import"),
			MaxUploadMB:     10,
		},
	}
	if err := cfg.Storage.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.Storage.AudioRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	conn, err := db.Open(ctx, cfg.DB)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	prods := products()
	for _, p := range prods {
		if err := db.MigrateModule(ctx, conn, p.Name(), p.Migrations()); err != nil {
			t.Fatal(err)
		}
	}
	store := storage.New(conn)
	auth := service.NewAuth(store, cfg.JWT)
	pluginSettings := plugin.NewSettings(store, storage.ErrNotFound)
	pluginHost := plugin.NewHost(filepath.Join(dir, "plugins"))
	pluginHost.SetConfigSource(pluginSettings)
	deps := product.Deps{
		Ctx: ctx, Cfg: cfg, DB: conn, Auth: auth,
		Users:          service.NewUser(store, auth),
		Registration:   service.NewRegistration(store),
		Audit:          service.NewAudit(store),
		Plugins:        pluginHost,
		PluginSettings: pluginSettings,
		StartedAt:      time.Now(),
	}
	for _, p := range prods {
		if err := p.Init(deps); err != nil {
			t.Fatalf("%s.Init: %v", p.Name(), err)
		}
	}
	return newRouter(cfg, deps, prods), deps
}

// Vydaná mobilní aplikace volá API Libriteru na pevných cestách. Každá cesta
// z doby před rozdělením na jádro a produkty musí existovat se stejnou metodou.
func TestRoutesKeepV1Compatibility(t *testing.T) {
	r := testRouter(t)

	have := map[string]bool{}
	err := chi.Walk(r.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.ReplaceAll(route, "/*/", "/")
		have[method+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile("testdata/routes_v1.txt")
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if !have[line] {
			missing = append(missing, line)
		}
	}
	if len(missing) > 0 {
		var got []string
		for k := range have {
			got = append(got, k)
		}
		sort.Strings(got)
		t.Errorf("chybějící cesty:\n  %s\n\nrouter má:\n  %s", strings.Join(missing, "\n  "), strings.Join(got, "\n  "))
	}
}

func TestUnknownAPIPathIsJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/nonsense", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("stav %d, typ %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

// Bezpečnostní hlavičky dostane API, SPA i /health. API má přísnější CSP,
// protože se nikdy nezobrazuje jako stránka.
func TestSecurityHeaders(t *testing.T) {
	r := testRouter(t)

	tests := []struct {
		name, method, path string
		wantStatus         int
		wantCSP            string
	}{
		{"veřejné API", http.MethodGet, "/api/v1/auth/config", http.StatusOK, middleware.APIContentSecurityPolicy},
		{"API bez přihlášení", http.MethodGet, "/api/v1/users/me", http.StatusUnauthorized, middleware.APIContentSecurityPolicy},
		{"neznámé API", http.MethodGet, "/api/v1/nonsense", http.StatusNotFound, middleware.APIContentSecurityPolicy},
		{"chybná metoda API", http.MethodDelete, "/api/v1/auth/config", http.StatusMethodNotAllowed, middleware.APIContentSecurityPolicy},
		// Frontend v testech sestavený není, SPA vrací stránku s návodem (503).
		{"SPA kořen", http.MethodGet, "/", http.StatusServiceUnavailable, middleware.WebContentSecurityPolicy},
		{"SPA podstránka", http.MethodGet, "/books/123", http.StatusServiceUnavailable, middleware.WebContentSecurityPolicy},
		{"health", http.MethodGet, "/health", http.StatusOK, middleware.WebContentSecurityPolicy},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.wantStatus {
				t.Errorf("stav %d, chci %d", rec.Code, tc.wantStatus)
			}
			h := rec.Header()
			for key, want := range map[string]string{
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
				"Content-Security-Policy": tc.wantCSP,
			} {
				if got := h.Get(key); got != want {
					t.Errorf("%s = %q, chci %q", key, got, want)
				}
			}
			// HSTS přidává nginx, server sám HTTPS neobsluhuje.
			if got := h.Get("Strict-Transport-Security"); got != "" {
				t.Errorf("Strict-Transport-Security = %q, server ji nastavovat nemá", got)
			}
		})
	}
}
