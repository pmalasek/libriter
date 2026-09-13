// Package cbdb čte metadata z cbdb.cz (Česká bibliografická databáze).
//
// Web nemá veřejné API, takže jde o scraper HTML se stejnými pravidly jako
// u databazeknih.cz: identifikující User-Agent, prodleva mezi požadavky
// a dotazy jen na vyžádání, ne hromadně.
//
// Struktura stránky, na které stojíme:
//   - hledání: /hledat?text=<dotaz>, odkazy tvaru "kniha-<id>-<slug>"
//   - detail:  <h1> název, odkaz "autor-<id>-<slug>", <div class="book_description">,
//     obálka v <meta property="og:image">
package cbdb

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
	providerName = "cbdb"
	baseURL      = "https://www.cbdb.cz"
	host         = "cbdb.cz"
	minDelay     = 3 * time.Second
	searchLimit  = 10
)

var (
	bookHrefRe   = regexp.MustCompile(`(?:^|/)kniha-(\d+)-`)
	authorHrefRe = regexp.MustCompile(`(?:^|/)autor-(\d+)-`)
	ratingRe     = regexp.MustCompile(`^(\d{1,3})\s*%$`)
	// Popis začíná SEO otázkou "O čem je kniha <název>?" – ta do anotace nepatří.
	descPrefixRe = regexp.MustCompile(`^O čem je kniha[^?]*\?\s*`)
)

type Client struct {
	fetcher *metadata.Fetcher
}

func NewClient() *Client {
	return &Client{fetcher: metadata.NewFetcher(minDelay)}
}

func (c *Client) Name() string { return providerName }

func (c *Client) Supports(rawURL string) bool {
	return metadata.HostMatches(rawURL, host)
}

func (c *Client) fetch(ctx context.Context, targetURL string) (*html.Node, error) {
	body, err := c.fetcher.Get(ctx, targetURL, "text/html,application/xhtml+xml")
	if err != nil {
		return nil, err
	}
	defer body.Close()

	return html.Parse(io.LimitReader(body, 8<<20))
}

// --- vyhledávání ---

func (c *Client) Search(ctx context.Context, query string) ([]metadata.SearchResult, error) {
	searchURL := fmt.Sprintf("%s/hledat?text=%s&ok=", baseURL, url.QueryEscape(query))

	doc, err := c.fetch(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return parseSearchResults(doc), nil
}

// parseSearchResults čte výsledky hledání knih. Jeden výsledek je
// <div class="search_graphic_box">, uvnitř odkaz na knihu s názvem a
// <span class="search_author_link"> pro každého autora. Autor je pro výběr
// z výsledků zásadní – stejných názvů bývá víc.
//
// Když se rozložení stránky změní, spadne se na hledání holých odkazů:
// výsledky pak nemají autora, ale funkce se aspoň nerozbije úplně.
func parseSearchResults(doc *html.Node) []metadata.SearchResult {
	var results []metadata.SearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "div" || !htmlutil.HasClass(n, "search_graphic_box") {
			return true
		}

		result, ok := parseSearchRow(n)
		if ok && !seen[result.ID] {
			seen[result.ID] = true
			results = append(results, result)
		}
		return false // řádek je zpracovaný celý
	})

	if len(results) == 0 {
		return parseSearchLinks(doc)
	}
	return results
}

// parseSearchRow přečte jeden <div class="search_graphic_box">.
func parseSearchRow(row *html.Node) (metadata.SearchResult, bool) {
	result := metadata.SearchResult{Source: providerName}
	var authors []string

	htmlutil.Walk(row, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}

		switch {
		case n.Data == "a" && result.Title == "":
			href := htmlutil.Attr(n, "href")
			match := bookHrefRe.FindStringSubmatch(href)
			if match == nil {
				return true
			}
			// Odkaz s obálkou vede na tutéž adresu, ale nese jen hodnocení.
			title := htmlutil.Collapse(htmlutil.Text(n))
			if title == "" || ratingRe.MatchString(title) {
				return true
			}
			result.ID, _ = strconv.Atoi(match[1])
			result.Title = title
			result.URL = htmlutil.ResolveURL(baseURL, href)

		case n.Data == "span" && htmlutil.HasClass(n, "search_author_link"):
			if name := htmlutil.Collapse(htmlutil.Text(n)); name != "" {
				authors = append(authors, name)
			}
		}
		return true
	})

	result.Author = strings.Join(authors, ", ")
	return result, result.ID > 0 && result.Title != ""
}

// parseSearchLinks je záložní parser pro případ, že se rozložení stránky změní.
func parseSearchLinks(doc *html.Node) []metadata.SearchResult {
	var results []metadata.SearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}

		href := htmlutil.Attr(n, "href")
		match := bookHrefRe.FindStringSubmatch(href)
		if match == nil {
			return true
		}
		id, err := strconv.Atoi(match[1])
		if err != nil || id == 0 || seen[id] {
			return true
		}

		title := htmlutil.Collapse(htmlutil.Text(n))
		if title == "" || ratingRe.MatchString(title) {
			return true
		}

		seen[id] = true
		results = append(results, metadata.SearchResult{
			ID:     id,
			Title:  title,
			URL:    htmlutil.ResolveURL(baseURL, href),
			Source: providerName,
		})
		return true
	})

	return results
}

// --- detail ---

func (c *Client) FetchByURL(ctx context.Context, rawURL string) (*metadata.BookMetadata, error) {
	doc, err := c.fetch(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch book: %w", err)
	}

	meta := parseBookPage(doc)
	if meta.Title == "" {
		return nil, fmt.Errorf("titul nenalezen na stránce %s", rawURL)
	}

	if match := bookHrefRe.FindStringSubmatch(rawURL); match != nil {
		meta.ID, _ = strconv.Atoi(match[1])
	}
	meta.SourceURL = rawURL
	meta.Source = providerName
	return meta, nil
}

func parseBookPage(doc *html.Node) *metadata.BookMetadata {
	meta := &metadata.BookMetadata{}

	if h1 := htmlutil.Element(doc, "h1"); h1 != nil {
		meta.Title = htmlutil.Collapse(htmlutil.Text(h1))
	}
	meta.CoverURL = htmlutil.MetaContent(doc, "og:image")

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}

		switch {
		// Anotace knihy.
		case n.Data == "div" && htmlutil.HasClass(n, "book_description") && meta.Description == "":
			text := htmlutil.Collapse(htmlutil.Text(n))
			meta.Description = strings.TrimSpace(descPrefixRe.ReplaceAllString(text, ""))

		case n.Data == "a":
			href := htmlutil.Attr(n, "href")
			text := htmlutil.Collapse(htmlutil.Text(n))

			if match := authorHrefRe.FindStringSubmatch(href); match != nil && text != "" && meta.Author == "" {
				meta.Author = text
				meta.AuthorID, _ = strconv.Atoi(match[1])
			}

		// Hodnocení je samostatný odkaz/element s textem "87 %".
		case meta.Rating == 0:
			if match := ratingRe.FindStringSubmatch(htmlutil.Collapse(htmlutil.Text(n))); match != nil {
				if value, err := strconv.Atoi(match[1]); err == nil && value <= 100 {
					meta.Rating = value
				}
			}
		}
		return true
	})

	return meta
}
