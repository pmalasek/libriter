package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeTMDB(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path+"?"+r.URL.RawQuery)
		if !strings.HasPrefix(r.URL.Path, "/w") && r.URL.Query().Get("api_key") != "k" && r.Header.Get("Authorization") != "Bearer eyJtoken" {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		lang := r.URL.Query().Get("language")
		switch {
		case r.URL.Path == "/search/movie":
			_, _ = w.Write([]byte(`{"results":[{"id":1934,"title":"Pelíšky","original_title":"Pelíšky","release_date":"1999-02-18","overview":"Vánoce 1967","poster_path":"/p.jpg","popularity":12.5}]}`))
		case r.URL.Path == "/search/tv":
			_, _ = w.Write([]byte(`{"results":[{"id":77,"name":"Docent","original_name":"Docent","first_air_date":"2022-09-01"}]}`))
		case r.URL.Path == "/movie/1934" && lang == "cs-CZ":
			_, _ = w.Write([]byte(`{"id":1934,"imdb_id":"tt0167116","title":"Pelíšky","original_title":"Pelíšky","overview":"","release_date":"1999-02-18","runtime":115,"vote_average":8.1,"poster_path":"/p.jpg","backdrop_path":"/b.jpg","genres":[{"id":35,"name":"Komedie"}],"credits":{"cast":[{"name":"Miroslav Donutil","character":"Šebek","order":0}],"crew":[{"name":"Jan Hřebejk","job":"Director"},{"name":"Petr Šabach","job":"Writer"}]}}`))
		case r.URL.Path == "/movie/1934" && lang == "en-US":
			_, _ = w.Write([]byte(`{"id":1934,"overview":"Christmas 1967","tagline":"Cosy Dens"}`))
		case r.URL.Path == "/tv/77/season/3":
			_, _ = w.Write([]byte(`{"season_number":3,"name":"Řada 3","episodes":[{"episode_number":1,"name":"Únos","air_date":"2024-09-01","runtime":58}]}`))
		case r.URL.Path == "/w500/p.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("JPEG"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestClient(t *testing.T) {
	srv, calls := fakeTMDB(t)
	c := New("k", "cs-CZ").WithBaseURL(srv.URL, srv.URL)
	ctx := context.Background()

	res, err := c.SearchMovie(ctx, "Pelíšky", 1999)
	if err != nil || len(res) != 1 || res[0].ID != 1934 || res[0].Year != 1999 {
		t.Fatalf("SearchMovie = %+v, %v", res, err)
	}
	if !strings.Contains((*calls)[0], "year=1999") {
		t.Errorf("chybí rok v dotazu: %s", (*calls)[0])
	}

	d, err := c.Movie(ctx, 1934)
	if err != nil {
		t.Fatal(err)
	}
	if d.Overview != "Christmas 1967" || d.Tagline != "Cosy Dens" {
		t.Errorf("anglický záložní popis: %q %q", d.Overview, d.Tagline)
	}
	if d.RuntimeMin != 115 || d.Year != 1999 || len(d.Genres) != 1 || d.Directors[0].Name != "Jan Hřebejk" || len(d.Cast) != 1 {
		t.Errorf("Movie = %+v", d)
	}

	tv, err := c.SearchTV(ctx, "Docent", 0)
	if err != nil || len(tv) != 1 || tv[0].Title != "Docent" || tv[0].Year != 2022 {
		t.Fatalf("SearchTV = %+v, %v", tv, err)
	}

	s, err := c.Season(ctx, 77, 3)
	if err != nil || len(s.Episodes) != 1 || s.Episodes[0].Name != "Únos" {
		t.Fatalf("Season = %+v, %v", s, err)
	}

	img, _, err := c.Image(ctx, SizePoster, "/p.jpg")
	if err != nil || string(img) != "JPEG" {
		t.Fatalf("Image = %q, %v", img, err)
	}

	if _, err := c.Movie(ctx, 999); err != ErrNotFound {
		t.Errorf("neexistující film: %v", err)
	}
}

func TestClientBearerAndNoKey(t *testing.T) {
	srv, _ := fakeTMDB(t)
	c := New("eyJtoken", "en-US").WithBaseURL(srv.URL, srv.URL)
	if _, err := c.SearchMovie(context.Background(), "x", 0); err != nil {
		t.Fatalf("v4 token: %v", err)
	}
	if _, err := New("", "").SearchMovie(context.Background(), "x", 0); err != ErrNoKey {
		t.Fatalf("bez klíče: %v", err)
	}
}
