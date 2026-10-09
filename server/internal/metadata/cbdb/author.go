package cbdb

// Metadata autorů z cbdb.cz.
//
// Struktura, na které stojíme:
//   - hledání: /hledat?text=<dotaz>, odkazy tvaru "autor-<id>-<slug>"
//   - detail:  JSON-LD schema.org/Person (jméno + životopis), fotka
//     v <meta property="og:image">, záložně <div class="author_lifestory">

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"libriter/internal/metadata"
	"libriter/internal/metadata/htmlutil"

	"golang.org/x/net/html"
)

var (
	// Životopis na webu začíná nadpisem "Životopis - <jméno>:", ten do textu nepatří.
	lifestoryPrefixRe = regexp.MustCompile(`^Životopis\s*-\s*[^:]*:\s*`)
	// Patička s tím, kdo záznam založil – ovládací text, ne obsah.
	createdBySuffixRe = regexp.MustCompile(`\s*\((?:Založil|Založila|Založil/a)\s*:[^)]*\)\s*$`)
)

func (c *Client) SupportsAuthorURL(rawURL string) bool {
	return metadata.HostMatches(rawURL, host) && authorHrefRe.MatchString(rawURL)
}

// SupportsImageURL – fotky autorů jsou na témže hostiteli (/img.php?...).
func (c *Client) SupportsImageURL(rawURL string) bool {
	return metadata.HostMatches(rawURL, host)
}

func (c *Client) SearchAuthors(ctx context.Context, query string) ([]metadata.AuthorSearchResult, error) {
	searchURL := fmt.Sprintf("%s/hledat?text=%s&ok=", baseURL, url.QueryEscape(query))

	doc, err := c.fetch(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search authors: %w", err)
	}
	return parseAuthorSearchResults(doc), nil
}

// parseAuthorSearchResults čte jen blok #search_result_box_authors.
//
// Stránka s výsledky má bloky pro knihy, autory, série i uživatele a odkazy
// na autory jsou i u knih (jako jejich autoři). Bez omezení na správný blok
// by hledání vracelo autory prvních nalezených knih, ne hledané jméno.
func parseAuthorSearchResults(doc *html.Node) []metadata.AuthorSearchResult {
	box := htmlutil.ByID(doc, "search_result_box_authors")
	if box == nil {
		return nil
	}

	var results []metadata.AuthorSearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(box, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "div" || !htmlutil.HasClass(n, "search_graphic_box") {
			return true
		}

		result, ok := parseAuthorSearchRow(n)
		if ok && !seen[result.ID] {
			seen[result.ID] = true
			results = append(results, result)
		}
		return false
	})

	return results
}

// parseAuthorSearchRow přečte jednoho autora ve výsledcích. Kromě jména nese
// řádek i datum narození ("*09.01.1890"), které odliší jmenovce.
func parseAuthorSearchRow(row *html.Node) (metadata.AuthorSearchResult, bool) {
	result := metadata.AuthorSearchResult{Source: providerName}

	htmlutil.Walk(row, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "a" || result.Name != "" {
			return true
		}

		href := htmlutil.Attr(n, "href")
		match := authorHrefRe.FindStringSubmatch(href)
		if match == nil {
			return true
		}
		// Odkaz s fotkou vede na tutéž adresu, ale text nenese.
		name := htmlutil.Collapse(htmlutil.Text(n))
		if name == "" {
			return true
		}

		result.ID, _ = strconv.Atoi(match[1])
		result.Name = name
		result.URL = htmlutil.ResolveURL(baseURL, href)
		return true
	})

	if result.Name != "" {
		rowText := htmlutil.Collapse(htmlutil.Text(row))
		result.BirthYear = metadata.YearFromDate(strings.TrimPrefix(rowText, result.Name))
		if result.BirthYear > 0 {
			result.Note = fmt.Sprintf("* %d", result.BirthYear)
		}
	}

	return result, result.ID > 0 && result.Name != ""
}

func (c *Client) FetchAuthorByURL(ctx context.Context, rawURL string) (*metadata.AuthorMetadata, error) {
	doc, err := c.fetch(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch author: %w", err)
	}

	meta := parseAuthorPage(doc)
	if meta.Name == "" {
		return nil, fmt.Errorf("jméno autora nenalezeno na stránce %s", rawURL)
	}

	if match := authorHrefRe.FindStringSubmatch(rawURL); match != nil {
		meta.ID, _ = strconv.Atoi(match[1])
	}
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

	// Životopis v HTML bývá delší než description v JSON-LD.
	lifestory := findLifestory(doc)
	if lifestory != "" {
		meta.Bio = cleanBio(lifestoryPrefixRe.ReplaceAllString(lifestory, ""))
	}

	// Roky se odhadnou z textu, když je strukturovaná data nemají. Čte se
	// celý životopis včetně nadpisu – rozmezí bývá právě v něm
	// ("Životopis - Karel Čapek (1890–1938):").
	if meta.BirthYear == 0 {
		source := lifestory
		if source == "" {
			source = meta.Bio
		}
		meta.BirthYear, meta.DeathYear = metadata.YearsFromText(source)
	}

	return meta
}

// cleanBio odstraní patičky, které patří webu, ne autorovi.
func cleanBio(text string) string {
	return strings.TrimSpace(createdBySuffixRe.ReplaceAllString(strings.TrimSpace(text), ""))
}

// findLifestory vrátí text bloku se životopisem včetně úvodního nadpisu.
func findLifestory(doc *html.Node) string {
	node := htmlutil.Find(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" && htmlutil.HasClass(n, "author_lifestory")
	})
	if node == nil {
		return ""
	}
	return htmlutil.Collapse(htmlutil.Text(node))
}
