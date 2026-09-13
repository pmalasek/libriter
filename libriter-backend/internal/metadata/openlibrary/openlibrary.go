// Package openlibrary čte metadata z openlibrary.org.
//
// Na rozdíl od scraperů jde o oficiální JSON API: zdarma, bez klíče a bez
// limitu na počet dotazů, jen s prosbou o identifikující User-Agent.
// Českou beletrii pokrývá spíš děravě, hodí se proto jako záložní zdroj.
package openlibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"libriter/internal/metadata"
)

const (
	providerName = "openlibrary"
	baseURL      = "https://openlibrary.org"
	host         = "openlibrary.org"
	coversURL    = "https://covers.openlibrary.org/b/id"
	searchLimit  = 10
)

type Client struct {
	fetcher *metadata.Fetcher
}

// NewClient vytvoří klienta. Prodleva mezi požadavky není potřeba – jde
// o oficiální API, ne o scraping.
func NewClient() *Client {
	return &Client{fetcher: metadata.NewFetcher(0)}
}

func (c *Client) Name() string { return providerName }

func (c *Client) Supports(rawURL string) bool {
	return metadata.HostMatches(rawURL, host)
}

// --- vyhledávání ---

type searchResponse struct {
	NumFound int `json:"numFound"`
	Docs     []struct {
		Key              string   `json:"key"` // "/works/OL575639W"
		Title            string   `json:"title"`
		AuthorName       []string `json:"author_name"`
		FirstPublishYear int      `json:"first_publish_year"`
		CoverID          int      `json:"cover_i"`
	} `json:"docs"`
}

func (c *Client) Search(ctx context.Context, query string) ([]metadata.SearchResult, error) {
	searchURL := fmt.Sprintf(
		"%s/search.json?q=%s&limit=%d&fields=key,title,author_name,first_publish_year,cover_i",
		baseURL, url.QueryEscape(query), searchLimit,
	)

	var resp searchResponse
	if err := c.fetcher.GetJSON(ctx, searchURL, &resp); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	results := make([]metadata.SearchResult, 0, len(resp.Docs))
	for _, doc := range resp.Docs {
		if doc.Title == "" || !strings.HasPrefix(doc.Key, "/works/") {
			continue
		}
		results = append(results, metadata.SearchResult{
			Title:  doc.Title,
			Author: strings.Join(doc.AuthorName, ", "),
			Year:   doc.FirstPublishYear,
			URL:    baseURL + doc.Key,
			Source: providerName,
		})
	}
	return results, nil
}

// --- detail ---

type workResponse struct {
	Title string `json:"title"`
	// description je buď řetězec, nebo objekt {"type":…,"value":…}.
	Description flexibleText `json:"description"`
	Subjects    []string     `json:"subjects"`
	Covers      []int        `json:"covers"`
	Authors     []struct {
		Author struct {
			Key string `json:"key"` // "/authors/OL1155711A"
		} `json:"author"`
	} `json:"authors"`
}

func (c *Client) FetchByURL(ctx context.Context, rawURL string) (*metadata.BookMetadata, error) {
	workKey, ok := workKeyFromURL(rawURL)
	if !ok {
		return nil, fmt.Errorf("URL nevede na dílo (/works/…): %s", rawURL)
	}

	var work workResponse
	if err := c.fetcher.GetJSON(ctx, baseURL+workKey+".json", &work); err != nil {
		return nil, fmt.Errorf("fetch work: %w", err)
	}
	if work.Title == "" {
		return nil, fmt.Errorf("dílo %s nemá název", workKey)
	}

	meta := &metadata.BookMetadata{
		Title:       work.Title,
		Description: work.Description.String(),
		Genres:      work.Subjects,
		SourceURL:   baseURL + workKey,
		Source:      providerName,
	}
	if len(work.Covers) > 0 && work.Covers[0] > 0 {
		meta.CoverURL = fmt.Sprintf("%s/%d-L.jpg", coversURL, work.Covers[0])
	}

	// Jméno autora ve work JSONu není, jen odkaz – doplní se dalším dotazem.
	// Rok vydání a nakladatel patří konkrétnímu vydání, ne dílu, takže
	// zůstávají prázdné.
	if len(work.Authors) > 0 {
		if name := c.authorName(ctx, work.Authors[0].Author.Key); name != "" {
			meta.Author = name
		}
	}

	return meta, nil
}

// authorName dotáhne jméno autora. Selhání se ignoruje – kniha bez jména
// autora je pořád použitelnější než chyba celého importu.
func (c *Client) authorName(ctx context.Context, key string) string {
	if !strings.HasPrefix(key, "/authors/") {
		return ""
	}

	var author struct {
		Name string `json:"name"`
	}
	if err := c.fetcher.GetJSON(ctx, baseURL+key+".json", &author); err != nil {
		return ""
	}
	return author.Name
}

// workKeyFromURL vytáhne "/works/OL575639W" z adresy díla.
func workKeyFromURL(rawURL string) (string, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}

	path := strings.TrimSuffix(strings.TrimSuffix(u.Path, "/"), ".json")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 2 || parts[0] != "works" || parts[1] == "" {
		return "", false
	}
	return "/works/" + parts[1], true
}

// flexibleText čte pole, které OpenLibrary posílá jednou jako řetězec
// a jindy jako objekt {"type": "/type/text", "value": "…"}.
type flexibleText struct {
	value string
}

func (t *flexibleText) UnmarshalJSON(data []byte) error {
	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		t.value = asString
		return nil
	}

	var asObject struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &asObject); err != nil {
		return nil // neznámý tvar pole import neshazuje
	}
	t.value = asObject.Value
	return nil
}

func (t flexibleText) String() string { return strings.TrimSpace(t.value) }
