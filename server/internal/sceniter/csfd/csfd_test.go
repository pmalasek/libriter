package csfd

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"mediater/internal/core/plugin"
)

// Testovací stránky jsou skutečné stránky ČSFD (říjen 2026) bez skriptů a stylů.
func fixture(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func parse(t *testing.T, name string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(fixture(t, name)))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestParseSearchMovies(t *testing.T) {
	res := ParseSearch(parse(t, "search-pelisky.html"), plugin.KindMovie, "https://www.csfd.cz")
	if len(res) < 2 {
		t.Fatalf("výsledky %+v", res)
	}
	r := res[0]
	if r.ID != "4570" || r.Title != "Pelíšky" || r.Year != 1999 || !strings.HasPrefix(r.ImageURL, "https://image.pmgstatic.com/") {
		t.Errorf("první výsledek %+v", r)
	}
	if r.URL != "https://www.csfd.cz/film/4570-pelisky/prehled/" || !strings.Contains(r.Note, "Komedie") {
		t.Errorf("adresa/poznámka %+v", r)
	}
	if res[1].ImageURL != "" {
		t.Errorf("výsledek bez plakátu má mít prázdný obrázek: %q", res[1].ImageURL)
	}
}

func TestParseSearchShowsSkipsSeasons(t *testing.T) {
	res := ParseSearch(parse(t, "search-docent.html"), plugin.KindShow, "https://www.csfd.cz")
	if len(res) == 0 || res[0].ID != "1024280" || res[0].Title != "Docent" || res[0].Year != 2023 {
		t.Fatalf("výsledky %+v", res)
	}
	for _, r := range res {
		if strings.Contains(r.Title, "Série") {
			t.Errorf("řada mezi výsledky: %+v", r)
		}
	}
}

func TestParseFilm(t *testing.T) {
	d := ParseFilm(parse(t, "film-pelisky.html"))
	if d.Title != "Pelíšky" || d.Year != 1999 || d.RuntimeMin != 116 || d.Rating != 91 {
		t.Errorf("základ %+v", d)
	}
	if d.OriginalTitle != "" {
		t.Errorf("český film nemá původní název, je %q", d.OriginalTitle)
	}
	if strings.Join(d.Genres, ",") != "Komedie,Drama" {
		t.Errorf("žánry %v", d.Genres)
	}
	if !strings.HasPrefix(d.Overview, "Příběhy jedné historické generace") || strings.Contains(d.Overview, "oficiální text") {
		t.Errorf("děj %q", d.Overview)
	}
	if len(d.Credits.Directors) != 1 || d.Credits.Directors[0].Name != "Jan Hřebejk" ||
		len(d.Credits.Writers) != 1 || d.Credits.Writers[0].Name != "Petr Jarchovský" ||
		len(d.Credits.Cast) == 0 || d.Credits.Cast[0].Name != "Miroslav Donutil" {
		t.Errorf("tvůrci %+v", d.Credits)
	}
	// ID tvůrců z odkazů /tvurce/<id>-…
	if d.Credits.Directors[0].ID != "3195" || d.Credits.Writers[0].ID != "72212" || d.Credits.Cast[0].ID != "949" {
		t.Errorf("ID tvůrců %+v", d.Credits)
	}
	if !strings.HasPrefix(d.PosterURL, "https://image.pmgstatic.com/") || len(d.Seasons) != 0 {
		t.Errorf("plakát %q řady %v", d.PosterURL, d.Seasons)
	}

	m := ParseFilm(parse(t, "film-matrix.html"))
	if m.Title != "Matrix" || m.OriginalTitle != "The Matrix" || m.Year != 1999 {
		t.Errorf("Matrix %+v", m)
	}
}

func TestParseShowSeasons(t *testing.T) {
	d := ParseFilm(parse(t, "show-docent.html"))
	if d.Title != "Docent" || len(d.Seasons) != 3 || d.Seasons[0] != 1 || d.Seasons[2] != 3 {
		t.Errorf("seriál %+v", d)
	}
	if !strings.Contains(d.BackdropURL, "/cache/resized/w1326/files/images/film/photos/") {
		t.Errorf("pozadí z galerie: %q", d.BackdropURL)
	}
}

func TestSeason(t *testing.T) {
	pages := map[string]string{
		"/film/1024280/prehled/":                             "show-docent.html",
		"/film/1024280-docent/1239640-serie-1/prehled/":      "season-docent-1.html",
		"/film/1024280-docent/1239641-lesni-vrazda/prehled/": "episode-docent-1-1.html",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != userAgent {
			t.Errorf("User-Agent %q", r.Header.Get("User-Agent"))
		}
		name, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(fixture(t, name)))
	}))
	defer srv.Close()

	c := New().WithBaseURL(srv.URL)
	s, err := c.Season(context.Background(), "1024280", 1)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Série 1" || len(s.Episodes) != 3 || s.Episodes[0].Number != 1 || s.Episodes[0].Title != "Lesní vražda" {
		t.Fatalf("řada %+v", s)
	}
	// Popis a obrázek dílu jsou z jeho stránky; díl bez stránky zůstane jen s názvem.
	e1 := s.Episodes[0]
	if !strings.HasPrefix(e1.Overview, "Docent Stehlík") || !strings.Contains(e1.StillURL, "/cache/resized/w663/files/images/film/photos/") {
		t.Errorf("díl 1: %+v", e1)
	}
	if s.Episodes[1].Overview != "" || s.Episodes[1].StillURL != "" {
		t.Errorf("díl 2 bez stránky: %+v", s.Episodes[1])
	}
	if _, err := c.Season(context.Background(), "1024280", 9); err != ErrNotFound {
		t.Fatalf("neexistující řada: %v", err)
	}
	if _, err := c.Film(context.Background(), "1"); err != ErrNotFound {
		t.Fatalf("neexistující film: %v", err)
	}
}

func TestIDFromURL(t *testing.T) {
	for raw, want := range map[string]string{
		"https://www.csfd.cz/film/4570-pelisky/prehled/":                   "4570",
		"https://www.csfd.cz/film/4570-pelisky/":                           "4570",
		"https://www.csfd.cz/film/1024280-docent/1239640-serie-1/prehled/": "1024280",
	} {
		if got, ok := IDFromURL(raw); !ok || got != want {
			t.Errorf("%s = %q, chci %q", raw, got, want)
		}
	}
}

// Minisérie vedená na ČSFD jako „TV film“ se u seriálu najde taky – za
// výsledky ze sekce seriálů.
func TestParseSearchShowsIncludesTVFilms(t *testing.T) {
	res := ParseSearch(parse(t, "search-bitva.html"), plugin.KindShow, "https://www.csfd.cz")
	var found *plugin.SearchResult
	for i := range res {
		if res[i].Title == "Bitva o nebe nad Británií" {
			found = &res[i]
		}
	}
	if found == nil || found.Year != 2005 || !strings.Contains(found.Note, "TV film") {
		t.Fatalf("TV film mezi výsledky chybí: %+v", res)
	}
	if res[0].Title == found.Title {
		t.Error("seriály mají být před filmy")
	}
}
