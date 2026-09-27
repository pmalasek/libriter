package audible

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"libriter/internal/metadata"
)

// Výřezy skutečných odpovědí katalogu (audible.com, audible.de).
const searchJSON = `{"products":[
 {"asin":"B08G9PRS1K","title":"Project Hail Mary","authors":[{"asin":"B00G0WYW92","name":"Andy Weir"}],
  "language":"english","release_date":"2021-05-04"},
 {"asin":"B0ITALIAN1","title":"Project Hail Mary (Italian edition)","authors":[{"name":"Andy Weir"}],
  "language":"italian","release_date":"2021-10-01"},
 {"asin":"B0OTHER001","title":"Project Hail Mary: Summary","authors":[{"name":"Someone Else"}],
  "language":"english","release_date":"2021-06-01"}
],"total_results":3}`

const detailJSON = `{"product":{
 "asin":"B017WRJTPA","title":"Harry Potter und der Stein der Weisen","subtitle":"Harry Potter 1",
 "authors":[{"asin":"B000AP9A6K","name":"J.K. Rowling"},{"name":"Klaus Fritz - Übersetzer"}],
 "narrators":[{"name":"Rufus Beck"}],
 "language":"german","publisher_name":"Der Hörverlag","release_date":"2015-11-20",
 "publisher_summary":"<p>Bis zu seinem elften Geburtstag glaubt Harry,</p><p>er sei ein ganz normaler Junge.</p>",
 "product_images":{"500":"https://m.media-amazon.com/images/I/small.jpg","1215":"https://m.media-amazon.com/images/I/big.jpg"},
 "series":[{"asin":"B0182K6YIG","sequence":"1","title":"Harry Potter","url":"/pd/Harry-Potter-Hoerbuch/B0182K6YIG"}],
 "rating":{"overall_distribution":{"average_rating":4.87}},
 "category_ladders":[
  {"ladder":[{"id":"1","name":"Kinder & Jugendliche"},{"id":"2","name":"Fantasy"}],"root":"Genres"},
  {"ladder":[{"id":"3","name":"Science Fiction & Fantasy"},{"id":"2","name":"Fantasy"}],"root":"Genres"}
 ]
}}`

func newTestClient(t *testing.T, market Marketplace, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := NewClient(market)
	c.apiBase = srv.URL
	return c
}

func TestSearchFiltersLanguageAndRanksAuthor(t *testing.T) {
	var gotQuery string
	c := newTestClient(t, MarketplaceCOM, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.URL.Path != "/1.0/catalog/products" {
			t.Errorf("cesta = %s", r.URL.Path)
		}
		if got := r.Header.Get("Accept-Language"); !strings.HasPrefix(got, "en") {
			t.Errorf("Accept-Language = %q, chtěna angličtina", got)
		}
		w.Write([]byte(searchJSON))
	})

	results, err := c.Search(context.Background(), metadata.SearchQuery{
		Title: "Project Hail Mary", Author: "Andy Weir", Language: "en",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(gotQuery, "title=Project+Hail+Mary") || !strings.Contains(gotQuery, "author=Andy+Weir") {
		t.Errorf("dotaz = %s", gotQuery)
	}

	want := []metadata.SearchResult{
		{Title: "Project Hail Mary", Author: "Andy Weir", Year: 2021, URL: "https://www.audible.com/pd/B08G9PRS1K", Source: "audible_com"},
		{Title: "Project Hail Mary: Summary", Author: "Someone Else", Year: 2021, URL: "https://www.audible.com/pd/B0OTHER001", Source: "audible_com"},
	}
	if !reflect.DeepEqual(results, want) {
		t.Errorf("výsledky = %+v\nchtěno %+v", results, want)
	}
}

// Bez jazyka se nefiltruje – editor hledá napříč vydáními.
func TestSearchWithoutLanguageKeepsAll(t *testing.T) {
	c := newTestClient(t, MarketplaceCOM, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(searchJSON))
	})
	results, err := c.Search(context.Background(), metadata.SearchQuery{Title: "Project Hail Mary"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("výsledků = %d, chtěny 3", len(results))
	}
}

func TestFetchByURL(t *testing.T) {
	c := newTestClient(t, MarketplaceDE, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/1.0/catalog/products/B017WRJTPA" {
			t.Errorf("cesta = %s", r.URL.Path)
		}
		w.Write([]byte(detailJSON))
	})

	meta, err := c.FetchByURL(context.Background(),
		"https://www.audible.de/pd/Harry-Potter-und-der-Stein-der-Weisen-Hoerbuch/B017WRJTPA")
	if err != nil {
		t.Fatalf("FetchByURL: %v", err)
	}

	want := &metadata.BookMetadata{
		// Podtitul („Harry Potter 1“) je u Audible obvykle jen série – nepřebírá se.
		Title:          "Harry Potter und der Stein der Weisen",
		Author:         "J.K. Rowling",
		Narrator:       "Rufus Beck",
		Description:    "Bis zu seinem elften Geburtstag glaubt Harry,\n\ner sei ein ganz normaler Junge.",
		Genres:         []string{"Kinder & Jugendliche", "Fantasy", "Science Fiction & Fantasy"},
		CoverURL:       "https://m.media-amazon.com/images/I/big.jpg",
		Rating:         97,
		Publisher:      "Der Hörverlag",
		Year:           2015,
		Series:         "Harry Potter",
		SeriesPosition: 1,
		Language:       "de",
		SourceURL:      "https://www.audible.de/pd/B017WRJTPA",
		Source:         "audible_de",
	}
	if !reflect.DeepEqual(meta, want) {
		t.Errorf("metadata = %+v\nchtěno %+v", meta, want)
	}
}

func TestSupports(t *testing.T) {
	de := NewClient(MarketplaceDE)
	tests := map[string]bool{
		"https://www.audible.de/pd/Der-Schwarm-Hoerbuch/B004V09PMY": true,
		"https://audible.de/pd/B004V09PMY":                          true,
		"https://www.audible.com/pd/Project-Hail-Mary/B08G9PRS1K":   false, // jiný marketplace
		"https://www.audible.de/search?keywords=schwarm":            false,
		"https://evil.example/pd/B004V09PMY":                        false,
	}
	for u, want := range tests {
		if got := de.Supports(u); got != want {
			t.Errorf("Supports(%s) = %v, chtěno %v", u, got, want)
		}
	}
}
