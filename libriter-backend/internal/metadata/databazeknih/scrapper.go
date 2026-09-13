// internal/metadata/databazeknih/scraper.go
//
// Scraper pro databazeknih.cz
// Použití povoleno provozovatelem webu pro osobní nekomerční účely.
//
// Respektuje slušné chování:
//   - User-Agent identifikuje aplikaci
//   - Rate limiting min. 3s mezi požadavky
//   - Scraping pouze on-demand (nová kniha), ne bulk

package databazeknih

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"libriter/internal/metadata"
	"libriter/internal/metadata/htmlutil"

	"golang.org/x/net/html"
)

const (
	providerName = "databazeknih"
	baseURL      = "https://www.databazeknih.cz"
	host         = "databazeknih.cz"
	minDelay     = 3 * time.Second
	searchLimit  = 10
)

// Aliasy na sdílené typy – ať se v parserech nečte metadata.BookMetadata.
type (
	SearchResult = metadata.SearchResult
	BookMetadata = metadata.BookMetadata
)

// Client je scraper klient s rate limitingem
type Client struct {
	fetcher *metadata.Fetcher
}

// NewClient vytvoří nový scraper klient
func NewClient() *Client {
	return &Client{fetcher: metadata.NewFetcher(minDelay)}
}

func (c *Client) Name() string { return providerName }

// Supports přijímá jen adresy z databazeknih.cz – slouží zároveň jako
// allowlist pro /metadata/book?url=.
func (c *Client) Supports(rawURL string) bool {
	return metadata.HostMatches(rawURL, host)
}

// Search vyhledá knihy podle dotazu, vrátí max. 10 výsledků
func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	// Vyhledávací formulář na webu míří na /search?in=books; starší /hledat
	// dnes vrací 404.
	searchURL := fmt.Sprintf("%s/search?in=books&q=%s",
		baseURL,
		url.QueryEscape(query),
	)

	body, err := c.fetch(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search fetch: %w", err)
	}
	defer body.Close()

	return parseSearchResults(body)
}

// FetchBook stáhne metadata knihy podle jejího ID
func (c *Client) FetchBook(ctx context.Context, bookID int) (*BookMetadata, error) {
	// Potřebujeme slug - použijeme redirect přes zkrácené URL
	bookURL := fmt.Sprintf("%s/prehled-knihy/kniha-%d", baseURL, bookID)

	body, err := c.fetch(ctx, bookURL)
	if err != nil {
		return nil, fmt.Errorf("fetch book %d: %w", bookID, err)
	}
	defer body.Close()

	meta, err := parseBookPage(body)
	if err != nil {
		return nil, fmt.Errorf("parse book %d: %w", bookID, err)
	}

	meta.ID = bookID
	meta.SourceURL = bookURL
	return meta, nil
}

// FetchByURL stáhne metadata knihy přímo z URL.
// Preferovaná metoda - URL máme ze search výsledků.
func (c *Client) FetchByURL(ctx context.Context, bookURL string) (*BookMetadata, error) {
	body, err := c.fetch(ctx, bookURL)
	if err != nil {
		return nil, fmt.Errorf("fetch book url: %w", err)
	}
	defer body.Close()

	meta, err := parseBookPage(body)
	if err != nil {
		return nil, fmt.Errorf("parse book page: %w", err)
	}

	meta.ID = extractIDFromURL(bookURL)
	meta.SourceURL = bookURL
	meta.Source = providerName
	return meta, nil
}

// DownloadCover stáhne obálku knihy a vrátí její obsah
func (c *Client) DownloadCover(ctx context.Context, coverURL string) ([]byte, string, error) {
	body, err := c.fetch(ctx, coverURL)
	if err != nil {
		return nil, "", fmt.Errorf("download cover: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, "", fmt.Errorf("read cover: %w", err)
	}

	// Detekce MIME typu podle prvních bytů
	mime := "image/jpeg"
	if len(data) > 4 && string(data[1:4]) == "PNG" {
		mime = "image/png"
	}

	return data, mime, nil
}

// -------------------------------------------------------------------
//  HTTP fetch s rate limitingem
// -------------------------------------------------------------------

func (c *Client) fetch(ctx context.Context, targetURL string) (io.ReadCloser, error) {
	return c.fetcher.Get(ctx, targetURL, "text/html,application/xhtml+xml")
}

// -------------------------------------------------------------------
//  HTML parsery
// -------------------------------------------------------------------

// parseBookPage parsuje stránku prehled-knihy
func parseBookPage(r io.Reader) (*BookMetadata, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	meta := &BookMetadata{}

	// Procházení DOM
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {

			// <h1> = název knihy
			case "h1":
				if meta.Title == "" {
					meta.Title = strings.TrimSpace(htmlutil.Text(n))
				}

			// <meta> tagy - og:image pro obálku, description pro popis
			case "meta":
				prop := htmlutil.Attr(n, "property")
				name := htmlutil.Attr(n, "name")
				content := htmlutil.Attr(n, "content")

				switch {
				case prop == "og:image" && content != "":
					// Chceme full-size, ne bmid_ variantu
					meta.CoverURL = strings.Replace(content, "bmid_", "", 1)
				case (prop == "og:description" || name == "description") && meta.Description == "":
					// og:description je zkrácená - použijeme jen jako fallback
					if meta.Description == "" {
						meta.Description = cleanDescription(content)
					}
				}

			// <a> linky - autor, žánry, nakladatel
			case "a":
				href := htmlutil.Attr(n, "href")
				text := strings.TrimSpace(htmlutil.Text(n))

				switch {
				case strings.Contains(href, "/autori/") && text != "" && meta.Author == "":
					meta.Author = text
					meta.AuthorID = extractIDFromURL(href)

				case strings.Contains(href, "/zanry/") && text != "":
					meta.Genres = append(meta.Genres, text)

				case strings.Contains(href, "/nakladatelstvi/") && text != "" && meta.Publisher == "":
					meta.Publisher = text
				}
			}
		}

		// Hledáme popis v sekci "O knize" - je v <p> za <h2> s textem "O knize"
		if n.Type == html.TextNode {
			trimmed := strings.TrimSpace(n.Data)

			// Rating - hledáme pattern "86 %" v textu
			if ratingRe.MatchString(trimmed) && meta.Rating == 0 {
				if m := ratingRe.FindStringSubmatch(trimmed); len(m) > 1 {
					if v, err := strconv.Atoi(m[1]); err == nil {
						meta.Rating = v
					}
				}
			}

			// Rok vydání - 4 číslice
			if yearRe.MatchString(trimmed) && meta.Year == 0 {
				if m := yearRe.FindStringSubmatch(trimmed); len(m) > 1 {
					if v, err := strconv.Atoi(m[1]); err == nil && v > 1800 && v <= time.Now().Year()+1 {
						meta.Year = v
					}
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(doc)

	// Popis - zkusíme najít sekci "O knize" lépe
	if desc := extractDescription(doc); desc != "" {
		meta.Description = desc
	}

	if meta.Title == "" {
		return nil, fmt.Errorf("titul nenalezen na stránce")
	}

	return meta, nil
}

// parseSearchResults parsuje výsledky vyhledávání knih.
//
// Jeden výsledek je <p class="new"> a uvnitř odkaz na knihu plus
// <span class="pozn"> s textem "2007, Karel Čapek". Autor je pro výběr
// z výsledků zásadní – stejných názvů bývá víc.
//
// Když se struktura stránky změní, spadne se na hledání holých odkazů:
// výsledky pak nemají autora, ale funkce se aspoň nerozbije úplně.
func parseSearchResults(r io.Reader) ([]SearchResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "p" || !htmlutil.HasClass(n, "new") {
			return true
		}

		result, ok := parseSearchRow(n)
		if ok && !seen[result.ID] {
			seen[result.ID] = true
			results = append(results, result)
		}
		return false // řádek je zpracovaný celý, dovnitř už nelezeme
	})

	if len(results) == 0 {
		return parseSearchLinks(doc), nil
	}
	return results, nil
}

// parseSearchRow přečte jeden <p class="new"> s výsledkem.
func parseSearchRow(row *html.Node) (SearchResult, bool) {
	var result SearchResult

	htmlutil.Walk(row, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}

		switch {
		// Odkaz s názvem knihy. Obálka odkazuje na tutéž adresu, ale text nemá.
		case n.Data == "a" && result.Title == "":
			href := htmlutil.Attr(n, "href")
			if !strings.Contains(href, "/prehled-knihy/") && !strings.Contains(href, "/knihy/") {
				return true
			}
			title := htmlutil.Collapse(htmlutil.Text(n))
			if title == "" || strings.Contains(title, "Více") {
				return true
			}
			result.ID = extractIDFromURL(href)
			result.Title = title
			result.URL = htmlutil.ResolveURL(baseURL, href)

		// Poznámka pod názvem: "2007, Karel Čapek".
		case n.Data == "span" && htmlutil.HasClass(n, "pozn") && result.Author == "":
			result.Year, result.Author = parseNote(htmlutil.Collapse(htmlutil.Text(n)))
		}
		return true
	})

	return result, result.ID > 0 && result.Title != ""
}

// parseNote rozdělí "2007, Karel Čapek" na rok a autory. Rok i autor mohou
// chybět; víc autorů zůstane oddělených čárkou.
func parseNote(note string) (year int, author string) {
	parts := strings.Split(note, ",")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i == 0 && yearOnlyRe.MatchString(part) {
			year, _ = strconv.Atoi(part)
			continue
		}
		if author == "" {
			author = part
		} else {
			author += ", " + part
		}
	}
	return year, author
}

// parseSearchLinks je záložní parser pro případ, že se rozložení stránky změní.
func parseSearchLinks(doc *html.Node) []SearchResult {
	var results []SearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}

		href := htmlutil.Attr(n, "href")
		if !strings.Contains(href, "/prehled-knihy/") && !strings.Contains(href, "/knihy/") {
			return true
		}

		id := extractIDFromURL(href)
		title := htmlutil.Collapse(htmlutil.Text(n))
		if id == 0 || title == "" || seen[id] || strings.Contains(title, "Více") {
			return true
		}

		seen[id] = true
		results = append(results, SearchResult{
			ID:    id,
			Title: title,
			URL:   htmlutil.ResolveURL(baseURL, href),
		})
		return true
	})

	return results
}

// -------------------------------------------------------------------
//  Pomocné funkce
// -------------------------------------------------------------------

var (
	ratingRe   = regexp.MustCompile(`(\d{1,3})\s*%`)
	yearRe     = regexp.MustCompile(`\b(1[89]\d{2}|20[012]\d)\b`)
	idRe       = regexp.MustCompile(`-(\d+)$`)
	yearOnlyRe = regexp.MustCompile(`^(?:1[89]\d{2}|20\d{2})$`)
)

func extractIDFromURL(u string) int {
	// Odstranit query string a fragment
	if idx := strings.IndexAny(u, "?#"); idx >= 0 {
		u = u[:idx]
	}
	u = strings.TrimRight(u, "/")

	m := idRe.FindStringSubmatch(u)
	if len(m) < 2 {
		return 0
	}
	id, _ := strconv.Atoi(m[1])
	return id
}

func cleanDescription(s string) string {
	// Oříznutí "... od Autor Jméno" na konci (formát meta description)
	if idx := strings.LastIndex(s, "... od "); idx > 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// extractDescription hledá text v sekci "O knize" v DOM
func extractDescription(doc *html.Node) string {
	var result string
	var inSection bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != "" {
			return
		}

		if n.Type == html.ElementNode && n.Data == "h2" {
			text := strings.TrimSpace(htmlutil.Text(n))
			inSection = strings.Contains(text, "O knize")
		}

		if inSection && n.Type == html.ElementNode && n.Data == "p" {
			text := strings.TrimSpace(htmlutil.Text(n))
			if len(text) > 50 {
				result = text
				inSection = false
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return result
}
