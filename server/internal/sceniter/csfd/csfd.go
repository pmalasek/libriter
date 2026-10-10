// Package csfd čte metadata filmů a seriálů z ČSFD (csfd.cz).
//
// ČSFD nemá veřejné API, takže jde o scraping. Z čeho se čte:
//   - hledání: /hledat/?q=<dotaz>, sekce #films a #series s články
//     a.film-title-name, rok a typ v span.film-title-info;
//   - detail: JSON-LD schema.org Movie/TVSeries (název, rok, délka, režie,
//     herci, hodnocení v %, plakát), žánry v div.genres, děj v div.plot-full,
//     scénář v div.creators, původní název v ul.film-names;
//   - řady a díly: div.film-episodes-list – na stránce seriálu řady
//     („Série 1“) nebo rovnou díly („(S01E02)“), na stránce řady díly.
//
// Web je za ochranou proti botům (Anubis), která zkouší prohlížeče; klient
// se proto představuje vlastním User-Agentem, ne jako prohlížeč.
package csfd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"mediater/internal/core/htmlutil"
	"mediater/internal/core/plugin"
)

const (
	defaultBaseURL = "https://www.csfd.cz"
	userAgent      = "Mediater/1.0 (osobni-videoknihovna; +https://github.com/libriter)"
	// minDelay je odstup mezi požadavky uvnitř jednoho volání (řada = stránka
	// seriálu + stránka řady). Mezi voláními drží odstup server.
	minDelay = time.Second
	maxBody  = 4 << 20
	maxCast  = 15
)

// ErrNotFound vrací ČSFD na neexistující záznam.
var ErrNotFound = errors.New("csfd: nenalezeno")

var (
	filmHrefRe   = regexp.MustCompile(`^/film/(\d+)-[^/]*/(?:prehled/)?$`)
	filmIDRe     = regexp.MustCompile(`/film/(\d+)(?:-[^/]*)?/`)
	yearRe       = regexp.MustCompile(`\b(1[89]\d\d|20\d\d)\b`)
	episodeRe    = regexp.MustCompile(`\(S(\d+)E(\d+)\)|\(E(\d+)\)`)
	seasonNameRe = regexp.MustCompile(`(?i)^(?:série|season|řada|sezóna)\s*(\d+)`)
	durationRe   = regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?`)
)

// Client čte ČSFD.
type Client struct {
	baseURL string
	http    *http.Client

	mu   sync.Mutex
	last time.Time
}

func New() *Client {
	return &Client{baseURL: defaultBaseURL, http: &http.Client{Timeout: 20 * time.Second}}
}

// WithBaseURL přesměruje klienta (testy).
func (c *Client) WithBaseURL(base string) *Client {
	c.baseURL = strings.TrimRight(base, "/")
	return c
}

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := time.Until(c.last.Add(minDelay)); d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	c.last = time.Now()
	return nil
}

func (c *Client) get(ctx context.Context, path string) (*html.Node, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "cs,en;q=0.5")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("csfd %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, &StatusError{Status: resp.StatusCode}
	case resp.StatusCode != http.StatusOK:
		return nil, &StatusError{Status: resp.StatusCode}
	}
	doc, err := html.Parse(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("csfd %s: %w", path, err)
	}
	// Ochrana proti botům vrací 200 se stránkou výzvy.
	if t := htmlutil.Element(doc, "title"); t != nil && strings.Contains(htmlutil.Text(t), "not a bot") {
		return nil, &StatusError{Status: http.StatusTooManyRequests}
	}
	return doc, nil
}

// StatusError je odpověď s jiným stavem než 200.
type StatusError struct{ Status int }

func (e *StatusError) Error() string { return fmt.Sprintf("csfd: HTTP %d", e.Status) }

// Search hledá film (kind movie) nebo seriál (kind show).
func (c *Client) Search(ctx context.Context, kind, query string) ([]plugin.SearchResult, error) {
	doc, err := c.get(ctx, "/hledat/?q="+url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	return ParseSearch(doc, kind, c.baseURL), nil
}

// ParseSearch vytáhne výsledky hledání pro daný druh. U seriálu se za
// výsledky ze sekce seriálů přidají i ty ze sekce filmů: ČSFD vede některé
// minisérie jako „TV film“ (s délkou 4×47 min), seriálem pak nejsou.
func ParseSearch(doc *html.Node, kind, base string) []plugin.SearchResult {
	out := parseSearchSection(doc, "films", base)
	if kind == plugin.KindShow {
		seen := map[string]bool{}
		series := parseSearchSection(doc, "series", base)
		for _, r := range series {
			seen[r.ID] = true
		}
		for _, r := range out {
			if !seen[r.ID] {
				series = append(series, r)
			}
		}
		out = series
	}
	return out
}

func parseSearchSection(doc *html.Node, section, base string) []plugin.SearchResult {
	box := htmlutil.Find(doc, func(n *html.Node) bool {
		return n.Data == "section" && htmlutil.Attr(n, "data-search-results") == section
	})
	out := []plugin.SearchResult{}
	if box == nil {
		return out
	}
	htmlutil.Walk(box, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "article" {
			return true
		}
		a := htmlutil.Find(n, func(n *html.Node) bool { return n.Data == "a" && htmlutil.HasClass(n, "film-title-name") })
		if a == nil {
			return false
		}
		href := htmlutil.Attr(a, "href")
		m := filmHrefRe.FindStringSubmatch(href)
		if m == nil {
			return false // řada nebo díl seriálu – páruje se jen celý seriál
		}
		r := plugin.SearchResult{ID: m[1], URL: base + href, Title: htmlutil.Collapse(htmlutil.Text(a))}
		var types []string
		if info := htmlutil.Find(n, func(n *html.Node) bool { return htmlutil.HasClass(n, "film-title-info") }); info != nil {
			htmlutil.Walk(info, func(x *html.Node) bool {
				if x.Type == html.ElementNode && htmlutil.HasClass(x, "info") {
					t := strings.Trim(htmlutil.Collapse(htmlutil.Text(x)), "()")
					if y, err := strconv.Atoi(t); err == nil && r.Year == 0 {
						r.Year = y
					} else if t != "" {
						types = append(types, t)
					}
					return false
				}
				return true
			})
		}
		var notes []string
		if g := htmlutil.Find(n, func(n *html.Node) bool { return htmlutil.HasClass(n, "film-origins-genres") }); g != nil {
			notes = append(notes, htmlutil.Collapse(htmlutil.Text(g)))
		}
		notes = append(notes, types...)
		r.Note = strings.Join(notes, " · ")
		if img := htmlutil.Element(n, "img"); img != nil && !htmlutil.HasClass(img, "empty-image") {
			r.ImageURL = absURL(htmlutil.Attr(img, "src"))
		}
		out = append(out, r)
		return false
	})
	return out
}

func absURL(src string) string {
	switch {
	case src == "" || strings.HasPrefix(src, "data:"):
		return ""
	case strings.HasPrefix(src, "//"):
		return "https:" + src
	}
	return src
}

// IDFromURL vrátí číselné ID z adresy stránky na ČSFD.
func IDFromURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	m := filmIDRe.FindStringSubmatch(u.Path + "/")
	if m == nil {
		return "", false
	}
	return m[1], true
}

// Film stáhne detail filmu nebo seriálu.
func (c *Client) Film(ctx context.Context, id string) (*plugin.VideoMetadata, error) {
	doc, err := c.get(ctx, "/film/"+id+"/prehled/")
	if err != nil {
		return nil, err
	}
	d := ParseFilm(doc)
	d.ID = id
	d.URL = c.baseURL + "/film/" + id + "/prehled/"
	d.ExternalIDs = map[string]string{"csfd": id}
	return d, nil
}

type jsonLD struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	DateCreated string `json:"dateCreated"`
	Duration    string `json:"duration"`
	Image       string `json:"image"`
	Director    []struct {
		Name string `json:"name"`
	} `json:"director"`
	Actor []struct {
		Name string `json:"name"`
	} `json:"actor"`
	AggregateRating *struct {
		RatingValue float64 `json:"ratingValue"`
		BestRating  float64 `json:"bestRating"`
	} `json:"aggregateRating"`
}

func findJSONLD(doc *html.Node) *jsonLD {
	var out *jsonLD
	htmlutil.Walk(doc, func(n *html.Node) bool {
		if out != nil {
			return false
		}
		if n.Type == html.ElementNode && n.Data == "script" && htmlutil.Attr(n, "type") == "application/ld+json" {
			var ld jsonLD
			if err := json.Unmarshal([]byte(htmlutil.Text(n)), &ld); err == nil && ld.Name != "" {
				out = &ld
			}
			return false
		}
		return true
	})
	return out
}

// ParseFilm vytáhne metadata ze stránky filmu nebo seriálu.
func ParseFilm(doc *html.Node) *plugin.VideoMetadata {
	d := &plugin.VideoMetadata{}
	if ld := findJSONLD(doc); ld != nil {
		d.Title = ld.Name
		if m := yearRe.FindString(ld.DateCreated); m != "" {
			d.Year, _ = strconv.Atoi(m)
		}
		if m := durationRe.FindStringSubmatch(ld.Duration); m != nil {
			h, _ := strconv.Atoi(m[1])
			mins, _ := strconv.Atoi(m[2])
			d.RuntimeMin = h*60 + mins
		}
		d.PosterURL = absURL(ld.Image)
		for _, p := range ld.Director {
			d.Credits.Directors = append(d.Credits.Directors, p.Name)
		}
		if r := ld.AggregateRating; r != nil && r.RatingValue > 0 {
			best := r.BestRating
			if best <= 0 {
				best = 100
			}
			d.Rating = int(math.Round(r.RatingValue * 100 / best))
		}
	}
	header := htmlutil.Find(doc, func(n *html.Node) bool { return htmlutil.HasClass(n, "film-header-name") })
	if d.Title == "" && header != nil {
		if h1 := htmlutil.Element(header, "h1"); h1 != nil {
			d.Title = htmlutil.Collapse(htmlutil.Text(h1))
		}
	}

	if g := htmlutil.Find(doc, func(n *html.Node) bool { return htmlutil.HasClass(n, "genres") }); g != nil {
		htmlutil.Walk(g, func(n *html.Node) bool {
			if n.Type == html.ElementNode && n.Data == "a" {
				d.Genres = append(d.Genres, htmlutil.Collapse(htmlutil.Text(n)))
				return false
			}
			return true
		})
	}

	country := ""
	if o := htmlutil.Find(doc, func(n *html.Node) bool { return htmlutil.HasClass(n, "origin") && n.Data == "div" }); o != nil {
		text := htmlutil.Collapse(htmlutil.Text(o))
		if i := strings.IndexAny(text, ",0123456789"); i > 0 {
			country = strings.TrimSpace(text[:i])
		}
		country = strings.TrimSpace(strings.Split(country, "/")[0])
		if d.Year == 0 {
			if m := yearRe.FindString(text); m != "" {
				d.Year, _ = strconv.Atoi(m)
			}
		}
	}
	if header != nil && country != "" {
		d.OriginalTitle = originalTitle(header, country)
		if d.OriginalTitle == d.Title {
			d.OriginalTitle = ""
		}
	}

	plot := htmlutil.Find(doc, func(n *html.Node) bool { return htmlutil.HasClass(n, "plot-full") })
	if plot == nil {
		plot = htmlutil.Find(doc, func(n *html.Node) bool { return htmlutil.HasClass(n, "plot-preview") })
	}
	if plot != nil {
		d.Overview = htmlutil.Collapse(htmlutil.TextSkipping(plot, func(n *html.Node) bool {
			return htmlutil.HasClass(n, "span-more-small") || htmlutil.HasClass(n, "more")
		}))
	}

	if cr := htmlutil.ByID(doc, "creators"); cr != nil {
		for c := cr.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode || c.Data != "div" {
				continue
			}
			h4 := htmlutil.Element(c, "h4")
			if h4 == nil {
				continue
			}
			role := strings.TrimSuffix(htmlutil.Collapse(htmlutil.Text(h4)), ":")
			var names []string
			htmlutil.Walk(c, func(n *html.Node) bool {
				if n.Type == html.ElementNode && n.Data == "a" && strings.HasPrefix(htmlutil.Attr(n, "href"), "/tvurce/") {
					names = append(names, htmlutil.Collapse(htmlutil.Text(n)))
					return false
				}
				return true
			})
			switch role {
			case "Režie":
				if len(d.Credits.Directors) == 0 {
					d.Credits.Directors = names
				}
			case "Scénář":
				d.Credits.Writers = names
			case "Tvůrci", "Námět":
				if len(d.Credits.Creators) == 0 {
					d.Credits.Creators = names
				}
			case "Hrají":
				for _, n := range names {
					if len(d.Credits.Cast) >= maxCast {
						break
					}
					d.Credits.Cast = append(d.Credits.Cast, plugin.CastMember{Name: n})
				}
			}
		}
	}

	for _, e := range parseEpisodeList(doc) {
		if !containsInt(d.Seasons, e.season) {
			d.Seasons = append(d.Seasons, e.season)
		}
	}
	return d
}

// originalTitle najde v ul.film-names název pro zemi původu.
func originalTitle(header *html.Node, country string) string {
	out := ""
	htmlutil.Walk(header, func(n *html.Node) bool {
		if out != "" {
			return false
		}
		if n.Type == html.ElementNode && n.Data == "li" {
			img := htmlutil.Element(n, "img")
			if img != nil && strings.EqualFold(htmlutil.Attr(img, "title"), country) {
				out = htmlutil.Collapse(htmlutil.TextSkipping(n, func(x *html.Node) bool { return x.Data == "span" }))
			}
			return false
		}
		return true
	})
	return out
}

func containsInt(list []int, n int) bool {
	for _, v := range list {
		if v == n {
			return true
		}
	}
	return false
}

// listEntry je položka div.film-episodes-list: řada (season > 0, episode 0,
// href na stránku řady) nebo díl.
type listEntry struct {
	season, episode int
	title, href     string
}

func parseEpisodeList(doc *html.Node) []listEntry {
	list := htmlutil.Find(doc, func(n *html.Node) bool { return htmlutil.HasClass(n, "film-episodes-list") })
	var out []listEntry
	if list == nil {
		return out
	}
	htmlutil.Walk(list, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "li" {
			return true
		}
		a := htmlutil.Find(n, func(n *html.Node) bool { return n.Data == "a" && htmlutil.HasClass(n, "film-title-name") })
		if a == nil {
			return false
		}
		e := listEntry{title: htmlutil.Collapse(htmlutil.Text(a)), href: htmlutil.Attr(a, "href")}
		info := htmlutil.Collapse(htmlutil.Text(n))
		if m := episodeRe.FindStringSubmatch(info); m != nil {
			if m[1] != "" {
				e.season, _ = strconv.Atoi(m[1])
				e.episode, _ = strconv.Atoi(m[2])
			} else {
				e.season = 1
				e.episode, _ = strconv.Atoi(m[3])
			}
		} else if m := seasonNameRe.FindStringSubmatch(e.title); m != nil {
			e.season, _ = strconv.Atoi(m[1])
		} else {
			return false
		}
		out = append(out, e)
		return false
	})
	return out
}

// Season stáhne řadu seriálu i s díly. Seriál, který na ČSFD řady nemá,
// vede díly rovnou na své stránce.
func (c *Client) Season(ctx context.Context, showID string, number int) (*plugin.SeasonMetadata, error) {
	doc, err := c.get(ctx, "/film/"+showID+"/prehled/")
	if err != nil {
		return nil, err
	}
	out := &plugin.SeasonMetadata{Number: number, Episodes: []plugin.EpisodeMetadata{}}
	var seasonHref string
	for _, e := range parseEpisodeList(doc) {
		switch {
		case e.episode == 0 && e.season == number:
			seasonHref, out.Name = e.href, e.title
		case e.episode > 0 && e.season == number:
			out.Episodes = append(out.Episodes, plugin.EpisodeMetadata{Number: e.episode, Title: e.title})
		}
	}
	if seasonHref == "" {
		if len(out.Episodes) == 0 {
			return nil, ErrNotFound
		}
		return out, nil
	}

	sdoc, err := c.get(ctx, seasonHref)
	if err != nil {
		return nil, err
	}
	sd := ParseFilm(sdoc)
	out.Overview, out.PosterURL = sd.Overview, sd.PosterURL
	for _, e := range parseEpisodeList(sdoc) {
		if e.episode > 0 {
			out.Episodes = append(out.Episodes, plugin.EpisodeMetadata{Number: e.episode, Title: e.title})
		}
	}
	return out, nil
}
