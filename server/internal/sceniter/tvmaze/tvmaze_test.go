package tvmaze

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/shows":
			if r.URL.Query().Get("q") != "the office" {
				t.Errorf("dotaz %q", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"score":1,"show":{"id":526,"url":"https://www.tvmaze.com/shows/526/the-office","name":"The Office","premiered":"2005-03-24","genres":["Comedy"],"image":{"medium":"https://static.tvmaze.com/m.jpg","original":"https://static.tvmaze.com/o.jpg"}}}]`))
		case "/shows/526":
			_, _ = w.Write([]byte(`{"id":526,"url":"https://www.tvmaze.com/shows/526/the-office","name":"The Office","premiered":"2005-03-24",
				"status":"Ended","averageRuntime":30,"genres":["Comedy"],"rating":{"average":8.5},
				"externals":{"imdb":"tt0386676","thetvdb":73244},"image":{"original":"https://static.tvmaze.com/o.jpg"},
				"summary":"<p>Steve Carell stars in <b>The Office</b>.</p>",
				"_embedded":{"cast":[{"person":{"id":11,"name":"Rainn Wilson","image":{"medium":"https://static.tvmaze.com/p.jpg"}},"character":{"name":"Dwight"}}],
				"crew":[{"type":"Creator","person":{"name":"Greg Daniels"}},{"type":"Producer","person":{"name":"X"}}],
				"seasons":[{"id":2087,"number":1},{"id":2088,"number":2}]}}`))
		case "/shows/526/seasons":
			_, _ = w.Write([]byte(`[{"id":2087,"number":1,"name":"","summary":"<p>First.</p>"}]`))
		case "/seasons/2087/episodes":
			_, _ = w.Write([]byte(`[{"number":1,"name":"Pilot","airdate":"2005-03-24","runtime":23,"summary":"<p>A.</p>","image":{"medium":"https://static.tvmaze.com/e.jpg"}},{"number":null,"name":"Special"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New().WithBaseURL(srv.URL)
	ctx := context.Background()

	res, err := c.Search(ctx, "the office")
	if err != nil || len(res) != 1 || res[0].ID != "526" || res[0].Year != 2005 || res[0].ImageURL != "https://static.tvmaze.com/m.jpg" {
		t.Fatalf("hledání %+v %v", res, err)
	}

	d, err := c.Show(ctx, 526)
	if err != nil {
		t.Fatal(err)
	}
	if d.Overview != "Steve Carell stars in The Office." || d.Rating != 85 || d.RuntimeMin != 30 ||
		d.ExternalIDs["imdb"] != "tt0386676" || d.ExternalIDs["thetvdb"] != "73244" {
		t.Errorf("detail %+v", d)
	}
	if len(d.Credits.Creators) != 1 || d.Credits.Cast[0].Character != "Dwight" || len(d.Seasons) != 2 ||
		d.Credits.Cast[0].ID != "11" || d.Credits.Cast[0].ImageURL != "https://static.tvmaze.com/p.jpg" {
		t.Errorf("tvůrci/řady %+v %v", d.Credits, d.Seasons)
	}

	s, err := c.Season(ctx, 526, 1)
	if err != nil || len(s.Episodes) != 1 || s.Episodes[0].Title != "Pilot" || s.Overview != "First." {
		t.Fatalf("řada %+v %v", s, err)
	}
	if _, err := c.Season(ctx, 526, 7); err != ErrNotFound {
		t.Fatalf("chybějící řada: %v", err)
	}
	if _, err := c.Show(ctx, 1); err != ErrNotFound {
		t.Fatalf("chybějící seriál: %v", err)
	}
}
