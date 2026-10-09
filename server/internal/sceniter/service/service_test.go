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
	coreservice "mediater/internal/core/service"
	corestorage "mediater/internal/core/storage"
	"mediater/internal/sceniter/migrations"
	"mediater/internal/sceniter/storage"
	"mediater/internal/sceniter/tmdb"
)

type env struct {
	store  *storage.Store
	lib    *Library
	userID uuid.UUID
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
	return &env{store: store, lib: NewLibrary(store), userID: u.ID}
}

func (e *env) movie(t *testing.T, title string) (movieID, mediaID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	id, err := e.store.EnsureMovie(ctx, storage.MovieKey{SourceKey: "Filmy/" + title, Title: title})
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
	id, err := e.store.EnsureEpisode(ctx, storage.EpisodeKey{ShowKey: "Seriály/" + show, ShowTitle: show, Season: season, Episode: number})
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
	if d.UpNext == nil || d.UpNext.ID != e2 || len(d.Seasons) != 2 || d.Seasons[0].Episodes[0].Progress == nil {
		t.Fatalf("show detail %+v", d)
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
	res := []tmdb.SearchResult{
		{ID: 1, Title: "Pelíšky", OriginalTitle: "Pelíšky", Year: 1999},
		{ID: 2, Title: "Pelíšky 2", Year: 2010},
	}
	if id, ok := pickResult(res, "Pelisky", "", 1999); !ok || id != 1 {
		t.Errorf("rok + název bez diakritiky: %d %v", id, ok)
	}
	if id, ok := pickResult(res, "Pelíšky", "", 2000); !ok || id != 1 {
		t.Errorf("rok ±1: %d %v", id, ok)
	}
	if _, ok := pickResult(res, "Pelíšky", "", 0); ok {
		t.Error("bez roku a víc výsledků se nesmí párovat automaticky")
	}
	if _, ok := pickResult(res, "Úplně jiný", "", 1980); ok {
		t.Error("jiný rok i název")
	}
	if id, ok := pickResult(res[:1], "Pelíšky", "", 0); !ok || id != 1 {
		t.Error("jediný výsledek se shodným názvem")
	}
}
