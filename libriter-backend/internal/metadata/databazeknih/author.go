package databazeknih

// Metadata autorů z databazeknih.cz.
//
// Struktura, na které stojíme:
//   - hledání: /search?in=authors&q=<dotaz>, odkazy tvaru "/autori/<slug>-<id>"
//   - detail:  JSON-LD schema.org/Person (jméno, životopis, fotka),
//     záložně <h1> a <meta property="og:image">

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"libriter/internal/metadata"
	"libriter/internal/metadata/htmlutil"

	"golang.org/x/net/html"
)

func (c *Client) SupportsAuthorURL(rawURL string) bool {
	return metadata.HostMatches(rawURL, host) && strings.Contains(rawURL, "/autori/")
}

// SupportsImageURL – fotky autorů jsou na témže hostiteli (/img/authors/...).
func (c *Client) SupportsImageURL(rawURL string) bool {
	return metadata.HostMatches(rawURL, host)
}

func (c *Client) SearchAuthors(ctx context.Context, query string) ([]metadata.AuthorSearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?in=authors&q=%s", baseURL, url.QueryEscape(query))

	body, err := c.fetch(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search authors: %w", err)
	}
	defer body.Close()

	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}
	return parseAuthorSearchResults(doc), nil
}

func parseAuthorSearchResults(doc *html.Node) []metadata.AuthorSearchResult {
	var results []metadata.AuthorSearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}

		href := htmlutil.Attr(n, "href")
		if !strings.Contains(href, "/autori/") {
			return true
		}

		id := extractIDFromURL(href)
		name := htmlutil.Collapse(htmlutil.Text(n))
		if id == 0 || name == "" || seen[id] || strings.Contains(name, "Více") {
			return true
		}

		seen[id] = true
		results = append(results, metadata.AuthorSearchResult{
			ID:     id,
			Name:   name,
			URL:    htmlutil.ResolveURL(baseURL, href),
			Source: providerName,
		})
		return true
	})

	return results
}

func (c *Client) FetchAuthorByURL(ctx context.Context, rawURL string) (*metadata.AuthorMetadata, error) {
	body, err := c.fetch(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch author: %w", err)
	}
	defer body.Close()

	doc, err := html.Parse(body)
	if err != nil {
		return nil, err
	}

	meta := parseAuthorPage(doc)
	if meta.Name == "" {
		return nil, fmt.Errorf("jméno autora nenalezeno na stránce %s", rawURL)
	}

	meta.ID = extractIDFromURL(rawURL)
	meta.SourceURL = rawURL
	meta.Source = providerName
	return meta, nil
}

func parseAuthorPage(doc *html.Node) *metadata.AuthorMetadata {
	meta := &metadata.AuthorMetadata{}

	// JSON-LD je stabilnější než HTML, proto má přednost.
	if person := htmlutil.FindPerson(doc); person != nil {
		meta.Name = person.Name
		meta.Bio = strings.TrimSpace(person.Description)
		meta.ImageURL = person.Image
		meta.BirthYear = metadata.YearFromDate(person.BirthDate)
		meta.DeathYear = metadata.YearFromDate(person.DeathDate)
	}

	if meta.Name == "" {
		if h1 := htmlutil.Element(doc, "h1"); h1 != nil {
			meta.Name = htmlutil.Collapse(htmlutil.Text(h1))
		}
	}
	if meta.ImageURL == "" {
		meta.ImageURL = htmlutil.MetaContent(doc, "og:image")
	}

	// Roky života bývají hned v první větě životopisu ("Narodil se 9.1. 1890").
	if meta.BirthYear == 0 {
		meta.BirthYear, meta.DeathYear = metadata.YearsFromText(meta.Bio)
	}

	return meta
}
