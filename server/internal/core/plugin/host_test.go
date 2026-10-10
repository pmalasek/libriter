package plugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mediater/internal/core/plugin"
	"mediater/internal/core/plugin/sdk"
)

// Testovací binárka se zkopíruje do dočasného adresáře jako
// mediater-meta-fake a podle jména v os.Args[0] se chová jako plugin.
func TestMain(m *testing.M) {
	if strings.HasPrefix(filepath.Base(os.Args[0]), plugin.FilePrefix) {
		os.Exit(sdk.Serve(context.Background(), fakePlugin(), os.Args[1:], os.Stdin, os.Stdout))
	}
	os.Exit(m.Run())
}

func fakePlugin() sdk.Plugin {
	return sdk.Plugin{
		Manifest: plugin.Manifest{
			Name: "fake", Title: "Fake", Version: "1.0",
			Kinds:         []string{plugin.KindMovie, plugin.KindShow},
			Config:        []plugin.ConfigField{{Key: "api_key", Label: "API key", Secret: true, Env: "FAKE_PLUGIN_KEY"}},
			URLPatterns:   map[string][]string{plugin.KindMovie: {`^https://fake\.example/film/\d+$`}},
			ImageHosts:    []string{"img.fake.example"},
			MinIntervalMS: 200,
		},
		Search: func(_ context.Context, req plugin.Request) ([]plugin.SearchResult, error) {
			return []plugin.SearchResult{{ID: "1", Title: req.Query.Title, Note: req.Config["api_key"], Year: req.Query.Year}}, nil
		},
		Fetch: func(_ context.Context, req plugin.Request) (any, error) {
			switch req.ID {
			case "missing":
				return nil, sdk.NotFound("nic")
			case "limited":
				return nil, sdk.Errorf(plugin.ErrCodeRateLimited, "HTTP 429")
			case "slow":
				time.Sleep(5 * time.Second)
			case "big":
				_, _ = io.WriteString(os.Stdout, strings.Repeat("x", 5<<20))
				os.Exit(0)
			case "crash":
				_, _ = io.WriteString(os.Stderr, "boom\n")
				os.Exit(2)
			}
			return plugin.VideoMetadata{ID: req.ID, Title: "Film " + req.ID + req.URL}, nil
		},
	}
}

func newHost(t *testing.T, mode os.FileMode) *plugin.Host {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, plugin.FilePrefix+"fake")
	if err := os.WriteFile(path, data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	// Soubor bez předpony se ignoruje.
	_ = os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644)

	h := plugin.NewHost(dir)
	if err := h.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestDiscoverAndSearch(t *testing.T) {
	h := newHost(t, 0o755)
	ps := h.Plugins()
	if len(ps) != 1 || ps[0].Name() != "fake" || ps[0].Manifest.Protocol != plugin.ProtocolVersion {
		t.Fatalf("plugins = %+v", ps)
	}
	if len(h.ByKind(plugin.KindMovie)) != 1 || len(h.ByKind(plugin.KindBook)) != 0 {
		t.Fatal("ByKind")
	}

	t.Setenv("FAKE_PLUGIN_KEY", "from-env")
	h.SetConfigSource(plugin.NewSettings(newMemStore(), errNotFound))
	res, err := h.Search(context.Background(), "fake", plugin.Request{Kind: plugin.KindMovie, Query: &plugin.Query{Title: "Pelíšky", Year: 1999}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Title != "Pelíšky" || res[0].Year != 1999 || res[0].Note != "from-env" {
		t.Fatalf("results = %+v", res)
	}

	if _, err := h.Search(context.Background(), "fake", plugin.Request{Kind: plugin.KindBook, Query: &plugin.Query{Title: "x"}}); !errors.Is(err, plugin.ErrUnsupported) {
		t.Fatalf("book: %v", err)
	}
	if _, err := h.Search(context.Background(), "nope", plugin.Request{Kind: plugin.KindMovie}); !errors.Is(err, plugin.ErrUnknownPlugin) {
		t.Fatalf("unknown: %v", err)
	}
}

func TestFetch(t *testing.T) {
	h := newHost(t, 0o755)
	ctx := context.Background()

	var v plugin.VideoMetadata
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "7"}, &v); err != nil || v.Title != "Film 7" {
		t.Fatalf("fetch: %+v %v", v, err)
	}
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, URL: "https://fake.example/film/8"}, &v); err != nil {
		t.Fatalf("fetch url: %v", err)
	}
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, URL: "https://evil.example/film/8"}, &v); !errors.Is(err, plugin.ErrNotAllowed) {
		t.Fatalf("allowlist: %v", err)
	}
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "missing"}, &v); !plugin.IsNotFound(err) {
		t.Fatalf("not found: %v", err)
	}
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "crash"}, &v); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("crash: %v", err)
	}
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "big"}, &v); err == nil {
		t.Fatal("big: chybí chyba")
	}

	h.SetTimeout(300 * time.Millisecond)
	start := time.Now()
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "slow"}, &v); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("slow: %v", err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("timeout nezabil plugin")
	}
}

func TestMinInterval(t *testing.T) {
	h := newHost(t, 0o755)
	ctx := context.Background()
	var v plugin.VideoMetadata
	start := time.Now()
	for i := 0; i < 3; i++ {
		if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "1"}, &v); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 400*time.Millisecond {
		t.Fatalf("tři volání za %v, odstup 200 ms nedodržen", d)
	}
}

func TestRateLimitPause(t *testing.T) {
	h := newHost(t, 0o755)
	h.SetBackoff(500*time.Millisecond, 2*time.Second)
	ctx := context.Background()
	p, _ := h.Get("fake")
	var v plugin.VideoMetadata

	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "limited"}, &v); !plugin.IsRateLimited(err) {
		t.Fatalf("limited: %v", err)
	}
	if p.PausedUntil().IsZero() {
		t.Fatal("plugin po 429 není pozastavený")
	}
	// Během pauzy volání skončí hned, plugin se nespouští.
	start := time.Now()
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "1"}, &v); !plugin.IsRateLimited(err) {
		t.Fatalf("během pauzy: %v", err)
	}
	if d := time.Since(start); d > 400*time.Millisecond {
		t.Fatalf("volání během pauzy čekalo %v", d)
	}
	if _, _, err := h.DownloadImage(ctx, "fake", "https://img.fake.example/a.jpg"); !plugin.IsRateLimited(err) {
		t.Fatalf("obrázek během pauzy: %v", err)
	}

	time.Sleep(time.Until(p.PausedUntil()) + 50*time.Millisecond)
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "1"}, &v); err != nil {
		t.Fatalf("po pauze: %v", err)
	}
	// Úspěch vynuloval backoff – další pauza je zase nejkratší.
	if err := h.Fetch(ctx, "fake", plugin.Request{Kind: plugin.KindMovie, ID: "limited"}, &v); !plugin.IsRateLimited(err) {
		t.Fatalf("limited 2: %v", err)
	}
	if d := time.Until(p.PausedUntil()); d > 600*time.Millisecond {
		t.Fatalf("pauza %v, backoff se nevynuloval", d)
	}
}

func TestDiscoverRejectsWritable(t *testing.T) {
	h := newHost(t, 0o777)
	if len(h.Plugins()) != 0 {
		t.Fatal("plugin zapisovatelný pro ostatní se nesmí načíst")
	}
}

func TestImageAllowlist(t *testing.T) {
	h := newHost(t, 0o755)
	p, _ := h.Get("fake")
	if !p.SupportsImageURL("https://img.fake.example/a.jpg") || p.SupportsImageURL("https://other.example/a.jpg") {
		t.Fatal("image hosts")
	}
	if _, _, err := h.DownloadImage(context.Background(), "fake", "http://127.0.0.1/a.jpg"); !errors.Is(err, plugin.ErrNotAllowed) {
		t.Fatalf("download: %v", err)
	}
}

func TestSettings(t *testing.T) {
	h := newHost(t, 0o755)
	p, _ := h.Get("fake")
	s := plugin.NewSettings(newMemStore(), errNotFound)
	ctx := context.Background()

	key := "secret"
	if err := s.Set(ctx, p, map[string]*string{"api_key": &key}); err != nil {
		t.Fatal(err)
	}
	st, err := s.State(ctx, p)
	if err != nil || len(st) != 1 || !st[0].IsSet || st[0].Value != "" {
		t.Fatalf("state = %+v %v", st, err)
	}
	cfg, _ := s.PluginConfig(ctx, p)
	if cfg["api_key"] != "secret" {
		t.Fatalf("config = %v", cfg)
	}
	bad := "x"
	if err := s.Set(ctx, p, map[string]*string{"nope": &bad}); !errors.Is(err, plugin.ErrInvalidConfig) {
		t.Fatalf("unknown key: %v", err)
	}
	if err := s.SetDefault(ctx, "fake", "api_key", "other"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := s.PluginConfig(ctx, p); cfg["api_key"] != "secret" {
		t.Fatal("SetDefault přepsal uloženou hodnotu")
	}
}

var errNotFound = errors.New("not found")

type memStore map[string]json.RawMessage

func newMemStore() memStore { return memStore{} }

func (m memStore) GetSetting(_ context.Context, key string) (json.RawMessage, error) {
	v, ok := m[key]
	if !ok {
		return nil, errNotFound
	}
	return v, nil
}

func (m memStore) SetSetting(_ context.Context, key string, value json.RawMessage) error {
	m[key] = value
	return nil
}
