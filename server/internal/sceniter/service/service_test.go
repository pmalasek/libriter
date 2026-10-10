package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/core/plugin"
	coreservice "mediater/internal/core/service"
	corestorage "mediater/internal/core/storage"
	"mediater/internal/sceniter/migrations"
	"mediater/internal/sceniter/storage"
)

type env struct {
	store             *storage.Store
	lib               *Library
	userID            uuid.UUID
	movieLib, showLib uuid.UUID
}

func newEnv(t *testing.T) *env {
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
	u, err := coreservice.NewUser(corestorage.New(conn), nil).Create(ctx, "Divák", "divak@example.com", "", "tajneheslo", "reader")
	if err != nil {
		t.Fatal(err)
	}
	store := storage.New(conn)
	movies, err := store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Filmy", Name: "Filmy", Kind: "movie", Language: "cs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	shows, err := store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Seriály", Name: "Seriály", Kind: "show", Language: "cs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	return &env{store: store, lib: NewLibrary(store), userID: u.ID, movieLib: movies.ID, showLib: shows.ID}
}

func (e *env) movie(t *testing.T, title string) (movieID, mediaID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	id, err := e.store.EnsureMovie(ctx, storage.MovieKey{LibraryID: e.movieLib, SourceKey: "Filmy/" + title, Title: title})
	if err != nil {
		t.Fatal(err)
	}
	mid, err := e.store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: "Filmy/" + title + ".mkv", MovieID: &id,
		SizeBytes: 1, ModTime: time.Now(), Probe: &storage.Probe{DurationSec: 6000}})
	if err != nil {
		t.Fatal(err)
	}
	return id, mid
}

func (e *env) episode(t *testing.T, show string, season, number int) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id, err := e.store.EnsureEpisode(ctx, storage.EpisodeKey{LibraryID: e.showLib, ShowKey: "Seriály/" + show, ShowTitle: show, Season: season, Episode: number})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("Seriály", show, uuid.NewString()+".mkv")
	if _, err := e.store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: path, EpisodeID: &id, SizeBytes: 1, ModTime: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestSaveProgressNewerWins(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	movieID, mediaID := e.movie(t, "Pelíšky")

	t0 := time.Now().Add(-time.Hour)
	save := func(pos float64, at time.Time, watched int) error {
		return e.lib.SaveProgress(ctx, e.userID, ProgressRequest{MovieID: &movieID, MediaID: &mediaID,
			PositionSec: pos, DurationSec: 6000, RecordedAt: &at, WatchedSeconds: watched})
	}
	if err := save(600, t0.Add(10*time.Minute), 60); err != nil {
		t.Fatal(err)
	}
	// Starší událost z jiného zařízení pozici nepřepíše, sekundy ale započte.
	if err := save(100, t0, 30); !errors.Is(err, storage.ErrStale) {
		t.Fatalf("starší zápis: %v, chci ErrStale", err)
	}
	p, err := e.store.GetProgress(ctx, e.userID, &movieID, nil)
	if err != nil || p.PositionSec != 600 || p.Completed {
		t.Fatalf("progress %+v, %v", p, err)
	}
	log, _ := e.store.WatchLog(ctx, e.userID, 10)
	total := 0
	for _, d := range log {
		total += d.Seconds
	}
	if total != 90 {
		t.Fatalf("odkoukáno %d s, chci 90", total)
	}

	// 95 % délky = dokoukáno.
	if err := save(5700, t0.Add(20*time.Minute), 0); err != nil {
		t.Fatal(err)
	}
	p, _ = e.store.GetProgress(ctx, e.userID, &movieID, nil)
	if !p.Completed {
		t.Fatal("95 % délky není dokoukané")
	}

	if err := e.lib.SaveProgress(ctx, e.userID, ProgressRequest{PositionSec: 1}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bez titulu: %v", err)
	}
}

func TestSaveProgressDuplicateEvent(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	movieID, _ := e.movie(t, "Amadeus")
	ev := uuid.New()
	req := ProgressRequest{MovieID: &movieID, PositionSec: 10, WatchedSeconds: 10, EventID: &ev, DeviceID: "tv"}
	if err := e.lib.SaveProgress(ctx, e.userID, req); err != nil {
		t.Fatal(err)
	}
	if err := e.lib.SaveProgress(ctx, e.userID, req); !errors.Is(err, storage.ErrDuplicateEvent) {
		t.Fatalf("opakovaná událost: %v", err)
	}
}

func TestContinueAndUpNext(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e1 := e.episode(t, "Docent", 1, 1)
	e2 := e.episode(t, "Docent", 1, 2)
	e3 := e.episode(t, "Docent", 2, 1)
	movieID, _ := e.movie(t, "Altamira")

	if err := e.lib.MarkWatched(ctx, e.userID, "episode", e1, true); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(time.Second)
	if err := e.lib.SaveProgress(ctx, e.userID, ProgressRequest{MovieID: &movieID, PositionSec: 300, DurationSec: 6000, RecordedAt: &at}); err != nil {
		t.Fatal(err)
	}

	items, err := e.lib.Continue(ctx, e.userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Kind != "movie" || items[1].Episode.ID != e2 || !items[1].Next {
		t.Fatalf("continue %+v", items)
	}

	shows, _ := e.store.ListShows(ctx, storage.ShowFilter{})
	d, err := e.lib.Show(ctx, e.userID, shows[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.UpNext == nil || d.UpNext.ID != e2 || len(d.Seasons) != 2 || d.Seasons[0].WatchedCount != 1 {
		t.Fatalf("show detail %+v", d)
	}
	season, err := e.lib.Season(ctx, e.userID, shows[0].ID, d.Seasons[0].Number)
	if err != nil {
		t.Fatal(err)
	}
	if len(season.Episodes) != d.Seasons[0].EpisodeCount || season.Episodes[0].Progress == nil {
		t.Fatalf("řada %+v", season)
	}
	if _, err := e.lib.Season(ctx, e.userID, shows[0].ID, 99); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("neexistující řada: %v", err)
	}

	// Poslední díl dokoukaný → seriál z „pokračovat“ zmizí.
	for _, id := range []uuid.UUID{e2, e3} {
		time.Sleep(10 * time.Millisecond)
		if err := e.lib.MarkWatched(ctx, e.userID, "episode", id, true); err != nil {
			t.Fatal(err)
		}
	}
	items, _ = e.lib.Continue(ctx, e.userID, 10)
	if len(items) != 1 || items[0].Kind != "movie" {
		t.Fatalf("po dokoukání %+v", items)
	}
}

func TestPickResult(t *testing.T) {
	res := []plugin.SearchResult{
		{ID: "1", Title: "Pelíšky", OriginalTitle: "Pelíšky", Year: 1999},
		{ID: "2", Title: "Pelíšky 2", Year: 2010},
	}
	if id, ok := pickResult(res, "Pelisky", "", 1999, true); !ok || id != "1" {
		t.Errorf("rok + název bez diakritiky: %s %v", id, ok)
	}
	if id, ok := pickResult(res, "Pelíšky", "", 2000, true); !ok || id != "1" {
		t.Errorf("rok ±1: %s %v", id, ok)
	}
	if id, ok := pickResult(res, "Pelíšky", "", 0, true); !ok || id != "1" {
		t.Errorf("bez roku: jediný výsledek se shodným názvem: %s %v", id, ok)
	}
	twins := append(res, plugin.SearchResult{ID: "3", Title: "Pelíšky", Year: 2021})
	if _, ok := pickResult(twins, "Pelíšky", "", 0, true); ok {
		t.Error("bez roku a dva stejně pojmenované tituly se nesmí párovat automaticky")
	}
	if _, ok := pickResult(res, "Úplně jiný", "", 1980, true); ok {
		t.Error("jiný rok i název")
	}
	if id, ok := pickResult(res[:1], "Pelíšky", "", 0, true); !ok || id != "1" {
		t.Error("jediný výsledek se shodným názvem")
	}
}

func TestPickResultHeuristics(t *testing.T) {
	pretty := []plugin.SearchResult{
		{ID: "1", Title: "Pretty Woman", Year: 1990, Votes: 9233},
		{ID: "2", Title: "Pretty Woman", Year: 1991, Votes: 18},
	}
	if id, ok := pickResult(pretty, "Pretty Woman", "", 0, true); !ok || id != "1" {
		t.Errorf("výrazně nejznámější ze stejně pojmenovaných: %s %v", id, ok)
	}
	pretty[1].Votes = 2000
	if _, ok := pickResult(pretty, "Pretty Woman", "", 0, true); ok {
		t.Error("dva podobně známé stejně pojmenované tituly – rozhodne člověk")
	}

	badBoys := []plugin.SearchResult{
		{ID: "1", Title: "Mizerové", OriginalTitle: "Bad Boys", Year: 1995},
		{ID: "2", Title: "Mizerové: Na život a na smrt", OriginalTitle: "Bad Boys: Ride or Die", Year: 2024},
	}
	if id, ok := pickResult(badBoys, "Mizerové", "Na život a na smrt", 0, true); !ok || id != "2" {
		t.Errorf("název s podtitulem: %s %v", id, ok)
	}
	jumanji := []plugin.SearchResult{{ID: "1", Title: "Jumanji", OriginalTitle: "Jumanji", Year: 1995}}
	if _, ok := pickResult(jumanji, "Jumanji", "Další level", 0, true); ok {
		t.Error("shoda bez podtitulu a bez roku se nesmí párovat")
	}
	hunger := []plugin.SearchResult{{ID: "1", Title: "Hunger Games", Year: 2012}}
	if _, ok := pickResult(hunger, "Hunger Games", "Vražedná pomsta", 2013, true); ok {
		t.Error("shoda bez podtitulu jen s rokem ±1 se nesmí párovat")
	}
	wings := []plugin.SearchResult{{ID: "1", Title: "Křídla vášně", OriginalTitle: "The Wings of the Dove", Year: 1997}}
	if id, ok := pickResult(wings, "Křídla vášně", "The Wings of the Dove", 1998, true); !ok || id != "1" {
		t.Errorf("český a původní název: %s %v", id, ok)
	}

	plusOne := []plugin.SearchResult{{ID: "1", Title: "+1", OriginalTitle: "+1", Year: 2013}}
	if _, ok := pickResult(plusOne, "1", "", 0, true); ok {
		t.Error("číselný název bez roku se nesmí párovat")
	}

	police := []plugin.SearchResult{{ID: "1", Title: "Policejní akademie 4: Občanská patrola", Year: 1987}}
	if id, ok := pickResult(police, "Policejni Akademie 4", "", 0, true); !ok || id != "1" {
		t.Errorf("jediný výsledek začínající názvem: %s %v", id, ok)
	}
	if _, ok := pickResult(police, "Policejni", "", 0, true); ok {
		t.Error("prefix z jednoho slova se nesmí párovat")
	}
	fast := []plugin.SearchResult{{ID: "1", Title: "Rychle a zběsile 10", Year: 2023}}
	if _, ok := pickResult(fast, "Rychle a zběsile 1", "", 0, true); ok {
		t.Error("prefix pokračující číslicí je jiný díl série")
	}

	// Hledání omezené rokem vrací první výsledek s tím rokem vždy.
	riddick := []plugin.SearchResult{{ID: "1", Title: "Into Pitch Black", Year: 2000}}
	if _, ok := pickResult(riddick, "Riddick", "", 2000, false); ok {
		t.Error("první výsledek hledání omezeného rokem se nesmí brát jen kvůli roku")
	}
	if id, ok := pickResult(riddick, "Riddick", "", 2000, true); !ok || id != "1" {
		t.Errorf("první výsledek neomezeného hledání s přesným rokem: %s %v", id, ok)
	}
}
