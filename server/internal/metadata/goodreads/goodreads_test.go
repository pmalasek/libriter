package goodreads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"libriter/internal/metadata"
)

// Výřez stránky hledání – dva řádky, první je studijní příručka.
const searchHTML = `<html><body><table class="tableList">
<tr itemscope itemtype="http://schema.org/Book">
  <td><a class="bookTitle" itemprop="url" href="/book/show/58007522-summary-of-project-hail-mary?from_search=true&amp;rank=1">
    <span itemprop='name'>Summary of Project Hail Mary</span></a>
  <span class='by'>by</span>
  <span itemprop='author'><div class='authorName__container'>
    <a class="authorName" href="https://www.goodreads.com/author/show/1.Someone"><span itemprop="name">Someone Else</span></a>
  </div></span>
  <div><span class="greyText smallText uitext">3.10 avg rating &mdash; published 2021 &mdash;</span></div></td>
</tr>
<tr itemscope itemtype="http://schema.org/Book">
  <td><a class="bookTitle" itemprop="url" href="/book/show/54493401-project-hail-mary?from_search=true&amp;rank=2">
    <span itemprop='name'>Project Hail Mary</span></a>
  <span class='by'>by</span>
  <span itemprop='author'><div class='authorName__container'>
    <a class="authorName" href="https://www.goodreads.com/author/show/6540057.Andy_Weir"><span itemprop="name">Andy Weir</span></a>
    <span class="greyText">(Goodreads Author)</span>
  </div></span>
  <div><span class="greyText smallText uitext">4.51 avg rating &mdash; 1,883,817 ratings &mdash;
     published
     2021 &mdash; <a class="greyText" href="/work/editions/79106958">5 editions</a></span></div></td>
</tr>
</table></body></html>`

// Výřez nového vzhledu hledání (web ho posílá s cookies relace). Údaje první
// knihy jsou na místě, druhé ve streamovaném skrytém bloku na konci.
const searchItemsHTML = `<html><body><ul class="Books" data-testid="book-list-item">
<li><div><div class="Book" data-testid="book-item-kca://book/amzn1.gr.book.v1.tJT">
 <div class="BookCard"><a href="/book/show/3?ref=s_s" class="BookCard__stretchedLink" aria-label="Harry Potter"></a></div>
 <div class="Book__details"><div class="Book__content" data-testid="book-item-content">
  <span data-testid="book-item-series"><a href="/series/45175-harry-potter">Harry Potter (Series #1)</a></span>
  <span class="Text Text__title3" data-testid="book-item-title"><a href="/book/show/3">Harry Potter and the Sorcerer&#x27;s Stone</a></span>
  <span data-testid="book-item-contributors"><a class="ContributorLink" href="https://www.goodreads.com/author/show/1077326.J_K_Rowling"><span class="ContributorLink__name" data-testid="name">J.K. Rowling</span></a></span>
  <div data-testid="book-item-publication"><span data-testid="book-item-publication-year">Published 2003</span>
   <a href="/work/editions/4640799"><span data-testid="book-item-editions-count">121 Editions</span></a></div>
 </div></div>
</div></div></li>
<li><div><div class="Book" data-testid="book-item-kca://book/amzn1.gr.book.v3.yMd">
 <div class="BookCard"><a href="/book/show/239930331?ref=s_s" aria-label="Harry Potter"></a></div>
 <template id="P:c"></template></div></div></li>
</ul>
<div hidden id="S:c"><div class="Book__details"><div class="Book__content" data-testid="book-item-content">
  <span data-testid="book-item-title"><a href="/book/show/239930331">Harry Potter and the Sorcerer’s Stone</a></span>
  <span data-testid="book-item-contributors"><a class="ContributorLink"><span data-testid="name">J.K. Rowling</span></a>,
   <a class="ContributorLink"><span data-testid="name">Jim Kay</span></a></span>
  <div data-testid="book-item-publication"><span data-testid="book-item-publication-year">Published 2026</span></div>
</div></div></div>
</body></html>`

// Výřez stránky knihy: Apollo cache s hledanou knihou, jiným vydáním,
// autorem, sérií a dílem.
const bookHTML = `<html><head>
<script type="application/ld+json">{"@type":"Book","name":"Harry Potter and the Sorcerer&#39;s Stone"}</script>
</head><body>
<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"apolloState":{
 "Book:jine-vydani":{"legacyId":42844155,"title":"Jiné vydání"},
 "Book:hledana":{"legacyId":3,"title":"Harry Potter and the Sorcerer's Stone",
   "description":"Harry Potter has never even heard of Hogwarts.<br /><br />Then a letter arrives.",
   "imageUrl":"https://images.gr-assets.com/books/3.jpg",
   "details":{"publisher":"Scholastic Inc","publicationTime":1067673600000,"language":{"name":"English"}},
   "bookGenres":[{"genre":{"name":"Fantasy"}},{"genre":{"name":"Fiction"}}],
   "bookSeries":[{"userPosition":"1","series":{"__ref":"Series:hp"}}],
   "primaryContributorEdge":{"node":{"__ref":"Contributor:rowling"},"role":"Author"},
   "secondaryContributorEdges":[
     {"node":{"__ref":"Contributor:dale"},"role":"Narrator"},
     {"node":{"__ref":"Contributor:grandpre"},"role":"Illustrator"}],
   "work":{"__ref":"Work:hp1"}},
 "Contributor:rowling":{"name":"J.K. Rowling"},
 "Contributor:dale":{"name":"Jim Dale"},
 "Contributor:grandpre":{"name":"Mary GrandPré"},
 "Series:hp":{"title":"Harry Potter"},
 "Work:hp1":{"details":{"publicationTime":867308400000,"originalTitle":"Harry Potter and the Philosopher’s Stone"},
   "stats":{"averageRating":4.47}}
}}}}</script>
</body></html>`

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := NewClient()
	c.baseURL = srv.URL
	c.fetcher = metadata.NewFetcher(0) // v testu nečekat 3 s
	return c
}

func TestSearch(t *testing.T) {
	var gotQuery string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(searchHTML))
	})

	results, err := c.Search(context.Background(), metadata.SearchQuery{Title: "Project Hail Mary", Author: "Andy Weir"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Autor se do dotazu neposílá – vytáhl by nahoru příručky.
	if strings.Contains(gotQuery, "Weir") {
		t.Errorf("dotaz obsahuje autora: %s", gotQuery)
	}

	want := []metadata.SearchResult{
		{Title: "Project Hail Mary", Author: "Andy Weir", Year: 2021, URL: c.baseURL + "/book/show/54493401", Source: "goodreads"},
		{Title: "Summary of Project Hail Mary", Author: "Someone Else", Year: 2021, URL: c.baseURL + "/book/show/58007522", Source: "goodreads"},
	}
	if !reflect.DeepEqual(results, want) {
		t.Errorf("výsledky = %+v\nchtěno %+v", results, want)
	}
}

func TestSearchNewLayout(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(searchItemsHTML))
	})

	results, err := c.Search(context.Background(), metadata.SearchQuery{Title: "Harry Potter"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	want := []metadata.SearchResult{
		{Title: "Harry Potter and the Sorcerer's Stone", Author: "J.K. Rowling", Year: 2003, URL: c.baseURL + "/book/show/3", Source: "goodreads"},
		{Title: "Harry Potter and the Sorcerer’s Stone", Author: "J.K. Rowling, Jim Kay", Year: 2026, URL: c.baseURL + "/book/show/239930331", Source: "goodreads"},
	}
	if !reflect.DeepEqual(results, want) {
		t.Errorf("výsledky = %+v\nchtěno %+v", results, want)
	}
}

func TestFetchByURL(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/book/show/3" {
			t.Errorf("cesta = %s", r.URL.Path)
		}
		w.Write([]byte(bookHTML))
	})

	meta, err := c.FetchByURL(context.Background(), "https://www.goodreads.com/book/show/3.Harry_Potter_and_the_Sorcerer_s_Stone")
	if err != nil {
		t.Fatalf("FetchByURL: %v", err)
	}

	want := &metadata.BookMetadata{
		Title:          "Harry Potter and the Sorcerer's Stone",
		Author:         "J.K. Rowling",
		Narrator:       "Jim Dale",
		Description:    "Harry Potter has never even heard of Hogwarts.\n\nThen a letter arrives.",
		Genres:         []string{"Fantasy", "Fiction"},
		CoverURL:       "https://images.gr-assets.com/books/3.jpg",
		Rating:         89,
		Publisher:      "Scholastic Inc",
		Year:           1997, // rok prvního vydání díla, ne tohoto vydání (2003)
		OriginalTitle:  "Harry Potter and the Philosopher’s Stone",
		Series:         "Harry Potter",
		SeriesPosition: 1,
		Language:       "en",
		SourceURL:      c.baseURL + "/book/show/3",
		Source:         "goodreads",
	}
	if !reflect.DeepEqual(meta, want) {
		t.Errorf("metadata = %+v\nchtěno %+v", meta, want)
	}
}

// Bez Next.js dat se použije JSON-LD.
func TestFetchByURLFallsBackToJSONLD(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><script type="application/ld+json">
{"@type":"Book","name":"Dune","image":"https://img/dune.jpg","inLanguage":"English",
 "author":[{"@type":"Person","name":"Frank Herbert"}],"aggregateRating":{"ratingValue":4.29}}
</script></head><body></body></html>`))
	})

	meta, err := c.FetchByURL(context.Background(), "https://www.goodreads.com/book/show/44767458-dune")
	if err != nil {
		t.Fatalf("FetchByURL: %v", err)
	}
	if meta.Title != "Dune" || meta.Author != "Frank Herbert" || meta.Rating != 86 || meta.Language != "en" {
		t.Errorf("metadata = %+v", meta)
	}
}

func TestSupports(t *testing.T) {
	c := NewClient()
	tests := map[string]bool{
		"https://www.goodreads.com/book/show/54493401-project-hail-mary":        true,
		"https://www.goodreads.com/book/show/3.Harry_Potter_and_the_Sorcerer_s": true,
		"https://goodreads.com/book/show/3":                                     true,
		"https://www.goodreads.com/author/show/6540057.Andy_Weir":               false,
		"https://evil.example/book/show/3":                                      false,
	}
	for u, want := range tests {
		if got := c.Supports(u); got != want {
			t.Errorf("Supports(%s) = %v, chtěno %v", u, got, want)
		}
	}
}

// Ochrana proti robotům vrací 202 s výzvou – editor má dostat srozumitelnou
// hlášku, ne „HTTP 202“.
func TestSearchBlockedByWAF(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	_, err := c.Search(context.Background(), metadata.SearchQuery{Title: "Dune"})
	if err == nil || !strings.Contains(err.Error(), "ochrana proti robotům") {
		t.Errorf("chyba = %v, chtěna hláška o ochraně proti robotům", err)
	}
}
