// Package googlebooks čte metadata z Google Books API.
//
// Oficiální JSON API, takže se nerozbíjí se změnou HTML. Má ale kvóty:
// bez API klíče je denní limit sdílený pro celou IP adresu a snadno se
// vyčerpá (odpověď 429). Vlastní klíč se nastaví přes GOOGLE_BOOKS_API_KEY.
// České tituly pokrývá slaběji než databazeknih.cz.
package googlebooks

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"libriter/internal/metadata"
)

const (
	providerName = "googlebooks"
	apiURL       = "https://www.googleapis.com/books/v1/volumes"
	host         = "googleapis.com"
	searchLimit  = 10
)

type Client struct {
	fetcher *metadata.Fetcher
	apiKey  string
	// country je potřeba tam, kde API nedokáže určit zemi z IP, jinak
	// odpovídá chybou o nedostupnosti služby v dané lokalitě.
	country string
}

// NewClient vytvoří klienta. apiKey smí být prázdný – pak se jede na sdílenou
// anonymní kvótu.
func NewClient(apiKey string) *Client {
	return &Client{
		fetcher: metadata.NewFetcher(0),
		apiKey:  apiKey,
		country: "CZ",
	}
}

func (c *Client) Name() string { return providerName }

func (c *Client) Supports(rawURL string) bool {
	return metadata.HostMatches(rawURL, host) && strings.Contains(rawURL, "/books/v1/volumes/")
}

// volume je výřez odpovědi API – zajímavá pole jednoho svazku.
type volume struct {
	ID         string `json:"id"`
	SelfLink   string `json:"selfLink"`
	VolumeInfo struct {
		Title         string   `json:"title"`
		Subtitle      string   `json:"subtitle"`
		Authors       []string `json:"authors"`
		Publisher     string   `json:"publisher"`
		PublishedDate string   `json:"publishedDate"`
		Description   string   `json:"description"`
		Categories    []string `json:"categories"`
		AverageRating float64  `json:"averageRating"` // 1–5
		ImageLinks    struct {
			Thumbnail      string `json:"thumbnail"`
			SmallThumbnail string `json:"smallThumbnail"`
		} `json:"imageLinks"`
		InfoLink string `json:"infoLink"`
	} `json:"volumeInfo"`
}

type volumesResponse struct {
	TotalItems int      `json:"totalItems"`
	Items      []volume `json:"items"`
}

func (c *Client) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.SearchResult, error) {
	params := url.Values{}
	params.Set("q", searchExpr(q))
	params.Set("maxResults", strconv.Itoa(searchLimit))
	params.Set("country", c.country)
	if c.apiKey != "" {
		params.Set("key", c.apiKey)
	}

	var resp volumesResponse
	if err := c.fetcher.GetJSON(ctx, apiURL+"?"+params.Encode(), &resp); err != nil {
		return nil, c.describe(err)
	}

	results := make([]metadata.SearchResult, 0, len(resp.Items))
	for _, item := range resp.Items {
		info := item.VolumeInfo
		if info.Title == "" || item.ID == "" {
			continue
		}
		results = append(results, metadata.SearchResult{
			Title:  fullTitle(info.Title, info.Subtitle),
			Author: strings.Join(info.Authors, ", "),
			Year:   parseYear(info.PublishedDate),
			// Do detailu se chodí přes selfLink, ne přes odkaz na web –
			// jen ten je strojově čitelný.
			URL:    c.volumeURL(item),
			Source: providerName,
		})
	}
	return results, nil
}

// searchExpr složí dotaz v syntaxi Google Books. Autor patří do inauthor:,
// jinak by se hledal i v názvu a popisu a knihu o autorovi by vrátil dřív
// než knihu od něj.
func searchExpr(q metadata.SearchQuery) string {
	title, author := strings.TrimSpace(q.Title), strings.TrimSpace(q.Author)
	switch {
	case title != "" && author != "":
		return fmt.Sprintf("intitle:%q inauthor:%q", title, author)
	case author != "":
		return fmt.Sprintf("inauthor:%q", author)
	default:
		return title
	}
}

func (c *Client) FetchByURL(ctx context.Context, rawURL string) (*metadata.BookMetadata, error) {
	fetchURL := rawURL
	if c.apiKey != "" && !strings.Contains(rawURL, "key=") {
		fetchURL = appendParam(rawURL, "key", c.apiKey)
	}

	var item volume
	if err := c.fetcher.GetJSON(ctx, fetchURL, &item); err != nil {
		return nil, c.describe(err)
	}

	info := item.VolumeInfo
	if info.Title == "" {
		return nil, fmt.Errorf("svazek nemá název: %s", rawURL)
	}

	meta := &metadata.BookMetadata{
		Title:       fullTitle(info.Title, info.Subtitle),
		Author:      strings.Join(info.Authors, ", "),
		Description: strings.TrimSpace(info.Description),
		Genres:      info.Categories,
		CoverURL:    coverURL(info.ImageLinks.Thumbnail, info.ImageLinks.SmallThumbnail),
		// averageRating je 1–5, společná škála je v procentech.
		Rating:    int(info.AverageRating * 20),
		Publisher: info.Publisher,
		Year:      parseYear(info.PublishedDate),
		SourceURL: rawURL,
		Source:    providerName,
	}
	return meta, nil
}

// describe převede odpověď o limitu na hlášku, ze které je poznat, co s tím.
// 429 bez klíče je nejčastější stav: anonymní denní kvóta je sdílená pro
// celou IP adresu, takže dojde i při pár dotazech.
func (c *Client) describe(err error) error {
	switch metadata.StatusOf(err) {
	case http.StatusTooManyRequests:
		if c.apiKey == "" {
			return fmt.Errorf("vyčerpaná denní kvóta Google Books; nastavte GOOGLE_BOOKS_API_KEY: %w", err)
		}
		return fmt.Errorf("vyčerpaná kvóta Google Books API klíče: %w", err)
	case http.StatusForbidden:
		return fmt.Errorf("Google Books odmítl požadavek (neplatný GOOGLE_BOOKS_API_KEY?): %w", err)
	default:
		return err
	}
}

func (c *Client) volumeURL(item volume) string {
	if item.SelfLink != "" {
		return item.SelfLink
	}
	return apiURL + "/" + item.ID
}

func fullTitle(title, subtitle string) string {
	if subtitle == "" {
		return title
	}
	return title + ": " + subtitle
}

// parseYear vytáhne rok z publishedDate, které bývá "2016", "2016-05" i "2016-05-17".
func parseYear(published string) int {
	if len(published) < 4 {
		return 0
	}
	year, err := strconv.Atoi(published[:4])
	if err != nil {
		return 0
	}
	return year
}

// coverURL preferuje větší náhled a přepne odkaz na https.
func coverURL(thumbnail, small string) string {
	link := thumbnail
	if link == "" {
		link = small
	}
	return strings.Replace(link, "http://", "https://", 1)
}

func appendParam(rawURL, key, value string) string {
	separator := "?"
	if strings.Contains(rawURL, "?") {
		separator = "&"
	}
	return rawURL + separator + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}
