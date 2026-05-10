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
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	baseURL   = "https://www.databazeknih.cz"
	userAgent = "Libriter/1.0 (osobni-audiobook-knihovna; +https://github.com/libriter)"
	minDelay  = 3 * time.Second
)

// BookMetadata jsou metadata stažená z databazeknih.cz
type BookMetadata struct {
	DatabazeknihID int
	Title          string
	Author         string
	AuthorID       int
	Description    string
	Genres         []string
	CoverURL       string
	Rating         int // 0-100
	Publisher      string
	Year           int
	SourceURL      string
}

// SearchResult je jeden výsledek vyhledávání
type SearchResult struct {
	ID     int
	Title  string
	Author string
	Year   int
	URL    string
}

// Client je scraper klient s rate limitingem
type Client struct {
	http     *http.Client
	lastReq  time.Time
	minDelay time.Duration
}

// NewClient vytvoří nový scraper klient
func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		minDelay: minDelay,
	}
}

// Search vyhledá knihy podle dotazu, vrátí max. 10 výsledků
func (c *Client) Search(ctx context.Context, query string) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("%s/hledat?q=%s&hledat=Hledat",
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

	meta.DatabazeknihID = bookID
	meta.SourceURL = bookURL
	return meta, nil
}

// FetchBookByURL stáhne metadata knihy přímo z URL
// Preferovaná metoda - URL máme ze search výsledků
func (c *Client) FetchBookByURL(ctx context.Context, bookURL string) (*BookMetadata, error) {
	body, err := c.fetch(ctx, bookURL)
	if err != nil {
		return nil, fmt.Errorf("fetch book url: %w", err)
	}
	defer body.Close()

	meta, err := parseBookPage(body)
	if err != nil {
		return nil, fmt.Errorf("parse book page: %w", err)
	}

	meta.DatabazeknihID = extractIDFromURL(bookURL)
	meta.SourceURL = bookURL
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
	// Rate limiting - počkáme pokud jsme volali příliš nedávno
	if elapsed := time.Since(c.lastReq); elapsed < c.minDelay {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.minDelay - elapsed):
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "cs,en;q=0.9")

	resp, err := c.http.Do(req)
	c.lastReq = time.Now()

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d pro %s", resp.StatusCode, targetURL)
	}

	return resp.Body, nil
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
					meta.Title = strings.TrimSpace(nodeText(n))
				}

			// <meta> tagy - og:image pro obálku, description pro popis
			case "meta":
				prop := attr(n, "property")
				name := attr(n, "name")
				content := attr(n, "content")

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
				href := attr(n, "href")
				text := strings.TrimSpace(nodeText(n))

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

// parseSearchResults parsuje výsledky vyhledávání
func parseSearchResults(r io.Reader) ([]SearchResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Výsledky hledání jsou v <a> odkazech na /knihy/ nebo /prehled-knihy/
		if n.Type == html.ElementNode && n.Data == "a" {
			href := attr(n, "href")
			if strings.Contains(href, "/knihy/") || strings.Contains(href, "/prehled-knihy/") {
				id := extractIDFromURL(href)
				title := strings.TrimSpace(nodeText(n))
				if id > 0 && title != "" && !strings.Contains(title, "Více") {
					// Deduplikace
					for _, r := range results {
						if r.ID == id {
							goto next
						}
					}
					results = append(results, SearchResult{
						ID:    id,
						Title: title,
						URL:   fullURL(href),
					})
					if len(results) >= 10 {
						return
					}
				}
			}
		}
	next:
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return results, nil
}

// -------------------------------------------------------------------
//  Pomocné funkce
// -------------------------------------------------------------------

var (
	ratingRe = regexp.MustCompile(`(\d{1,3})\s*%`)
	yearRe   = regexp.MustCompile(`\b(1[89]\d{2}|20[012]\d)\b`)
	idRe     = regexp.MustCompile(`-(\d+)$`)
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

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func fullURL(href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return baseURL + href
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
			text := strings.TrimSpace(nodeText(n))
			inSection = strings.Contains(text, "O knize")
		}

		if inSection && n.Type == html.ElementNode && n.Data == "p" {
			text := strings.TrimSpace(nodeText(n))
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
