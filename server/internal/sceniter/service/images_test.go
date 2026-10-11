package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"mediater/internal/sceniter/storage"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Ruční plakát přežije obnovu metadat a po odebrání se vrátí plakát ze zdroje.
func TestTitleImagesManualPosterSurvivesRefresh(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	root := t.TempDir()
	images := NewTitleImages(e.store, root)
	id, _ := e.movie(t, "Nikola Tesla")

	if err := images.Upload(ctx, "movie", id, storage.ImagePoster, []byte("not an image")); err != ErrNotImage {
		t.Fatalf("cizí data: %v", err)
	}
	if err := images.Upload(ctx, "movie", id, storage.ImagePoster, pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	m, err := e.store.GetMovie(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := "poster-manual-movie-" + id.String() + ".png"
	if m.PosterPath == nil || *m.PosterPath != want || m.PosterSource != "manual" {
		t.Fatalf("po nahrání: path=%v source=%q", m.PosterPath, m.PosterSource)
	}
	if _, err := os.Stat(filepath.Join(root, want)); err != nil {
		t.Fatal(err)
	}

	// Obnova metadat: zdroj dodá vlastní plakát i pozadí – plakát zůstane ruční, pozadí se vezme.
	provider := "poster-movie-" + id.String() + ".jpg"
	if _, err := e.store.ApplyMovieMeta(ctx, id, storage.TitleMeta{Provider: "fake", ID: "1", Title: "Nikola Tesla",
		PosterPath: provider, BackdropPath: "backdrop-movie-" + id.String() + ".jpg"}, "auto"); err != nil {
		t.Fatal(err)
	}
	m, _ = e.store.GetMovie(ctx, id)
	if *m.PosterPath != want || m.PosterSource != "manual" {
		t.Fatalf("po obnově: path=%s source=%q", *m.PosterPath, m.PosterSource)
	}
	if m.BackdropPath == nil || m.BackdropSource != "" {
		t.Fatalf("pozadí ze zdroje: path=%v source=%q", m.BackdropPath, m.BackdropSource)
	}

	// Odebrání ručního: na disku leží soubor ze zdroje → vrátí se.
	if err := os.WriteFile(filepath.Join(root, provider), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := images.Remove(ctx, "movie", id, storage.ImagePoster); err != nil {
		t.Fatal(err)
	}
	m, _ = e.store.GetMovie(ctx, id)
	if m.PosterPath == nil || *m.PosterPath != provider || m.PosterSource != "" {
		t.Fatalf("po odebrání: path=%v source=%q", m.PosterPath, m.PosterSource)
	}
	if _, err := os.Stat(filepath.Join(root, want)); !os.IsNotExist(err) {
		t.Fatalf("ruční soubor má být smazán: %v", err)
	}

	// Bez souboru ze zdroje zůstane titul bez obrázku.
	if err := images.Upload(ctx, "movie", id, storage.ImagePoster, pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(root, provider))
	if err := images.Remove(ctx, "movie", id, storage.ImagePoster); err != nil {
		t.Fatal(err)
	}
	m, _ = e.store.GetMovie(ctx, id)
	if m.PosterPath != nil || m.PosterSource != "" {
		t.Fatalf("bez zdroje: path=%v source=%q", m.PosterPath, m.PosterSource)
	}
}
