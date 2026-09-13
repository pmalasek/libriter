// Package metadata sjednocuje zdroje knižních metadat (databazeknih.cz,
// cbdb.cz, OpenLibrary, Google Books) za jedno rozhraní.
//
// Poskytovatelé se zapojují do Chain, která je zkouší v pořadí ze
// konfigurace – první, kdo něco najde, vyhrává. Díky tomu rozbitý scraper
// cizího webu nezablokuje celou funkci, jen se přeskočí.
package metadata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

// SearchResult je jeden výsledek vyhledávání. Author a Year mohou zůstat
// prázdné – ne každý zdroj je dává už v seznamu, doplní se až detailem.
type SearchResult struct {
	// ID je interní identifikátor u zdroje; 0, pokud zdroj číselné ID nemá.
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
	// URL se posílá zpět do /metadata/book?url= pro stažení detailu.
	URL string `json:"url"`
	// Source je jméno poskytovatele, ze kterého výsledek pochází.
	Source string `json:"source"`
}

// BookMetadata jsou metadata jedné knihy. Nevyplněná pole zůstávají nulová –
// zdroje se v tom, co nabízejí, dost liší.
type BookMetadata struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	AuthorID    int      `json:"author_id"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
	CoverURL    string   `json:"cover_url"`
	// Rating je hodnocení zdroje v procentech (0–100), přepočtené na společnou
	// škálu. Není to internal_rating knihy (1–5).
	Rating    int    `json:"rating"`
	Publisher string `json:"publisher"`
	Year      int    `json:"year"`
	SourceURL string `json:"source_url"`
	Source    string `json:"source"`
}

// Provider je jeden zdroj metadat.
type Provider interface {
	// Name je krátký identifikátor používaný v konfiguraci (METADATA_PROVIDERS).
	Name() string
	// Supports říká, jestli poskytovatel umí stáhnout detail z této URL.
	// Slouží zároveň jako allowlist – handler přes něj pouští /metadata/book?url=.
	Supports(rawURL string) bool
	Search(ctx context.Context, query string) ([]SearchResult, error)
	FetchByURL(ctx context.Context, rawURL string) (*BookMetadata, error)
}

// ErrNoProvider znamená, že žádný nakonfigurovaný zdroj danou URL neumí.
var ErrNoProvider = errors.New("žádný zdroj metadat neumí tuto adresu")

// Chain zkouší poskytovatele v zadaném pořadí.
type Chain struct {
	providers []Provider
}

func NewChain(providers ...Provider) *Chain {
	return &Chain{providers: providers}
}

// Providers vrací jména zdrojů v pořadí, ve kterém se zkoušejí.
func (c *Chain) Providers() []string {
	names := make([]string, 0, len(c.providers))
	for _, p := range c.providers {
		names = append(names, p.Name())
	}
	return names
}

// Search vrátí výsledky prvního zdroje, který něco najde. Zdroj, který selže
// nebo nic nevrátí, se přeskočí.
//
// Chyba se vrací jen tehdy, když nic nenašel nikdo A aspoň jeden zdroj selhal –
// jinak by se rozbitý scraper tvářil jako „kniha nenalezena“. Když všechny
// zdroje odpověděly a shodly se na prázdnu, je to prázdný seznam bez chyby.
func (c *Chain) Search(ctx context.Context, query string) ([]SearchResult, error) {
	var firstErr error

	for _, p := range c.providers {
		results, err := p.Search(ctx, query)
		switch {
		case err != nil:
			// Cizí weby se rozbíjejí; zalogujeme a jdeme na další zdroj.
			slog.Warn("zdroj metadat selhal při hledání", "zdroj", p.Name(), "err", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", p.Name(), err)
			}
		case len(results) > 0:
			for i := range results {
				results[i].Source = p.Name()
			}
			return results, nil
		default:
			slog.Debug("zdroj metadat nic nenašel", "zdroj", p.Name(), "dotaz", query)
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

// Supports vrátí true, pokud URL zvládne některý z nakonfigurovaných zdrojů.
func (c *Chain) Supports(rawURL string) bool {
	for _, p := range c.providers {
		if p.Supports(rawURL) {
			return true
		}
	}
	return false
}

// FetchByURL předá požadavek tomu zdroji, který URL umí. Tady se fallback
// nedělá – adresa jednoznačně určuje zdroj.
func (c *Chain) FetchByURL(ctx context.Context, rawURL string) (*BookMetadata, error) {
	for _, p := range c.providers {
		if !p.Supports(rawURL) {
			continue
		}
		meta, err := p.FetchByURL(ctx, rawURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name(), err)
		}
		meta.Source = p.Name()
		return meta, nil
	}
	return nil, ErrNoProvider
}

// HostMatches ověří, že URL má http(s) schéma a hostitele host (volitelně
// s předponou "www."). Poskytovatelé jím implementují Supports.
func HostMatches(rawURL, host string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return strings.EqualFold(u.Hostname(), host) ||
		strings.EqualFold(u.Hostname(), "www."+host)
}
