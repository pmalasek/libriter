// Package audible čte metadata audioknih z katalogového API Audible.
//
// Katalog (api.audible.<tld>/1.0/catalog/products) je veřejný JSON bez
// přihlášení – stejný, ze kterého čte mobilní aplikace Audible. Každý
// marketplace (audible.com, audible.de, …) má vlastní katalog, proto se
// zdroj registruje zvlášť pro každý z nich (audible_com, audible_de).
//
// Oproti knižním databázím zná interpreta a sérii s pořadím dílu; rok je ale
// rok vydání audioknihy, ne prvního vydání díla.
package audible

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"libriter/internal/metadata"
	"libriter/internal/metadata/htmlutil"
	"libriter/internal/model"
)

const (
	searchLimit = 10
	// Skupiny určují části záznamu v odpovědi – název je až v product_desc.
	searchGroups = "contributors,product_attrs,product_desc"
	detailGroups = "contributors,media,product_attrs,product_desc,product_extended_attrs,series,rating,category_ladders"
	imageSizes   = "1215,500"
)

// Marketplace je jeden katalog Audible.
type Marketplace struct {
	// Name je jméno zdroje v nastavení.
	Name string
	// TLD je doména katalogu: "com", "de", "co.uk".
	TLD string
	// AcceptLanguage určuje jazyk anotací a žánrů v odpovědi.
	AcceptLanguage string
}

var (
	MarketplaceCOM = Marketplace{Name: "audible_com", TLD: "com", AcceptLanguage: "en-US,en;q=0.9"}
	MarketplaceDE  = Marketplace{Name: "audible_de", TLD: "de", AcceptLanguage: "de-DE,de;q=0.9,en;q=0.5"}
)

// asinRe je ASIN na konci cesty stránky produktu (/pd/<slug>/<ASIN>).
var asinRe = regexp.MustCompile(`/([A-Z0-9]{10})/?$`)

var sequenceRe = regexp.MustCompile(`\d+`)

type Client struct {
	fetcher *metadata.Fetcher
	market  Marketplace
	// apiBase a webBase jdou v testech přesměrovat na httptest server.
	apiBase string
	webBase string
}

func NewClient(market Marketplace) *Client {
	return &Client{
		fetcher: metadata.NewFetcher(0).WithAcceptLanguage(market.AcceptLanguage),
		market:  market,
		apiBase: "https://api.audible." + market.TLD,
		webBase: "https://www.audible." + market.TLD,
	}
}

func (c *Client) Name() string { return c.market.Name }

// SupportsCoverURL: obálky jsou na CDN Amazonu.
func (c *Client) SupportsCoverURL(rawURL string) bool {
	return metadata.HostMatches(rawURL, "m.media-amazon.com")
}

// Supports přijme stránku produktu na webu tohoto marketplace – tu vrací
// hledání a tu si editor může zkopírovat z prohlížeče.
func (c *Client) Supports(rawURL string) bool {
	return metadata.HostMatches(rawURL, "audible."+c.market.TLD) && asinFromURL(rawURL) != ""
}

func asinFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if m := asinRe.FindStringSubmatch(u.Path); m != nil {
		return m[1]
	}
	return ""
}

// product je výřez záznamu katalogu – jen pole, která zdroj používá.
type product struct {
	ASIN      string   `json:"asin"`
	Title     string   `json:"title"`
	Authors   []person `json:"authors"`
	Narrators []person `json:"narrators"`
	// Language je anglický název jazyka („english“, „german“).
	Language         string            `json:"language"`
	PublisherName    string            `json:"publisher_name"`
	ReleaseDate      string            `json:"release_date"`
	PublisherSummary string            `json:"publisher_summary"`
	MerchSummary     string            `json:"merchandising_summary"`
	ProductImages    map[string]string `json:"product_images"`
	Series           []struct {
		Title    string `json:"title"`
		Sequence string `json:"sequence"`
	} `json:"series"`
	Rating *struct {
		Overall struct {
			Average float64 `json:"average_rating"` // 1–5
		} `json:"overall_distribution"`
	} `json:"rating"`
	CategoryLadders []struct {
		Ladder []struct {
			Name string `json:"name"`
		} `json:"ladder"`
	} `json:"category_ladders"`
}

type person struct {
	ASIN string `json:"asin"`
	Name string `json:"name"`
}

func (c *Client) Search(ctx context.Context, q metadata.SearchQuery) ([]metadata.SearchResult, error) {
	params := url.Values{}
	if title := strings.TrimSpace(q.Title); title != "" {
		params.Set("title", title)
	}
	if author := strings.TrimSpace(q.Author); author != "" {
		params.Set("author", author)
	}
	if len(params) == 0 {
		return nil, nil
	}
	params.Set("num_results", strconv.Itoa(searchLimit))
	params.Set("products_sort_by", "Relevance")
	params.Set("response_groups", searchGroups)

	var resp struct {
		Products []product `json:"products"`
	}
	if err := c.fetcher.GetJSON(ctx, c.apiBase+"/1.0/catalog/products?"+params.Encode(), &resp); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	results := make([]metadata.SearchResult, 0, len(resp.Products))
	for _, p := range resp.Products {
		if p.ASIN == "" || p.Title == "" {
			continue
		}
		// Katalog míchá vydání v různých jazycích (anglický Audible nabízí
		// i italský překlad). Když jazyk knihy známe, jiná vydání vynecháme;
		// záznam bez jazyka necháme být.
		if lang := model.LanguageFromEnglishName(p.Language); q.Language != "" && lang != "" && lang != q.Language {
			continue
		}
		results = append(results, metadata.SearchResult{
			Title:  p.Title,
			Author: authorNames(p.Authors),
			Year:   metadata.YearFromDate(p.ReleaseDate),
			URL:    c.webBase + "/pd/" + p.ASIN,
			Source: c.market.Name,
		})
	}
	return metadata.RankByAuthor(results, q.Author), nil
}

func (c *Client) FetchByURL(ctx context.Context, rawURL string) (*metadata.BookMetadata, error) {
	asin := asinFromURL(rawURL)
	if asin == "" {
		return nil, fmt.Errorf("adresa neobsahuje ASIN: %s", rawURL)
	}

	params := url.Values{}
	params.Set("response_groups", detailGroups)
	params.Set("image_sizes", imageSizes)

	var resp struct {
		Product product `json:"product"`
	}
	if err := c.fetcher.GetJSON(ctx, c.apiBase+"/1.0/catalog/products/"+asin+"?"+params.Encode(), &resp); err != nil {
		return nil, fmt.Errorf("detail %s: %w", asin, err)
	}

	p := resp.Product
	if p.Title == "" {
		return nil, fmt.Errorf("produkt %s nemá název", asin)
	}

	summary := p.PublisherSummary
	if strings.TrimSpace(summary) == "" {
		summary = p.MerchSummary
	}

	meta := &metadata.BookMetadata{
		Title:       p.Title,
		Author:      authorNames(p.Authors),
		Narrator:    names(p.Narrators),
		Description: htmlutil.FragmentText(summary),
		Genres:      genres(p),
		CoverURL:    largestImage(p.ProductImages),
		Publisher:   p.PublisherName,
		Year:        metadata.YearFromDate(p.ReleaseDate),
		Language:    model.LanguageFromEnglishName(p.Language),
		SourceURL:   c.webBase + "/pd/" + asin,
		Source:      c.market.Name,
	}
	if p.Rating != nil {
		meta.Rating = int(math.Round(p.Rating.Overall.Average * 20))
	}
	if len(p.Series) > 0 {
		meta.Series = p.Series[0].Title
		if m := sequenceRe.FindString(p.Series[0].Sequence); m != "" {
			meta.SeriesPosition, _ = strconv.Atoi(m)
		}
	}
	return meta, nil
}

// authorNames vybere skutečné autory. Katalog mezi autory míchá překladatele
// a úpravce rozhlasové hry („Klaus Fritz - Übersetzer“) bez označení role;
// ti ale nemají stránku autora (ASIN). Když ji má aspoň jeden, ostatní se
// vynechají.
func authorNames(people []person) string {
	var withPage []person
	for _, p := range people {
		if p.ASIN != "" {
			withPage = append(withPage, p)
		}
	}
	if len(withPage) > 0 {
		return names(withPage)
	}
	return names(people)
}

func names(people []person) string {
	out := make([]string, 0, len(people))
	for _, p := range people {
		if name := strings.TrimSpace(p.Name); name != "" {
			out = append(out, name)
		}
	}
	return strings.Join(out, ", ")
}

// genres posbírá kategorie ze všech větví stromu, bez opakování.
func genres(p product) []string {
	var out []string
	seen := map[string]bool{}
	for _, ladder := range p.CategoryLadders {
		for _, cat := range ladder.Ladder {
			if cat.Name != "" && !seen[cat.Name] {
				seen[cat.Name] = true
				out = append(out, cat.Name)
			}
		}
	}
	return out
}

// largestImage vybere obálku s největším rozměrem (klíče jsou pixely).
func largestImage(images map[string]string) string {
	best, bestSize := "", 0
	for size, u := range images {
		if n, err := strconv.Atoi(size); err == nil && n > bestSize {
			best, bestSize = u, n
		}
	}
	return best
}
