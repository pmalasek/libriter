package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	coreservice "mediater/internal/core/service"
	corestorage "mediater/internal/core/storage"
	"mediater/internal/sceniter/handler"
	"mediater/internal/sceniter/migrations"
	"mediater/internal/sceniter/scanner"
	"mediater/internal/sceniter/service"
	"mediater/internal/sceniter/storage"
	"mediater/internal/sceniter/stream"
)

type aspectEnv struct {
	router              http.Handler
	store               *storage.Store
	auth                *coreservice.AuthService
	users               *coreservice.UserService
	movieID, movieMedia uuid.UUID
	showID, episodeID   uuid.UUID
	episodeMedia        uuid.UUID
}

func newAspectEnv(t *testing.T) *aspectEnv {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, config.DBConfig{Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.MigrateModule(ctx, conn, "sceniter", migrations.FS); err != nil {
		t.Fatal(err)
	}
	core := corestorage.New(conn)
	auth := coreservice.NewAuth(core, config.JWTConfig{Secret: "test-secret", ExpiryHours: time.Hour})
	store := storage.New(conn)

	h := handler.New(handler.Deps{
		Store: store, Library: service.NewLibrary(store), Auth: auth,
		HLS:     stream.NewManager(t.TempDir(), 1, time.Minute),
		Scanner: scanner.New(t.TempDir(), store),
	})
	r := chi.NewRouter()
	h.Routes(r)

	e := &aspectEnv{router: r, store: store, auth: auth, users: coreservice.NewUser(core, auth)}

	movies, err := store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Filmy", Name: "Filmy", Kind: "movie", Language: "cs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	shows, err := store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Seriály", Name: "Seriály", Kind: "show", Language: "cs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.movieID, err = store.EnsureMovie(ctx, storage.MovieKey{LibraryID: movies.ID, SourceKey: "Filmy/Pelíšky", Title: "Pelíšky"}); err != nil {
		t.Fatal(err)
	}
	if e.movieMedia, err = store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: "Filmy/Pelíšky.mkv", MovieID: &e.movieID,
		SizeBytes: 1, ModTime: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if e.episodeID, err = store.EnsureEpisode(ctx, storage.EpisodeKey{LibraryID: shows.ID, ShowKey: "Seriály/Arabela",
		ShowTitle: "Arabela", Season: 1, Episode: 1}); err != nil {
		t.Fatal(err)
	}
	ep, err := store.GetEpisode(ctx, e.episodeID)
	if err != nil {
		t.Fatal(err)
	}
	e.showID = ep.ShowID
	if e.episodeMedia, err = store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: "Seriály/Arabela/s01e01.mkv",
		EpisodeID: &e.episodeID, SizeBytes: 1, ModTime: time.Now()}); err != nil {
		t.Fatal(err)
	}
	// Sdílená výchozí hodnota seriálu (dřívější globální volba).
	if _, err := conn.ExecContext(ctx, `UPDATE sc_shows SET aspect_ratio = '16:9' WHERE id = ?1`, e.showID); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *aspectEnv) login(t *testing.T, email string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := e.users.Create(ctx, "Divák", email, "", "tajneheslo", "reader"); err != nil {
		t.Fatal(err)
	}
	_, token, err := e.auth.Login(ctx, email, "tajneheslo")
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func (e *aspectEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "/sceniter"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// aspect vrátí aspect_ratio z JSON odpovědi GET (u dílu z vnořeného show).
func (e *aspectEnv) aspect(t *testing.T, path, token string) string {
	t.Helper()
	rec := e.do(t, http.MethodGet, path, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var body struct {
		AspectRatio *string `json:"aspect_ratio"`
		Show        *struct {
			AspectRatio *string `json:"aspect_ratio"`
		} `json:"show"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	r := body.AspectRatio
	if body.Show != nil {
		r = body.Show.AspectRatio
	}
	if r == nil {
		return "<nil>"
	}
	return *r
}

func (e *aspectEnv) setAspect(t *testing.T, kind string, id uuid.UUID, token string, ratio *string) {
	t.Helper()
	rec := e.do(t, http.MethodPut, "/aspect/"+kind+"/"+id.String(), token, map[string]*string{"aspect_ratio": ratio})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT aspect %s: %d %s", kind, rec.Code, rec.Body.String())
	}
}

func strp(s string) *string { return &s }

// Poměr stran zvolený jedním uživatelem vidí jen on – v detailu titulu,
// v detailu dílu i v odpovědi pro přehrávač. Ostatní mají sdílenou hodnotu.
func TestAspectRatioPerUser(t *testing.T) {
	e := newAspectEnv(t)
	alice := e.login(t, "alena@example.com")
	bob := e.login(t, "bohous@example.com")

	movie := "/movies/" + e.movieID.String()
	moviePlayback := "/media/" + e.movieMedia.String() + "/playback"
	show := "/shows/" + e.showID.String()
	episode := "/episodes/" + e.episodeID.String()
	episodePlayback := "/media/" + e.episodeMedia.String() + "/playback"

	e.setAspect(t, "movie", e.movieID, alice, strp("4:3"))
	e.setAspect(t, "show", e.showID, alice, strp("4:3"))

	for _, c := range []struct{ path, token, want string }{
		{movie, alice, "4:3"},
		{moviePlayback, alice, "4:3"},
		{show, alice, "4:3"},
		{episode, alice, "4:3"},
		{episodePlayback, alice, "4:3"},
		// Bohouš nic nezvolil: film podle souboru, seriál sdílených 16:9.
		{movie, bob, "<nil>"},
		{moviePlayback, bob, "<nil>"},
		{show, bob, "16:9"},
		{episode, bob, "16:9"},
		{episodePlayback, bob, "16:9"},
	} {
		if got := e.aspect(t, c.path, c.token); got != c.want {
			who := "Alena"
			if c.token == bob {
				who = "Bohouš"
			}
			t.Errorf("%s %s: %s, chce %s", who, c.path, got, c.want)
		}
	}

	// null smaže volbu – Alena má zase sdílenou hodnotu.
	e.setAspect(t, "show", e.showID, alice, nil)
	if got := e.aspect(t, episodePlayback, alice); got != "16:9" {
		t.Errorf("po smazání volby: %s, chce 16:9", got)
	}
	if got := e.aspect(t, movie, alice); got != "4:3" {
		t.Errorf("film po smazání volby seriálu: %s, chce 4:3", got)
	}
}

func TestAspectRatioValidation(t *testing.T) {
	e := newAspectEnv(t)
	token := e.login(t, "alena@example.com")

	if rec := e.do(t, http.MethodPut, "/aspect/movie/"+e.movieID.String(), token, map[string]string{"aspect_ratio": "21:9"}); rec.Code != http.StatusBadRequest {
		t.Errorf("neplatný poměr: %d", rec.Code)
	}
	if rec := e.do(t, http.MethodPut, "/aspect/movie/"+uuid.NewString(), token, map[string]string{"aspect_ratio": "4:3"}); rec.Code != http.StatusNotFound {
		t.Errorf("neexistující film: %d", rec.Code)
	}
}
