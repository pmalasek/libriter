// Package tmdb je klient The Movie Database (API v3) pro metadata filmů a
// seriálů. Podmínky TMDB vyžadují uvést zdroj: „This product uses the TMDB
// API but is not endorsed or certified by TMDB.“ – uvádí to webové rozhraní.
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL  = "https://api.themoviedb.org/3"
	defaultImageURL = "https://image.tmdb.org/t/p"
	// minInterval drží klienta pod limitem TMDB (~50 požadavků za sekundu)
	// s velkou rezervou – první sken knihovny jinak pošle stovky dotazů naráz.
	minInterval = 50 * time.Millisecond
)

// ErrNoKey znamená, že klíč TMDB není nastavený.
var ErrNoKey = errors.New("TMDB_API_KEY není nastaven")

// ErrNotFound vrací TMDB na neexistující ID.
var ErrNotFound = errors.New("tmdb: nenalezeno")

// Client volá TMDB API.
type Client struct {
	key      string
	language string
	baseURL  string
	imageURL string
	http     *http.Client

	mu   sync.Mutex
	last time.Time
}

// New vytvoří klienta. key je API klíč v3, nebo čtecí token v4 (JWT, začíná
// „eyJ“) – ten se posílá v hlavičce Authorization.
func New(key, language string) *Client {
	if language == "" {
		language = "cs-CZ"
	}
	return &Client{
		key:      strings.TrimSpace(key),
		language: language,
		baseURL:  defaultBaseURL,
		imageURL: defaultImageURL,
		http:     &http.Client{Timeout: 20 * time.Second},
	}
}

// WithBaseURL přesměruje klienta (testy).
func (c *Client) WithBaseURL(api, images string) *Client {
	c.baseURL, c.imageURL = strings.TrimRight(api, "/"), strings.TrimRight(images, "/")
	return c
}

// Enabled řekne, jestli je klíč nastavený.
func (c *Client) Enabled() bool { return c != nil && c.key != "" }

// Language vrátí jazyk metadat (např. cs-CZ).
func (c *Client) Language() string { return c.language }

func (c *Client) wait(ctx context.Context) error {
	c.mu.Lock()
	next := c.last.Add(minInterval)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	c.last = next
	c.mu.Unlock()
	select {
	case <-time.After(time.Until(next)):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	if !c.Enabled() {
		return ErrNoKey
	}
	if err := c.wait(ctx); err != nil {
		return err
	}
	if q == nil {
		q = url.Values{}
	}
	bearer := strings.HasPrefix(c.key, "eyJ")
	if !bearer {
		q.Set("api_key", c.key)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if bearer {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("tmdb %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("tmdb %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return fmt.Errorf("tmdb %s: %w", path, err)
	}
	return nil
}

// SearchResult je výsledek hledání filmu nebo seriálu.
type SearchResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Year          int     `json:"year,omitempty"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path,omitempty"`
	Popularity    float64 `json:"popularity"`
}

type searchResponse struct {
	Results []struct {
		ID            int     `json:"id"`
		Title         string  `json:"title"`
		Name          string  `json:"name"`
		OriginalTitle string  `json:"original_title"`
		OriginalName  string  `json:"original_name"`
		ReleaseDate   string  `json:"release_date"`
		FirstAirDate  string  `json:"first_air_date"`
		Overview      string  `json:"overview"`
		PosterPath    string  `json:"poster_path"`
		Popularity    float64 `json:"popularity"`
	} `json:"results"`
}

func (r searchResponse) normalize() []SearchResult {
	out := make([]SearchResult, 0, len(r.Results))
	for _, x := range r.Results {
		out = append(out, SearchResult{
			ID:            x.ID,
			Title:         firstNonEmpty(x.Title, x.Name),
			OriginalTitle: firstNonEmpty(x.OriginalTitle, x.OriginalName),
			Year:          yearOf(firstNonEmpty(x.ReleaseDate, x.FirstAirDate)),
			Overview:      x.Overview,
			PosterPath:    x.PosterPath,
			Popularity:    x.Popularity,
		})
	}
	return out
}

// SearchMovie hledá film; year 0 = bez omezení roku.
func (c *Client) SearchMovie(ctx context.Context, query string, year int) ([]SearchResult, error) {
	q := url.Values{"query": {query}, "language": {c.language}, "include_adult": {"false"}}
	if year > 0 {
		q.Set("year", strconv.Itoa(year))
	}
	var r searchResponse
	if err := c.get(ctx, "/search/movie", q, &r); err != nil {
		return nil, err
	}
	return r.normalize(), nil
}

// SearchTV hledá seriál; year 0 = bez omezení roku první epizody.
func (c *Client) SearchTV(ctx context.Context, query string, year int) ([]SearchResult, error) {
	q := url.Values{"query": {query}, "language": {c.language}, "include_adult": {"false"}}
	if year > 0 {
		q.Set("first_air_date_year", strconv.Itoa(year))
	}
	var r searchResponse
	if err := c.get(ctx, "/search/tv", q, &r); err != nil {
		return nil, err
	}
	return r.normalize(), nil
}

// Genre je žánr TMDB.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Person je člen obsazení nebo štábu.
type Person struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Job       string `json:"job"`
	Order     int    `json:"order"`
}

type credits struct {
	Cast []Person `json:"cast"`
	Crew []Person `json:"crew"`
}

// Details jsou podrobnosti filmu nebo seriálu.
type Details struct {
	ID            int
	IMDBID        string
	Title         string
	OriginalTitle string
	Overview      string
	Tagline       string
	Status        string
	Year          int
	RuntimeMin    int
	Rating        float64
	PosterPath    string
	BackdropPath  string
	Genres        []Genre
	Cast          []Person // seřazení podle pořadí v titulcích
	Directors     []string
	Writers       []string
	Creators      []string
	Seasons       []int // čísla řad (seriál)
}

type detailsResponse struct {
	ID             int                     `json:"id"`
	IMDBID         string                  `json:"imdb_id"`
	Title          string                  `json:"title"`
	Name           string                  `json:"name"`
	OriginalTitle  string                  `json:"original_title"`
	OriginalName   string                  `json:"original_name"`
	Overview       string                  `json:"overview"`
	Tagline        string                  `json:"tagline"`
	Status         string                  `json:"status"`
	ReleaseDate    string                  `json:"release_date"`
	FirstAirDate   string                  `json:"first_air_date"`
	Runtime        int                     `json:"runtime"`
	EpisodeRunTime []int                   `json:"episode_run_time"`
	VoteAverage    float64                 `json:"vote_average"`
	PosterPath     string                  `json:"poster_path"`
	BackdropPath   string                  `json:"backdrop_path"`
	Genres         []Genre                 `json:"genres"`
	Credits        credits                 `json:"credits"`
	CreatedBy      []struct{ Name string } `json:"created_by"`
	Seasons        []struct {
		SeasonNumber int `json:"season_number"`
	} `json:"seasons"`
}

func (r detailsResponse) normalize() *Details {
	d := &Details{
		ID: r.ID, IMDBID: r.IMDBID,
		Title:         firstNonEmpty(r.Title, r.Name),
		OriginalTitle: firstNonEmpty(r.OriginalTitle, r.OriginalName),
		Overview:      r.Overview, Tagline: r.Tagline, Status: r.Status,
		Year:       yearOf(firstNonEmpty(r.ReleaseDate, r.FirstAirDate)),
		RuntimeMin: r.Runtime, Rating: r.VoteAverage,
		PosterPath: r.PosterPath, BackdropPath: r.BackdropPath, Genres: r.Genres,
	}
	if d.RuntimeMin == 0 && len(r.EpisodeRunTime) > 0 {
		d.RuntimeMin = r.EpisodeRunTime[0]
	}
	cast := r.Credits.Cast
	if len(cast) > 15 {
		cast = cast[:15]
	}
	d.Cast = cast
	for _, p := range r.Credits.Crew {
		switch p.Job {
		case "Director":
			d.Directors = append(d.Directors, p.Name)
		case "Screenplay", "Writer":
			if len(d.Writers) < 4 {
				d.Writers = append(d.Writers, p.Name)
			}
		}
	}
	for _, c := range r.CreatedBy {
		d.Creators = append(d.Creators, c.Name)
	}
	for _, s := range r.Seasons {
		d.Seasons = append(d.Seasons, s.SeasonNumber)
	}
	return d
}

// Movie vrátí podrobnosti filmu. Prázdný popis v jazyce knihovny se doplní
// z angličtiny.
func (c *Client) Movie(ctx context.Context, id int) (*Details, error) {
	return c.details(ctx, "/movie/"+strconv.Itoa(id))
}

// TV vrátí podrobnosti seriálu.
func (c *Client) TV(ctx context.Context, id int) (*Details, error) {
	return c.details(ctx, "/tv/"+strconv.Itoa(id))
}

func (c *Client) details(ctx context.Context, path string) (*Details, error) {
	var r detailsResponse
	if err := c.get(ctx, path, url.Values{"language": {c.language}, "append_to_response": {"credits"}}, &r); err != nil {
		return nil, err
	}
	d := r.normalize()
	if d.Overview == "" && !strings.HasPrefix(c.language, "en") {
		var en detailsResponse
		if err := c.get(ctx, path, url.Values{"language": {"en-US"}}, &en); err == nil {
			d.Overview = en.Overview
			if d.Tagline == "" {
				d.Tagline = en.Tagline
			}
		}
	}
	return d, nil
}

// Episode jsou metadata dílu.
type Episode struct {
	Number    int     `json:"episode_number"`
	Name      string  `json:"name"`
	Overview  string  `json:"overview"`
	AirDate   string  `json:"air_date"`
	StillPath string  `json:"still_path"`
	Runtime   int     `json:"runtime"`
	VoteAvg   float64 `json:"vote_average"`
}

// Season jsou metadata řady.
type Season struct {
	Number     int       `json:"season_number"`
	Name       string    `json:"name"`
	Overview   string    `json:"overview"`
	PosterPath string    `json:"poster_path"`
	Episodes   []Episode `json:"episodes"`
}

// Season vrátí řadu seriálu i s díly.
func (c *Client) Season(ctx context.Context, tvID, number int) (*Season, error) {
	var s Season
	path := fmt.Sprintf("/tv/%d/season/%d", tvID, number)
	if err := c.get(ctx, path, url.Values{"language": {c.language}}, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ImageSize jsou velikosti obrázků, které Sceniter stahuje.
const (
	SizePoster   = "w500"
	SizeBackdrop = "w1280"
	SizeStill    = "w300"
)

// Image stáhne obrázek (path z TMDB, např. „/abc.jpg“) v dané velikosti.
func (c *Client) Image(ctx context.Context, size, path string) ([]byte, string, error) {
	if path == "" {
		return nil, "", ErrNotFound
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.imageURL+"/"+size+path, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("tmdb obrázek %s: HTTP %d", path, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	return data, resp.Header.Get("Content-Type"), err
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}
