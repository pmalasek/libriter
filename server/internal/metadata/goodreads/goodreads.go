// Package goodreads čte metadata z goodreads.com.
//
// Goodreads veřejné API nemá (zrušili ho v roce 2020), takže jde o scraper
// se stejnými pravidly jako u českých webů: identifikující User-Agent,
// prodleva mezi požadavky a dotazy jen na vyžádání. Web občas roboty
// odmítne (WAF) – proto má v pořadí přednost Audible a tenhle zdroj je záloha.
//
// Struktura stránky, na které stojíme:
//   - hledání: /search?q=<název>&search_type=books. Web přechází na nový
//     vzhled a podle cookies posílá jeden, nebo druhý. Nový (React se
//     streamováním) má údaje každé knihy v bloku data-testid="book-item-content"
//     – u první knihy na místě, u dalších ve skrytých <div hidden id="S:…">
//     na konci stránky. Starý má řádky <tr itemtype="http://schema.org/Book">
//     s a.bookTitle a a.authorName.
//   - detail:  /book/show/<id>; data jsou v <script id="__NEXT_DATA__">
//     (Apollo cache Next.js), záložně v JSON-LD Book
package goodreads

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"libriter/internal/metadata"
	"libriter/internal/metadata/htmlutil"
	"libriter/internal/model"

	"golang.org/x/net/html"
)

const (
	providerName = "goodreads"
	host         = "goodreads.com"
	minDelay     = 3 * time.Second
	searchLimit  = 10
)

var (
	bookPathRe  = regexp.MustCompile(`^/book/show/(\d+)`)
	publishedRe = regexp.MustCompile(`(?i)published\s+(\d{4})`)
	sequenceRe  = regexp.MustCompile(`\d+`)
)

type Client struct {
	fetcher *metadata.Fetcher
	// baseURL jde v testech přesměrovat na httptest server.
	baseURL string
}

func NewClient() *Client {
	return &Client{
		fetcher: metadata.NewFetcher(minDelay).WithAcceptLanguage("en-US,en;q=0.9").WithCookies(),
		baseURL: "https://www.goodreads.com",
	}
}

func (c *Client) Name() string { return providerName }

// SupportsCoverURL: obálky jsou na CDN Amazonu, u starších knih na gr-assets.
func (c *Client) SupportsCoverURL(rawURL string) bool {
	for _, h := range []string{"m.media-amazon.com", "i.gr-assets.com", "images.gr-assets.com", "s.gr-assets.com"} {
		if metadata.HostMatches(rawURL, h) {
			return true
		}
	}
	return false
}

func (c *Client) Supports(rawURL string) bool {
	return metadata.HostMatches(rawURL, host) && bookID(rawURL) != 0
}

// bookID vytáhne číselné ID knihy z adresy /book/show/<id>[-slug|.Slug].
func bookID(rawURL string) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0
	}
	m := bookPathRe.FindStringSubmatch(u.Path)
	if m == nil {
		return 0
	}
	id, _ := strconv.Atoi(m[1])
	return id
}

func (c *Client) fetch(ctx context.Context, targetURL string) (*html.Node, error) {
	body, err := c.fetcher.Get(ctx, targetURL, "text/html,application/xhtml+xml")
	if err != nil {
		return nil, describe(err)
	}
	defer body.Close()

	return html.Parse(io.LimitReader(body, 8<<20))
}

// describe převede odpověď ochrany proti robotům na srozumitelnou hlášku.
// AWS WAF místo stránky vrací 202 s JavaScriptovou výzvou, případně 403
// nebo 429; bez vysvětlení by editor viděl jen „HTTP 202“.
func describe(err error) error {
	switch metadata.StatusOf(err) {
	case http.StatusAccepted, http.StatusForbidden, http.StatusTooManyRequests:
		return fmt.Errorf("Goodreads dočasně odmítá automatický přístup (ochrana proti robotům), zkuste to později: %w", err)
	default:
		return err
	}
}

// --- vyhledávání ---

// Search hledá podle názvu. Jméno autora v dotazu by vytáhlo nahoru
// „Summary of …“ a studijní příručky, proto se autor použije až na seřazení.
// Jazyk vydání výsledky hledání neuvádějí – filtruje se jen profilem.
func (c *Client) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.SearchResult, error) {
	title := strings.TrimSpace(q.Title)
	if title == "" {
		return nil, nil
	}
	params := url.Values{}
	params.Set("q", title)
	params.Set("search_type", "books")

	doc, err := c.fetch(ctx, c.baseURL+"/search?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return metadata.RankByAuthor(c.parseSearchResults(doc), q.Author), nil
}

func (c *Client) parseSearchResults(doc *html.Node) []metadata.SearchResult {
	var results []metadata.SearchResult

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode {
			return true
		}

		var r metadata.SearchResult
		switch {
		case htmlutil.Attr(n, "data-testid") == "book-item-content":
			r = c.parseItem(n)
		case n.Data == "tr" && htmlutil.Attr(n, "itemtype") == "http://schema.org/Book":
			r = c.parseLegacyRow(n)
		default:
			return true
		}

		if r.URL != "" && r.Title != "" {
			r.Source = providerName
			results = append(results, r)
		}
		return false
	})
	return results
}

// parseItem čte položku nového vzhledu hledání.
func (c *Client) parseItem(n *html.Node) metadata.SearchResult {
	var r metadata.SearchResult
	var authors []string
	htmlutil.Walk(n, func(m *html.Node) bool {
		if m.Type != html.ElementNode {
			return true
		}
		switch htmlutil.Attr(m, "data-testid") {
		case "book-item-title":
			if a := htmlutil.Element(m, "a"); a != nil {
				r.URL = c.bookURL(htmlutil.Attr(a, "href"))
			}
			r.Title = htmlutil.Collapse(htmlutil.Text(m))
			return false
		case "name":
			if name := htmlutil.Collapse(htmlutil.Text(m)); name != "" {
				authors = append(authors, name)
			}
			return false
		case "book-item-publication-year":
			if y := publishedRe.FindStringSubmatch(htmlutil.Text(m)); y != nil {
				r.Year, _ = strconv.Atoi(y[1])
			}
			return false
		}
		return true
	})
	r.Author = strings.Join(authors, ", ")
	return r
}

// parseLegacyRow čte řádek starého vzhledu hledání.
func (c *Client) parseLegacyRow(n *html.Node) metadata.SearchResult {
	var r metadata.SearchResult
	var authors []string
	htmlutil.Walk(n, func(m *html.Node) bool {
		if m.Type != html.ElementNode || m.Data != "a" {
			return true
		}
		switch {
		case htmlutil.HasClass(m, "bookTitle"):
			r.URL = c.bookURL(htmlutil.Attr(m, "href"))
			r.Title = htmlutil.Collapse(htmlutil.Text(m))
			return false
		case htmlutil.HasClass(m, "authorName"):
			if name := htmlutil.Collapse(htmlutil.Text(m)); name != "" {
				authors = append(authors, name)
			}
			return false
		}
		return true
	})
	if y := publishedRe.FindStringSubmatch(htmlutil.Collapse(htmlutil.Text(n))); y != nil {
		r.Year, _ = strconv.Atoi(y[1])
	}
	r.Author = strings.Join(authors, ", ")
	return r
}

// bookURL převede odkaz z výsledku na kanonickou adresu knihy bez parametrů
// hledání; odkaz, který na knihu nevede, vrátí prázdný.
func (c *Client) bookURL(href string) string {
	if id := bookID(htmlutil.ResolveURL(c.baseURL, href)); id != 0 {
		return fmt.Sprintf("%s/book/show/%d", c.baseURL, id)
	}
	return ""
}

// --- detail ---

func (c *Client) FetchByURL(ctx context.Context, rawURL string) (*metadata.BookMetadata, error) {
	id := bookID(rawURL)
	if id == 0 {
		return nil, fmt.Errorf("adresa není stránka knihy: %s", rawURL)
	}
	pageURL := fmt.Sprintf("%s/book/show/%d", c.baseURL, id)

	doc, err := c.fetch(ctx, pageURL)
	if err != nil {
		return nil, fmt.Errorf("detail %d: %w", id, err)
	}

	meta := parseNextData(doc, id)
	if meta == nil {
		meta = parseJSONLD(doc)
	}
	if meta == nil || meta.Title == "" {
		return nil, fmt.Errorf("stránka knihy %d nemá čitelná data (změnil se web?)", id)
	}
	meta.SourceURL = pageURL
	meta.Source = providerName
	return meta, nil
}

// ref je odkaz na jiný záznam Apollo cache.
type ref struct {
	Ref string `json:"__ref"`
}

type apolloBook struct {
	LegacyID    int    `json:"legacyId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	Details     struct {
		Publisher       string  `json:"publisher"`
		PublicationTime float64 `json:"publicationTime"` // ms od epochy
		Language        struct {
			Name string `json:"name"`
		} `json:"language"`
	} `json:"details"`
	BookGenres []struct {
		Genre struct {
			Name string `json:"name"`
		} `json:"genre"`
	} `json:"bookGenres"`
	BookSeries []struct {
		UserPosition string `json:"userPosition"`
		Series       ref    `json:"series"`
	} `json:"bookSeries"`
	PrimaryContributorEdge    contributorEdge   `json:"primaryContributorEdge"`
	SecondaryContributorEdges []contributorEdge `json:"secondaryContributorEdges"`
	Work                      ref               `json:"work"`
}

type contributorEdge struct {
	Node ref    `json:"node"`
	Role string `json:"role"`
}

type apolloWork struct {
	Details struct {
		PublicationTime float64 `json:"publicationTime"`
		OriginalTitle   string  `json:"originalTitle"`
	} `json:"details"`
	Stats struct {
		AverageRating float64 `json:"averageRating"` // 1–5
	} `json:"stats"`
}

// parseNextData čte data z Apollo cache stránky. Cache obsahuje i jiné knihy
// (doporučené, jiná vydání), správná je ta s legacyId z adresy.
func parseNextData(doc *html.Node, id int) *metadata.BookMetadata {
	script := htmlutil.ByID(doc, "__NEXT_DATA__")
	if script == nil {
		return nil
	}
	var next struct {
		Props struct {
			PageProps struct {
				ApolloState map[string]json.RawMessage `json:"apolloState"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal([]byte(htmlutil.Text(script)), &next); err != nil {
		return nil
	}
	state := next.Props.PageProps.ApolloState

	var book *apolloBook
	for key, raw := range state {
		if !strings.HasPrefix(key, "Book:") {
			continue
		}
		var b apolloBook
		if json.Unmarshal(raw, &b) == nil && b.LegacyID == id {
			book = &b
			break
		}
	}
	if book == nil {
		return nil
	}

	lookup := func(r ref, v any) bool {
		raw, ok := state[r.Ref]
		return ok && json.Unmarshal(raw, v) == nil
	}
	contributorName := func(e contributorEdge) string {
		var c struct {
			Name string `json:"name"`
		}
		lookup(e.Node, &c)
		return strings.TrimSpace(c.Name)
	}

	meta := &metadata.BookMetadata{
		Title:       strings.TrimSpace(book.Title),
		Description: htmlutil.FragmentText(book.Description),
		CoverURL:    book.ImageURL,
		Publisher:   book.Details.Publisher,
		Year:        yearFromMillis(book.Details.PublicationTime),
		Language:    model.LanguageFromEnglishName(book.Details.Language.Name),
	}

	// Autoři: hlavní a další s rolí autora; vypravěč je u vydání audioknihy
	// mezi dalšími přispěvateli.
	authors := []string{}
	if name := contributorName(book.PrimaryContributorEdge); name != "" {
		authors = append(authors, name)
	}
	var narrators []string
	for _, e := range book.SecondaryContributorEdges {
		name := contributorName(e)
		if name == "" {
			continue
		}
		switch strings.ToLower(e.Role) {
		case "author":
			authors = append(authors, name)
		case "narrator":
			narrators = append(narrators, name)
		}
	}
	meta.Author = strings.Join(authors, ", ")
	meta.Narrator = strings.Join(narrators, ", ")

	for _, g := range book.BookGenres {
		if g.Genre.Name != "" {
			meta.Genres = append(meta.Genres, g.Genre.Name)
		}
	}

	if len(book.BookSeries) > 0 {
		var s struct {
			Title string `json:"title"`
		}
		if lookup(book.BookSeries[0].Series, &s) {
			meta.Series = s.Title
			if m := sequenceRe.FindString(book.BookSeries[0].UserPosition); m != "" {
				meta.SeriesPosition, _ = strconv.Atoi(m)
			}
		}
	}

	// Dílo (work) zná rok prvního vydání a název originálu – rok má přednost
	// před rokem konkrétního vydání, stejně jako u ostatních zdrojů.
	var work apolloWork
	if lookup(book.Work, &work) {
		if y := yearFromMillis(work.Details.PublicationTime); y != 0 {
			meta.Year = y
		}
		if work.Details.OriginalTitle != "" && work.Details.OriginalTitle != meta.Title {
			meta.OriginalTitle = work.Details.OriginalTitle
		}
		meta.Rating = int(math.Round(work.Stats.AverageRating * 20))
	}
	return meta
}

// parseJSONLD je záloha pro případ, že se změní struktura Next.js dat.
func parseJSONLD(doc *html.Node) *metadata.BookMetadata {
	var meta *metadata.BookMetadata
	htmlutil.Walk(doc, func(n *html.Node) bool {
		if meta != nil {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "script" || htmlutil.Attr(n, "type") != "application/ld+json" {
			return true
		}
		var ld struct {
			Type       string `json:"@type"`
			Name       string `json:"name"`
			Image      string `json:"image"`
			InLanguage string `json:"inLanguage"`
			Author     []struct {
				Name string `json:"name"`
			} `json:"author"`
			AggregateRating struct {
				RatingValue float64 `json:"ratingValue"`
			} `json:"aggregateRating"`
		}
		if json.Unmarshal([]byte(htmlutil.Text(n)), &ld) != nil || ld.Type != "Book" {
			return false
		}
		authors := make([]string, 0, len(ld.Author))
		for _, a := range ld.Author {
			authors = append(authors, a.Name)
		}
		meta = &metadata.BookMetadata{
			Title:    html.UnescapeString(ld.Name),
			Author:   strings.Join(authors, ", "),
			CoverURL: ld.Image,
			Rating:   int(math.Round(ld.AggregateRating.RatingValue * 20)),
			Language: model.LanguageFromEnglishName(ld.InLanguage),
		}
		return false
	})
	return meta
}

func yearFromMillis(ms float64) int {
	if ms == 0 {
		return 0
	}
	return time.UnixMilli(int64(ms)).UTC().Year()
}
