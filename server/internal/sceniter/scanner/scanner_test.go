package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/sceniter/migrations"
	"mediater/internal/sceniter/model"
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

func addLibrary(t *testing.T, store *storage.Store, dir, kind string) {
	t.Helper()
	if _, err := store.CreateLibrary(context.Background(), storage.LibraryInput{
		Dir: dir, Name: dir, Kind: kind, Language: "cs", Enabled: true,
	}); err != nil {
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
		// Podsložky ve filmech jen třídí: každý soubor je film, díly cdN jsou jeden film.
		"Filmy/Sbírka/Pelíšky (1999).cd1.mkv",
		"Filmy/Sbírka/Pelíšky (1999).cd2.mkv",
		"Filmy/Divadelní hry/Lakomec.avi",
		"Seriály/Docent/S01/Docent - 01x01 - Lesní vražda.mkv",
		"Seriály/Docent/S01/Docent - 01x02 - Preso s mlíčkem.mkv",
		"Seriály/Docent/S03/Docent - 01. díl - Únos.mkv",
		"Seriály/Docent/S03/Docent - 02. díl - Příliš mnoho podezřelých.mkv",
		"Seriály/Docent/S03/@eaDir/Docent - 01. díl - Únos.mkv",
		"Dokumentární cykly/Show.Name.S02E05.720p.mkv",
		// Mimo knihovny – ignoruje se.
		"Ostatní/Jiný film (2001).mkv",
		"Volně ležící (2002).mkv",
	} {
		touch(t, root, f)
	}

	store := newTestStore(t)
	addLibrary(t, store, "Filmy", "movie")
	addLibrary(t, store, "Seriály", "show")
	addLibrary(t, store, "Dokumentární cykly", "show")
	s := New(root, store)
	ctx := context.Background()

	st := scanOnce(t, s)
	if st.Processed != 10 || st.Added != 10 {
		t.Fatalf("status %+v, chci 10 zpracovaných i přidaných (soubor v @eaDir a mimo knihovny se přeskočí)", st)
	}
	libs, err := store.ListLibraries(ctx, false)
	if err != nil || len(libs) != 3 {
		t.Fatalf("knihovny %+v %v", libs, err)
	}
	if libs[0].ItemCount != 4 || libs[1].ItemCount != 1 || libs[2].ItemCount != 1 {
		t.Errorf("počty položek %d/%d/%d, chci 4/1/1", libs[0].ItemCount, libs[1].ItemCount, libs[2].ItemCount)
	}
	if shows, _ := store.ListShows(ctx, storage.ShowFilter{Library: libs[2].ID}); len(shows) != 1 || shows[0].Title != "Show Name" {
		t.Errorf("seriály v knihovně dokumentů: %+v", shows)
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
	want := map[string]int{"21 gramů": 2003, "Křídla vášně": 1997, "Pelíšky": 1999, "Lakomec": 0}
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
	if len(movies) != 3 {
		t.Fatalf("po smazání %d filmů, chci 3", len(movies))
	}
}

func TestScanMissingRootDeletesNothing(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Filmy/Pelíšky (1999).mkv")
	store := newTestStore(t)
	addLibrary(t, store, "Filmy", "movie")
	s := New(root, store)
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

func TestScanMissingLibraryDirDeletesNothing(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Filmy/Pelíšky (1999).mkv")
	touch(t, root, "Seriály/Docent/S01E01.mkv")
	store := newTestStore(t)
	addLibrary(t, store, "Filmy", "movie")
	addLibrary(t, store, "Seriály", "show")
	s := New(root, store)
	scanOnce(t, s)

	// Složka jedné knihovny zmizí (přejmenovaná) – její soubory zůstanou,
	// druhá knihovna se projde normálně.
	if err := os.Rename(filepath.Join(root, "Seriály"), filepath.Join(root, "Serialy")); err != nil {
		t.Fatal(err)
	}
	st := scanOnce(t, s)
	if st.Errors != 1 || st.Removed != 0 {
		t.Fatalf("status %+v", st)
	}
	paths, err := store.ListMediaPaths(context.Background())
	if err != nil || len(paths) != 2 {
		t.Fatalf("paths %v, err %v", paths, err)
	}
}

func TestDeleteLibraryRemovesItems(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "Filmy/Pelíšky (1999).mkv")
	store := newTestStore(t)
	addLibrary(t, store, "Filmy", "movie")
	s := New(root, store)
	scanOnce(t, s)
	ctx := context.Background()
	libs, _ := store.ListLibraries(ctx, false)
	if err := store.DeleteLibrary(ctx, libs[0].ID); err != nil {
		t.Fatal(err)
	}
	paths, _ := store.ListMediaPaths(ctx)
	movies, _ := store.ListMovies(ctx, storage.MovieFilter{WithoutFiles: true})
	if len(paths) != 0 || len(movies) != 0 {
		t.Fatalf("po smazání knihovny zbylo %d souborů a %d filmů", len(paths), len(movies))
	}
}

// Díl očíslovaný podle pořadí v adresáři (soubory se kopírovaly postupně)
// se při další kontrole přeřadí podle čísla v názvu souboru.
func TestScanReassignsEpisodeByName(t *testing.T) {
	root := t.TempDir()
	for i := 1; i <= 4; i++ {
		touch(t, root, fmt.Sprintf("Cykly/Bitva o nebe nad Británii/Bitva_o_nebe_nad_Británií_%d.avi", i))
	}
	store := newTestStore(t)
	addLibrary(t, store, "Cykly", "show")
	s := New(root, store)
	scanOnce(t, s)
	ctx := context.Background()

	shows, _ := store.ListShows(ctx, storage.ShowFilter{})
	if len(shows) != 1 || shows[0].EpisodeCount != 4 {
		t.Fatalf("seriály %+v", shows)
	}

	// Stav z databáze uživatele: díl _4 je omylem u 3. dílu.
	seasons, _ := store.ListSeasons(ctx, shows[0].ID)
	third := seasons[0].Episodes[2].ID
	paths, _ := store.ListMediaPaths(ctx)
	p4 := "Cykly/Bitva o nebe nad Británii/Bitva_o_nebe_nad_Británií_4.avi"
	if _, ok := paths[p4]; !ok {
		t.Fatalf("soubor chybí: %v", paths)
	}
	if _, err := store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: p4, EpisodeID: &third, SizeBytes: 18, ModTime: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if shows, _ := store.ListShows(ctx, storage.ShowFilter{}); shows[0].EpisodeCount != 3 {
		t.Fatalf("příprava: %d dílů", shows[0].EpisodeCount)
	}

	scanOnce(t, s)
	if shows, _ := store.ListShows(ctx, storage.ShowFilter{}); shows[0].EpisodeCount != 4 {
		t.Fatalf("po kontrole %d dílů, chci 4", shows[0].EpisodeCount)
	}
}

// Filmy založené dřív podle podsložky se při kontrole rozdělí na filmy podle
// souborů. Film s jediným souborem se jen přejmenuje a nechá si metadata.
func TestScanReassignsLegacyFolderMovies(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{
		"Filmy/Divadelní hry/Lakomec.avi",
		"Filmy/Divadelní hry/Hamlet (1990).avi",
		"Filmy/Pelíšky (1999)/pelisky.avi",
	} {
		touch(t, root, f)
	}
	store := newTestStore(t)
	addLibrary(t, store, "Filmy", "movie")
	ctx := context.Background()
	libs, _ := store.ListLibraries(ctx, false)

	// Stav podle starého pravidla: podsložka = film.
	legacy := func(key, title string, year int, files ...string) {
		id, err := store.EnsureMovie(ctx, storage.MovieKey{LibraryID: libs[0].ID, SourceKey: key, Title: title, Year: year})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.SetMatchState(ctx, "movie", id, "manual"); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if _, err := store.SaveMedia(ctx, storage.SaveMediaInput{FilePath: f, MovieID: &id, SizeBytes: 18, ModTime: time.Now()}); err != nil {
				t.Fatal(err)
			}
		}
	}
	legacy("Filmy/Divadelní hry", "Divadelní hry", 0, "Filmy/Divadelní hry/Lakomec.avi", "Filmy/Divadelní hry/Hamlet (1990).avi")
	legacy("Filmy/Pelíšky (1999)", "Pelíšky", 1999, "Filmy/Pelíšky (1999)/pelisky.avi")

	scanOnce(t, New(root, store))

	movies, _ := store.ListMovies(ctx, storage.MovieFilter{})
	got := map[string]string{}
	for _, m := range movies {
		got[m.Title] = m.MatchState
	}
	want := map[string]string{"Lakomec": "unmatched", "Hamlet": "unmatched", "Pelíšky": "manual"}
	if len(got) != len(want) {
		t.Fatalf("filmy %v, chci %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("film %q stav %q, chci %q (filmy %v)", k, got[k], v, got)
		}
	}
}

func TestDetachMergedFile(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Filmy/Dej si pohov kámoši - 1.avi", "Filmy/R.E.D/RED - 1.avi"} {
		touch(t, root, f)
	}
	store := newTestStore(t)
	addLibrary(t, store, "Filmy", "movie")
	ctx := context.Background()
	s := New(root, store)
	scanOnce(t, s)

	// Chybné párování obou filmů na stejný titul je sloučí.
	var ids []string
	for _, f := range []string{"Filmy/Dej si pohov kámoši - 1.avi", "Filmy/R.E.D/RED - 1.avi"} {
		rec, err := store.MediaByPath(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		merged, err := store.ApplyMovieMeta(ctx, *rec.MovieID, storage.TitleMeta{Provider: "tmdb", ID: "176068", Title: "+1"}, "auto")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, merged.String())
	}
	if ids[0] != ids[1] {
		t.Fatalf("filmy se nesloučily: %v", ids)
	}

	red, _ := store.MediaByPath(ctx, "Filmy/R.E.D/RED - 1.avi")
	main, _ := store.MediaByPath(ctx, "Filmy/Dej si pohov kámoši - 1.avi")
	if _, err := s.DetachFile(ctx, main.ID); err != ErrPrimaryFile {
		t.Errorf("oddělení souboru, který drží film: %v", err)
	}
	id, err := s.DetachFile(ctx, red.ID)
	if err != nil {
		t.Fatal(err)
	}
	if id.String() == ids[0] {
		t.Fatal("soubor zůstal u sloučeného filmu")
	}
	m, err := store.GetMovie(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "RED" || m.MatchState != "unmatched" || m.MatchAttemptedAt == nil {
		t.Errorf("oddělený film %+v", m)
	}
	// Další průchod soubor nechá u odděleného filmu.
	scanOnce(t, s)
	if rec, _ := store.MediaByPath(ctx, "Filmy/R.E.D/RED - 1.avi"); *rec.MovieID != id {
		t.Errorf("po průchodu soubor patří k %s, chci %s", rec.MovieID, id)
	}
}

func TestPositionInDirSkipsNumbered(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"X 1.avi", "X 2.avi", "bez-cisla.avi", "jiny.avi"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := positionInDir(filepath.Join(dir, "bez-cisla.avi"), "X", 0); got != 3 {
		t.Errorf("bez-cisla = %d, chci 3", got)
	}
	if got := positionInDir(filepath.Join(dir, "jiny.avi"), "X", 0); got != 4 {
		t.Errorf("jiny = %d, chci 4", got)
	}
}

func TestEpisodeKeyAbsoluteNumbering(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Colombo", "S05")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Colombo 32.Případ.avi", "Colombo 33.Smrt.avi", "Colombo 34.Aréna.avi"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib := &model.Library{}
	parts := []string{"Colombo", "S05", "Colombo 33.Smrt.avi"}
	key, ok := episodeKey(lib, filepath.Join(dir, parts[2]), parts, []string{"Seriály"}, "Colombo 33.Smrt", "")
	if !ok || key.Season != 5 || key.Episode != 2 {
		t.Errorf("Colombo 33 v S05 = S%dE%d (%v), chci S5E2", key.Season, key.Episode, ok)
	}
}

// Přečíslování složky přejmenuje soubory (i titulky) na disku, přeřadí je
// k dílům a další průchod je nechá být.
func TestRenumberFolder(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Serialy/Show/S01/a.mkv", "Serialy/Show/S01/b.mkv", "Serialy/Show/S01/b.cs.srt", "Serialy/Show/S01/c.mkv"} {
		touch(t, root, f)
	}
	store := newTestStore(t)
	addLibrary(t, store, "Serialy", "show")
	s := New(root, store)
	scanOnce(t, s)
	ctx := context.Background()

	shows, _ := store.ListShows(ctx, storage.ShowFilter{})
	if len(shows) != 1 || shows[0].EpisodeCount != 3 {
		t.Fatalf("seriály %+v", shows)
	}
	showID := shows[0].ID
	media := func(p string) *storage.MediaRecord {
		t.Helper()
		rec, err := store.MediaByPath(ctx, p)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		return rec
	}
	a, b, c := media("Serialy/Show/S01/a.mkv"), media("Serialy/Show/S01/b.mkv"), media("Serialy/Show/S01/c.mkv")

	// Soubor mimo složku a duplicitní číslo jsou neplatné zadání.
	if _, err := s.PlanRenumber(ctx, showID, "Serialy/Show", 1, []RenumberFile{{a.ID, 1}}); !errors.Is(err, ErrRenumberInvalid) {
		t.Errorf("soubor mimo složku: %v", err)
	}
	if _, err := s.PlanRenumber(ctx, showID, "Serialy/Show/S01", 1, []RenumberFile{{a.ID, 1}, {b.ID, 1}}); !errors.Is(err, ErrRenumberInvalid) {
		t.Errorf("duplicitní díl: %v", err)
	}

	order := []RenumberFile{{c.ID, 1}, {a.ID, 2}, {b.ID, 3}}
	plan, err := s.PlanRenumber(ctx, showID, "Serialy/Show/S01", 1, order)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 3 || plan.Files[0].NewPath != "Serialy/Show/S01/S01E01 - c.mkv" || len(plan.Files[2].Subtitles) != 1 ||
		plan.Files[2].Subtitles[0].NewPath != "Serialy/Show/S01/S01E03 - b.cs.srt" {
		t.Fatalf("plán %+v", plan.Files)
	}
	// Náhled nic nezměnil.
	if _, err := os.Stat(filepath.Join(root, "Serialy/Show/S01/a.mkv")); err != nil {
		t.Fatal("náhled přejmenoval soubor")
	}

	// Kolize s existujícím souborem.
	touch(t, root, "Serialy/Show/S01/S01E02 - a.mkv")
	if _, err := s.Renumber(ctx, showID, "Serialy/Show/S01", 1, order); !errors.Is(err, ErrRenameConflict) {
		t.Fatalf("kolize: %v", err)
	}
	os.Remove(filepath.Join(root, "Serialy/Show/S01/S01E02 - a.mkv"))

	if _, err := s.Renumber(ctx, showID, "Serialy/Show/S01", 1, order); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"S01E01 - c.mkv", "S01E02 - a.mkv", "S01E03 - b.mkv", "S01E03 - b.cs.srt"} {
		if _, err := os.Stat(filepath.Join(root, "Serialy/Show/S01", f)); err != nil {
			t.Errorf("na disku chybí %s", f)
		}
	}
	seasons, _ := store.ListSeasons(ctx, showID)
	if len(seasons) != 1 || len(seasons[0].Episodes) != 3 {
		t.Fatalf("řady %+v", seasons)
	}
	want := map[int]uuid.UUID{1: c.ID, 2: a.ID, 3: b.ID}
	for _, e := range seasons[0].Episodes {
		files, _ := store.ListMedia(ctx, nil, &e.ID)
		if len(files) != 1 || files[0].ID != want[e.Number] {
			t.Errorf("díl %d: soubory %+v", e.Number, files)
		}
		if e.Number == 3 && len(files[0].Subtitles) != 1 {
			t.Errorf("díl 3 bez titulků: %+v", files[0])
		}
	}

	// Další průchod nic nepřeřadí ani nezaloží znovu.
	st := scanOnce(t, s)
	if st.Added != 0 || st.Removed != 0 {
		t.Errorf("průchod po přečíslování: %+v", st)
	}
	if rec := media("Serialy/Show/S01/S01E02 - a.mkv"); rec.ID != a.ID {
		t.Errorf("soubor a má nové ID")
	}
	seasons, _ = store.ListSeasons(ctx, showID)
	if len(seasons[0].Episodes) != 3 {
		t.Errorf("po průchodu %d dílů", len(seasons[0].Episodes))
	}
}

// Díl ležící naplocho v knihovně dostane do názvu i seriál, aby po
// přejmenování patřil ke stejnému seriálu.
func TestRenumberFlatKeepsShow(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Serialy/Pan casu S01E42.mkv", "Serialy/Pan casu S01E43.mkv"} {
		touch(t, root, f)
	}
	store := newTestStore(t)
	addLibrary(t, store, "Serialy", "show")
	s := New(root, store)
	scanOnce(t, s)
	ctx := context.Background()
	shows, _ := store.ListShows(ctx, storage.ShowFilter{})
	if len(shows) != 1 {
		t.Fatalf("seriály %+v", shows)
	}
	a, _ := store.MediaByPath(ctx, "Serialy/Pan casu S01E42.mkv")
	b, _ := store.MediaByPath(ctx, "Serialy/Pan casu S01E43.mkv")
	plan, err := s.Renumber(ctx, shows[0].ID, "Serialy", 2, []RenumberFile{{b.ID, 1}, {a.ID, 2}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Files[0].NewPath != "Serialy/Pan casu S02E01.mkv" {
		t.Errorf("plán %+v", plan.Files)
	}
	scanOnce(t, s)
	shows, _ = store.ListShows(ctx, storage.ShowFilter{})
	if len(shows) != 1 || shows[0].EpisodeCount != 2 {
		t.Errorf("po průchodu seriály %+v", shows)
	}
}

// Koš: soubor i titulky se přesunou do .trash knihovny se zachovanou
// cestou, záznam zmizí a průchod ho nezaloží znovu. Jediný soubor filmu
// se nemaže.
func TestTrashFile(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"Serialy/Show/S01/a.mkv", "Serialy/Show/S01/a.cs.srt", "Serialy/Show/S01/b.mkv", "Filmy/Film.mkv"} {
		touch(t, root, f)
	}
	store := newTestStore(t)
	addLibrary(t, store, "Serialy", "show")
	addLibrary(t, store, "Filmy", "movie")
	s := New(root, store)
	scanOnce(t, s)
	ctx := context.Background()

	film, _ := store.MediaByPath(ctx, "Filmy/Film.mkv")
	if _, err := s.TrashFile(ctx, film.ID); !errors.Is(err, ErrLastFile) {
		t.Errorf("jediný soubor filmu: %v", err)
	}

	a, _ := store.MediaByPath(ctx, "Serialy/Show/S01/a.mkv")
	dest, err := s.TrashFile(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dest != "Serialy/.trash/Show/S01/a.mkv" {
		t.Errorf("cesta v koši %s", dest)
	}
	for _, f := range []string{"Serialy/.trash/Show/S01/a.mkv", "Serialy/.trash/Show/S01/a.cs.srt"} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("v koši chybí %s", f)
		}
	}
	if _, err := store.MediaByPath(ctx, "Serialy/Show/S01/a.mkv"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("záznam zůstal: %v", err)
	}
	// Stejný název podruhé dostane pořadové číslo.
	touch(t, root, "Serialy/Show/S01/a.mkv")
	scanOnce(t, s)
	a2, _ := store.MediaByPath(ctx, "Serialy/Show/S01/a.mkv")
	if dest, err := s.TrashFile(ctx, a2.ID); err != nil || dest != "Serialy/.trash/Show/S01/a (2).mkv" {
		t.Errorf("druhý koš: %s, %v", dest, err)
	}
	st := scanOnce(t, s)
	if st.Added != 0 {
		t.Errorf("průchod založil soubor z koše: %+v", st)
	}
	if shows, _ := store.ListShows(ctx, storage.ShowFilter{}); len(shows) != 1 || shows[0].EpisodeCount != 1 {
		t.Errorf("seriály %+v", shows)
	}
}
