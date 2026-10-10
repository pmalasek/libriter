package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mediater/internal/core/plugin"
	"mediater/internal/core/plugin/sdk"
	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
)

// Testovací binárka se zkopíruje jako mediater-meta-fake a podle jména
// v os.Args[0] se chová jako plugin (viz core/plugin/host_test.go).
func TestMain(m *testing.M) {
	if strings.HasPrefix(filepath.Base(os.Args[0]), plugin.FilePrefix) {
		name := strings.TrimPrefix(filepath.Base(os.Args[0]), plugin.FilePrefix)
		os.Exit(sdk.Serve(context.Background(), fakePlugin(name), os.Args[1:], os.Stdin, os.Stdout))
	}
	os.Exit(m.Run())
}

// fakePlugin najde každý titul pod jeho názvem; „Blok“ odpoví 429,
// „Neznámý“ nenajde nic. Záložní zdroj „backup“ najde i „Neznámý“.
func fakePlugin(name string) sdk.Plugin {
	return sdk.Plugin{
		Manifest: plugin.Manifest{Name: name, Title: name, Version: "1.0", Kinds: []string{plugin.KindMovie}},
		Search: func(_ context.Context, req plugin.Request) ([]plugin.SearchResult, error) {
			switch req.Query.Title {
			case "Blok":
				return nil, sdk.Errorf(plugin.ErrCodeRateLimited, "HTTP 429")
			case "Neznámý":
				if name != "backup" {
					return nil, nil
				}
			}
			return []plugin.SearchResult{{ID: req.Query.Title, Title: req.Query.Title}}, nil
		},
		Fetch: func(_ context.Context, req plugin.Request) (any, error) {
			return plugin.VideoMetadata{ID: req.ID, Title: req.ID}, nil
		},
	}
}

func newFakeHost(t *testing.T, names ...string) *plugin.Host {
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
	if len(names) == 0 {
		names = []string{"fake"}
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, plugin.FilePrefix+n), data, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h := plugin.NewHost(dir)
	if err := h.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestAutoMatchStopsOnRateLimit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lib, err := e.store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Zdroj", Name: "Zdroj", Kind: "movie",
		Language: "cs", Provider: "fake", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.movieLib = lib.ID
	// Párování jde podle názvu: Alfa, Blok (429), Cyklon.
	alfa, _ := e.movie(t, "Alfa")
	_, _ = e.movie(t, "Blok")
	cyklon, _ := e.movie(t, "Cyklon")

	m := NewMatcher(e.store, newFakeHost(t), t.TempDir())
	m.autoMatch(ctx, false)
	t.Cleanup(func() {
		m.mu.Lock()
		if m.retry != nil {
			m.retry.Stop()
		}
		m.mu.Unlock()
	})

	state := func(id uuid.UUID) string {
		mv, err := e.store.GetMovie(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return mv.MatchState
	}
	if s := state(alfa); s != model.MatchAuto {
		t.Fatalf("Alfa: %s", s)
	}
	if s := state(cyklon); s != model.MatchUnmatched {
		t.Fatalf("Cyklon se po 429 neměl zkoušet: %s", s)
	}
	m.mu.Lock()
	scheduled := m.retry != nil
	m.mu.Unlock()
	if !scheduled {
		t.Fatal("po 429 není naplánované další párování")
	}
}

func TestAutoMatchRetryWindow(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lib, err := e.store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Zdroj", Name: "Zdroj", Kind: "movie",
		Language: "cs", Provider: "fake", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.movieLib = lib.ID
	unknown, _ := e.movie(t, "Neznámý")

	pending := func() int {
		mv, err := e.store.ListMovies(ctx, storage.MovieFilter{Library: lib.ID, MatchState: model.MatchUnmatched,
			WithoutFiles: true, RetryDays: matchRetryDays})
		if err != nil {
			t.Fatal(err)
		}
		return len(mv)
	}
	if n := pending(); n != 1 {
		t.Fatalf("před párováním: %d", n)
	}
	m := NewMatcher(e.store, newFakeHost(t), t.TempDir())
	m.autoMatch(ctx, false)
	if n := pending(); n != 0 {
		t.Fatalf("nepovedený titul se má %d dní vynechat, čeká %d", matchRetryDays, n)
	}
	// Ruční vrácení na „nespárovaný“ lhůtu zruší.
	if err := e.store.SetMatchState(ctx, model.KindMovie, unknown, model.MatchUnmatched); err != nil {
		t.Fatal(err)
	}
	if n := pending(); n != 1 {
		t.Fatalf("po ručním vrácení: %d", n)
	}
}

// Co zdroj knihovny nenajde, zkusí se v ostatních zdrojích. Průběh počítá
// spárované i nenalezené tituly.
func TestAutoMatchFallsBackToOtherProviders(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lib, err := e.store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Zdroj", Name: "Zdroj", Kind: "movie",
		Language: "cs", Provider: "fake", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	e.movieLib = lib.ID
	alfa, _ := e.movie(t, "Alfa")
	unknown, _ := e.movie(t, "Neznámý")

	m := NewMatcher(e.store, newFakeHost(t, "fake", "backup"), t.TempDir())
	m.autoMatch(ctx, false)

	for id, want := range map[uuid.UUID]string{alfa: "fake", unknown: "backup"} {
		mv, err := e.store.GetMovie(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if mv.MatchState != model.MatchAuto || mv.MetaProvider == nil || *mv.MetaProvider != want {
			t.Errorf("%s: stav %s, zdroj %v, chci %s", mv.Title, mv.MatchState, mv.MetaProvider, want)
		}
	}
	if st := m.MatchStatusNow(); st.Running || st.Total != 2 || st.Done != 2 || st.Matched != 2 || st.Failed != 0 {
		t.Errorf("průběh %+v", st)
	}
}
