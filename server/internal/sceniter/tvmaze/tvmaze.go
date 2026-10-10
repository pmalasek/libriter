// Package tvmaze je klient TVmaze (api.tvmaze.com) – veřejné API seriálů bez
// klíče. Metadata jsou anglicky; nese ID IMDb a TheTVDB.
package tvmaze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"mediater/internal/core/htmlutil"
	"mediater/internal/core/plugin"
)

const defaultBaseURL = "https://api.tvmaze.com"

// ErrNotFound vrací TVmaze na neexistující seriál nebo řadu.
var ErrNotFound = errors.New("tvmaze: nenalezeno")

// ErrRateLimited znamená překročení limitu (20 požadavků za 10 s).
var ErrRateLimited = errors.New("tvmaze: příliš mnoho požadavků")

// Client volá TVmaze API.
type Client struct {
	baseURL string
	http    *http.Client
}

func New() *Client {
	return &Client{baseURL: defaultBaseURL, http: &http.Client{Timeout: 20 * time.Second}}
}

// WithBaseURL přesměruje klienta (testy).
func (c *Client) WithBaseURL(base string) *Client {
	c.baseURL = strings.TrimRight(base, "/")
	return c
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mediater/1.0 (osobni-videoknihovna)")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tvmaze %s: %w", path, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return fmt.Errorf("tvmaze %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}

type image struct {
	Medium   string `json:"medium"`
	Original string `json:"original"`
}

type show struct {
	ID             int      `json:"id"`
	URL            string   `json:"url"`
	Name           string   `json:"name"`
	Genres         []string `json:"genres"`
	Status         string   `json:"status"`
	Runtime        int      `json:"runtime"`
	AverageRuntime int      `json:"averageRuntime"`
	Premiered      string   `json:"premiered"`
	Rating         struct {
		Average float64 `json:"average"`
	} `json:"rating"`
	Externals struct {
		IMDB    string `json:"imdb"`
		TheTVDB int    `json:"thetvdb"`
	} `json:"externals"`
	Image    *image `json:"image"`
	Summary  string `json:"summary"`
	Embedded struct {
		Cast []struct {
			Person    person                `json:"person"`
			Character struct{ Name string } `json:"character"`
		} `json:"cast"`
		Crew []struct {
			Type   string `json:"type"`
			Person person `json:"person"`
		} `json:"crew"`
		Seasons []season `json:"seasons"`
	} `json:"_embedded"`
}

// person je osoba z obsazení nebo štábu (embed cast/crew).
type person struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Image *image `json:"image"`
}

func (p person) plugin() plugin.Person {
	out := plugin.Person{Name: p.Name}
	if p.ID != 0 {
		out.ID = strconv.Itoa(p.ID)
	}
	if p.Image != nil {
		out.ImageURL = p.Image.Medium
	}
	return out
}

type season struct {
	ID      int    `json:"id"`
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Image   *image `json:"image"`
}

type episode struct {
	Number  int    `json:"number"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Airdate string `json:"airdate"`
	Runtime int    `json:"runtime"`
	Image   *image `json:"image"`
}

func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

// text převede HTML souhrn na prostý text.
func text(s string) string {
	if s == "" {
		return ""
	}
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return s
	}
	return htmlutil.Collapse(htmlutil.Text(doc))
}

func (i *image) url(original bool) string {
	if i == nil {
		return ""
	}
	if original && i.Original != "" {
		return i.Original
	}
	if i.Medium != "" {
		return i.Medium
	}
	return i.Original
}

// Search hledá seriál podle názvu.
func (c *Client) Search(ctx context.Context, query string) ([]plugin.SearchResult, error) {
	var res []struct {
		Show show `json:"show"`
	}
	if err := c.get(ctx, "/search/shows?q="+url.QueryEscape(query), &res); err != nil {
		return nil, err
	}
	out := make([]plugin.SearchResult, 0, len(res))
	for _, r := range res {
		out = append(out, plugin.SearchResult{
			ID: strconv.Itoa(r.Show.ID), URL: r.Show.URL, Title: r.Show.Name,
			Year: year(r.Show.Premiered), Note: strings.Join(r.Show.Genres, ", "),
			ImageURL: r.Show.Image.url(false),
		})
	}
	return out, nil
}

// Show vrátí detail seriálu s obsazením, tvůrci a řadami.
func (c *Client) Show(ctx context.Context, id int) (*plugin.VideoMetadata, error) {
	var s show
	if err := c.get(ctx, fmt.Sprintf("/shows/%d?embed[]=cast&embed[]=crew&embed[]=seasons", id), &s); err != nil {
		return nil, err
	}
	runtime := s.AverageRuntime
	if runtime == 0 {
		runtime = s.Runtime
	}
	d := &plugin.VideoMetadata{
		ID: strconv.Itoa(s.ID), URL: s.URL, Title: s.Name, Year: year(s.Premiered),
		Overview: text(s.Summary), Status: s.Status, RuntimeMin: runtime,
		Rating:      int(math.Round(s.Rating.Average * 10)),
		Genres:      s.Genres,
		PosterURL:   s.Image.url(true),
		ExternalIDs: map[string]string{"tvmaze": strconv.Itoa(s.ID)},
	}
	if s.Externals.IMDB != "" {
		d.ExternalIDs["imdb"] = s.Externals.IMDB
	}
	if s.Externals.TheTVDB != 0 {
		d.ExternalIDs["thetvdb"] = strconv.Itoa(s.Externals.TheTVDB)
	}
	for i, p := range s.Embedded.Cast {
		if i >= 15 {
			break
		}
		pp := p.Person.plugin()
		pp.Character = p.Character.Name
		d.Credits.Cast = append(d.Credits.Cast, pp)
	}
	for _, p := range s.Embedded.Crew {
		if p.Type == "Creator" {
			d.Credits.Creators = append(d.Credits.Creators, p.Person.plugin())
		}
	}
	for _, se := range s.Embedded.Seasons {
		d.Seasons = append(d.Seasons, se.Number)
	}
	return d, nil
}

// Season vrátí řadu seriálu i s díly.
func (c *Client) Season(ctx context.Context, showID, number int) (*plugin.SeasonMetadata, error) {
	var seasons []season
	if err := c.get(ctx, fmt.Sprintf("/shows/%d/seasons", showID), &seasons); err != nil {
		return nil, err
	}
	for _, se := range seasons {
		if se.Number != number {
			continue
		}
		var eps []episode
		if err := c.get(ctx, fmt.Sprintf("/seasons/%d/episodes", se.ID), &eps); err != nil {
			return nil, err
		}
		out := &plugin.SeasonMetadata{Number: se.Number, Name: se.Name, Overview: text(se.Summary),
			PosterURL: se.Image.url(true), Episodes: []plugin.EpisodeMetadata{}}
		for _, e := range eps {
			if e.Number == 0 {
				continue // speciály bez čísla
			}
			out.Episodes = append(out.Episodes, plugin.EpisodeMetadata{
				Number: e.Number, Title: e.Name, Overview: text(e.Summary), AirDate: e.Airdate,
				StillURL: e.Image.url(false), RuntimeMin: e.Runtime,
			})
		}
		return out, nil
	}
	return nil, ErrNotFound
}
