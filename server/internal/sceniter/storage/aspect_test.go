package storage_test

import (
	"context"
	"database/sql"
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
)

type aspectEnv struct {
	conn                 *sql.DB
	store                *storage.Store
	alice, bob           uuid.UUID
	movieID, movieMedia  uuid.UUID
	showID, episodeMedia uuid.UUID
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
	users := coreservice.NewUser(corestorage.New(conn), nil)
	alice, err := users.Create(ctx, "Alena", "alena@example.com", "", "tajneheslo", "reader")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := users.Create(ctx, "Bohouš", "bohous@example.com", "", "tajneheslo", "reader")
	if err != nil {
		t.Fatal(err)
	}

	e := &aspectEnv{conn: conn, store: storage.New(conn), alice: alice.ID, bob: bob.ID}
	movies, err := e.store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Filmy", Name: "Filmy", Kind: "movie", Language: "cs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	shows, err := e.store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Seriály", Name: "Seriály", Kind: "show", Language: "cs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	e.movieID, err = e.store.EnsureMovie(ctx, storage.MovieKey{LibraryID: movies.ID, SourceKey: "Filmy/Pelíšky", Title: "Pelíšky"})
	if err != nil {
		t.Fatal(err)
	}
	e.movieMedia, err = e.store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: "Filmy/Pelíšky.mkv", MovieID: &e.movieID,
		SizeBytes: 1, ModTime: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	episodeID, err := e.store.EnsureEpisode(ctx, storage.EpisodeKey{LibraryID: shows.ID, ShowKey: "Seriály/Arabela",
		ShowTitle: "Arabela", Season: 1, Episode: 1})
	if err != nil {
		t.Fatal(err)
	}
	ep, err := e.store.GetEpisode(ctx, episodeID)
	if err != nil {
		t.Fatal(err)
	}
	e.showID = ep.ShowID
	e.episodeMedia, err = e.store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: "Seriály/Arabela/s01e01.mkv",
		EpisodeID: &episodeID, SizeBytes: 1, ModTime: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func strp(s string) *string { return &s }

func ratioOf(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

func (e *aspectEnv) mediaRatio(t *testing.T, user, media uuid.UUID) string {
	t.Helper()
	r, err := e.store.MediaAspectRatio(context.Background(), user, media)
	if err != nil {
		t.Fatal(err)
	}
	return ratioOf(r)
}

// Volba jednoho uživatele se druhého netýká; kdo nic nezvolil, má sdílenou
// výchozí hodnotu titulu.
func TestAspectRatioIsPerUser(t *testing.T) {
	e := newAspectEnv(t)
	ctx := context.Background()

	// Sdílená výchozí hodnota z dob před migrací 011.
	if _, err := e.conn.ExecContext(ctx, `UPDATE sc_movies SET aspect_ratio = '16:9' WHERE id = ?1`, e.movieID); err != nil {
		t.Fatal(err)
	}

	if err := e.store.SetAspectRatio(ctx, e.alice, &e.movieID, nil, strp("4:3")); err != nil {
		t.Fatal(err)
	}
	if got := e.mediaRatio(t, e.alice, e.movieMedia); got != "4:3" {
		t.Errorf("Alena: %s, chce 4:3", got)
	}
	if got := e.mediaRatio(t, e.bob, e.movieMedia); got != "16:9" {
		t.Errorf("Bohouš (bez volby): %s, chce sdílených 16:9", got)
	}
	if r, err := e.store.UserAspectRatio(ctx, e.bob, &e.movieID, nil); err != nil || r != nil {
		t.Errorf("Bohouš nemá mít volbu: %v %v", ratioOf(r), err)
	}

	// Sdílený sloupec volba uživatele nemění.
	m, err := e.store.GetMovie(ctx, e.movieID)
	if err != nil {
		t.Fatal(err)
	}
	if ratioOf(m.AspectRatio) != "16:9" {
		t.Errorf("sdílená hodnota filmu: %s", ratioOf(m.AspectRatio))
	}

	// Přepsání volby a pak smazání (nil) vrátí sdílenou hodnotu.
	if err := e.store.SetAspectRatio(ctx, e.alice, &e.movieID, nil, strp("16:9")); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetAspectRatio(ctx, e.alice, &e.movieID, nil, strp("4:3")); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetAspectRatio(ctx, e.alice, &e.movieID, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := e.mediaRatio(t, e.alice, e.movieMedia); got != "16:9" {
		t.Errorf("Alena po smazání volby: %s, chce sdílených 16:9", got)
	}
}

// U seriálu platí volba pro soubory všech jeho dílů.
func TestAspectRatioShowAppliesToEpisodes(t *testing.T) {
	e := newAspectEnv(t)
	ctx := context.Background()

	if got := e.mediaRatio(t, e.alice, e.episodeMedia); got != "<nil>" {
		t.Errorf("bez volby a výchozí hodnoty: %s", got)
	}
	if err := e.store.SetAspectRatio(ctx, e.bob, nil, &e.showID, strp("4:3")); err != nil {
		t.Fatal(err)
	}
	if got := e.mediaRatio(t, e.bob, e.episodeMedia); got != "4:3" {
		t.Errorf("Bohouš: %s, chce 4:3", got)
	}
	if got := e.mediaRatio(t, e.alice, e.episodeMedia); got != "<nil>" {
		t.Errorf("Alena: %s, chce podle souboru", got)
	}
	// Volba seriálu se nepropíše do filmu.
	if got := e.mediaRatio(t, e.bob, e.movieMedia); got != "<nil>" {
		t.Errorf("film Bohouše: %s", got)
	}
}

func TestAspectRatioUnknownTitle(t *testing.T) {
	e := newAspectEnv(t)
	ctx := context.Background()
	missing := uuid.New()
	if err := e.store.SetAspectRatio(ctx, e.alice, &missing, nil, strp("4:3")); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("neexistující film: %v", err)
	}
	if err := e.store.SetAspectRatio(ctx, e.alice, nil, &missing, nil); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("neexistující seriál (mazání): %v", err)
	}
}
