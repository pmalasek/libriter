package service

import (
	"context"
	"testing"
	"time"

	"mediater/internal/sceniter/storage"
)

// Rozměry obrazu jdou do výpisů (štítek 4K) – u filmu z jeho souborů,
// u seriálu maximum přes díly.
func TestVideoSizeInListings(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	uhd, _ := e.movie(t, "Duna")
	if _, err := e.store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: "Filmy/Duna 4K.mkv", MovieID: &uhd,
		SizeBytes: 2, ModTime: time.Now(), Probe: &storage.Probe{DurationSec: 6000, Width: 3840, Height: 2160}}); err != nil {
		t.Fatal(err)
	}
	e.movie(t, "Bez rozměrů")

	movies, err := e.store.ListMovies(ctx, storage.MovieFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range movies {
		switch m.Title {
		case "Duna":
			if m.VideoWidth == nil || *m.VideoWidth != 3840 || m.VideoHeight == nil || *m.VideoHeight != 2160 {
				t.Errorf("Duna: %v×%v", m.VideoWidth, m.VideoHeight)
			}
		case "Bez rozměrů":
			if m.VideoWidth != nil || m.VideoHeight != nil {
				t.Errorf("film bez rozměrů: %v×%v", *m.VideoWidth, *m.VideoHeight)
			}
		}
	}
	detail, err := e.store.GetMovie(ctx, uhd)
	if err != nil || detail.VideoWidth == nil || *detail.VideoWidth != 3840 {
		t.Fatalf("detail: %+v %v", detail, err)
	}

	ep := e.episode(t, "Fallout", 1, 1)
	e.episode(t, "Fallout", 1, 2)
	if _, err := e.store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: "Seriály/Fallout/uhd.mkv", EpisodeID: &ep,
		SizeBytes: 2, ModTime: time.Now(), Probe: &storage.Probe{Width: 3840, Height: 1600}}); err != nil {
		t.Fatal(err)
	}
	shows, err := e.store.ListShows(ctx, storage.ShowFilter{})
	if err != nil || len(shows) != 1 {
		t.Fatalf("shows: %+v %v", shows, err)
	}
	if s := shows[0]; s.VideoWidth == nil || *s.VideoWidth != 3840 || *s.VideoHeight != 1600 {
		t.Errorf("seriál: %v×%v", s.VideoWidth, s.VideoHeight)
	}
	if got := shows[0].Summary().LatestEpisodeAt; got.IsZero() {
		t.Error("ShowSummary bez latest_episode_at")
	}
}

// Obrázek knihovny pro rozcestník: pozadí nejlépe hodnoceného titulu,
// jinak plakát; knihovna bez obrázků žádný nemá.
func TestLibraryCover(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	libs := NewLibraries(e.store, t.TempDir(), nil, nil)

	list, err := libs.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range list {
		if l.CoverPath != nil {
			t.Fatalf("prázdná knihovna %s má obrázek %q", l.Name, *l.CoverPath)
		}
	}

	low, _ := e.movie(t, "Slabý")
	top, _ := e.movie(t, "Nejlepší")
	posterOnly, _ := e.movie(t, "Jen plakát")
	for _, u := range []struct {
		id               any
		rating           int
		backdrop, poster any
	}{
		{low, 50, "low-bd.jpg", "low-p.jpg"},
		{top, 90, "top-bd.jpg", "top-p.jpg"},
		{posterOnly, 99, nil, "only-p.jpg"},
	} {
		if _, err := e.conn.ExecContext(ctx, `UPDATE sc_movies SET rating = ?2, backdrop_path = ?3, poster_path = ?4 WHERE id = ?1`,
			u.id, u.rating, u.backdrop, u.poster); err != nil {
			t.Fatal(err)
		}
	}

	list, err = libs.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range list {
		if l.ID == e.movieLib && (l.CoverPath == nil || *l.CoverPath != "top-bd.jpg" || l.CoverUpdatedAt == nil) {
			t.Errorf("obrázek filmové knihovny: %v", l.CoverPath)
		}
	}
	admin, err := libs.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range admin {
		if l.CoverPath != nil {
			t.Error("administrace obrázek knihovny nepotřebuje")
		}
	}
}
