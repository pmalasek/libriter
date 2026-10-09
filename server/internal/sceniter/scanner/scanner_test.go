package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/sceniter/migrations"
	"mediater/internal/sceniter/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, config.DBConfig{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.MigrateModule(ctx, conn, "sceniter", migrations.FS); err != nil {
		t.Fatal(err)
	}
	return storage.New(conn)
}

func touch(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("not really a video"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func scanOnce(t *testing.T, s *Scanner) Status {
	t.Helper()
	if !s.begin("test") {
		t.Fatal("průchod už běží")
	}
	s.run(context.Background())
	return s.Status()
}

func TestScanLibrary(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"Filmy/21 gramů (2003 - Alejandro González Iñárritu).avi",
		"Filmy/21 gramů (2003 - Alejandro González Iñárritu).avi.vsmeta",
		"Filmy/Křídla vášně - The Wings of the Dove (1997).avi",
		"Filmy/Křídla vášně - The Wings of the Dove (1997).srt",
		"Filmy/Pelíšky (1999)/pelisky.cd1.mkv",
		"Seriály/Docent/S01/Docent - 01x01 - Lesní vražda.mkv",
		"Seriály/Docent/S01/Docent - 01x02 - Preso s mlíčkem.mkv",
		"Seriály/Docent/S03/Docent - 01. díl - Únos.mkv",
		"Seriály/Docent/S03/Docent - 02. díl - Příliš mnoho podezřelých.mkv",
		"Seriály/Docent/S03/@eaDir/Docent - 01. díl - Únos.mkv",
		"Ostatní/Show.Name.S02E05.720p.mkv",
	} {
		touch(t, root, f)
	}

	store := newTestStore(t)
	s := New(root, []string{"Filmy"}, []string{"Seriály"}, store)
	ctx := context.Background()

	st := scanOnce(t, s)
	if st.Processed != 8 || st.Added != 8 {
		t.Fatalf("status %+v, chci 8 zpracovaných i přidaných (soubor v @eaDir se přeskočí)", st)
	}

	movies, err := store.ListMovies(ctx, storage.MovieFilter{})
	if err != nil {
		t.Fatal(err)
	}
	titles := map[string]int{}
	for _, m := range movies {
		y := 0
		if m.Year != nil {
			y = *m.Year
		}
		titles[m.Title] = y
	}
	want := map[string]int{"21 gramů": 2003, "Křídla vášně": 1997, "Pelíšky": 1999}
	if len(titles) != len(want) {
		t.Fatalf("filmy %v, chci %v", titles, want)
	}
	for k, v := range want {
		if titles[k] != v {
			t.Errorf("film %q rok %d, chci %d (filmy %v)", k, titles[k], v, titles)
		}
	}

	shows, err := store.ListShows(ctx, storage.ShowFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(shows) != 2 {
		t.Fatalf("seriály %d, chci 2", len(shows))
	}
	var docentID = shows[0].ID
	if shows[0].Title != "Docent" {
		docentID = shows[1].ID
	}
	seasons, err := store.ListSeasons(ctx, docentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(seasons) != 2 || seasons[0].Number != 1 || seasons[1].Number != 3 {
		t.Fatalf("řady %+v", seasons)
	}
	s3 := seasons[1].Episodes
	if len(s3) != 2 || s3[0].Number != 1 || *s3[0].Title != "Únos" || s3[1].Number != 2 {
		t.Fatalf("díly řady 3: %+v", s3)
	}

	// Titulky vedle filmu.
	for _, m := range movies {
		if m.Title != "Křídla vášně" {
			continue
		}
		media, err := store.ListMedia(ctx, &m.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(media) != 1 || len(media[0].Subtitles) != 1 || media[0].Subtitles[0].Format != "srt" {
			t.Fatalf("titulky: %+v", media)
		}
		if m.OriginalTitle == nil || *m.OriginalTitle != "The Wings of the Dove" {
			t.Errorf("původní název %v", m.OriginalTitle)
		}
	}

	// Druhý průchod nic nového nepřidá.
	if st := scanOnce(t, s); st.Added != 0 || st.Removed != 0 {
		t.Fatalf("druhý průchod %+v", st)
	}

	// Smazaný soubor zmizí, film bez souborů se nevypisuje.
	if err := os.Remove(filepath.Join(root, "Filmy", "21 gramů (2003 - Alejandro González Iñárritu).avi")); err != nil {
		t.Fatal(err)
	}
	if st := scanOnce(t, s); st.Removed != 1 {
		t.Fatalf("po smazání %+v", st)
	}
	movies, _ = store.ListMovies(ctx, storage.MovieFilter{})
	if len(movies) != 2 {
		t.Fatalf("po smazání %d filmů, chci 2", len(movies))
	}
}

func TestScanMissingRootDeletesNothing(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Filmy/Pelíšky (1999).mkv")
	store := newTestStore(t)
	s := New(root, []string{"Filmy"}, nil, store)
	scanOnce(t, s)

	// Nepřipojený disk: kořen zmizí. Knihovna musí zůstat.
	s.root = filepath.Join(root, "neni")
	st := scanOnce(t, s)
	if st.Errors != 1 || st.Removed != 0 {
		t.Fatalf("status %+v", st)
	}
	paths, err := store.ListMediaPaths(context.Background())
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths %v, err %v", paths, err)
	}
}
