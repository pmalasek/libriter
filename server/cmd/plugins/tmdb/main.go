// Plugin metadat The Movie Database (TMDB) – filmy a seriály.
//
// Podmínky TMDB vyžadují uvést zdroj: „This product uses the TMDB API but is
// not endorsed or certified by TMDB.“ – uvádí to webové rozhraní.
package main

import (
	"context"
	"errors"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"mediater/internal/core/plugin"
	"mediater/internal/core/plugin/sdk"
	"mediater/internal/core/version"
	"mediater/internal/sceniter/tmdb"
)

var pageRe = regexp.MustCompile(`/(movie|tv)/(\d+)`)

func main() {
	sdk.Run(sdk.Plugin{
		Manifest: plugin.Manifest{
			Name:      "tmdb",
			Title:     "The Movie Database (TMDB)",
			Version:   version.String(),
			Kinds:     []string{plugin.KindMovie, plugin.KindShow},
			Languages: []string{"*"},
			Config: []plugin.ConfigField{{Key: "api_key", Label: "API key (v3) / Read Access Token (v4)",
				Secret: true, Required: true, Env: "TMDB_API_KEY"}},
			URLPatterns: map[string][]string{
				plugin.KindMovie: {`(?i)^https?://(www\.)?themoviedb\.org/movie/\d+`},
				plugin.KindShow:  {`(?i)^https?://(www\.)?themoviedb\.org/tv/\d+`},
			},
			ImageHosts:    []string{"image.tmdb.org"},
			MinIntervalMS: 50,
		},
		Search: search,
		Fetch:  fetch,
		Season: season,
	})
}

// locale převede jazyk knihovny (ISO 639-1) na jazyk TMDB.
func locale(lang string) string {
	switch lang = strings.ToLower(strings.TrimSpace(lang)); lang {
	case "", "cs":
		return "cs-CZ"
	case "en":
		return "en-US"
	case "sk":
		return "sk-SK"
	case "de":
		return "de-DE"
	}
	if strings.Contains(lang, "-") {
		return lang
	}
	return lang + "-" + strings.ToUpper(lang)
}

func client(req plugin.Request) (*tmdb.Client, error) {
	key := req.Config["api_key"]
	if key == "" {
		return nil, sdk.Errorf(plugin.ErrCodeUnavailable, "TMDB API klíč není nastaven")
	}
	return tmdb.New(key, locale(req.Language)), nil
}

func search(ctx context.Context, req plugin.Request) ([]plugin.SearchResult, error) {
	c, err := client(req)
	if err != nil {
		return nil, err
	}
	var res []tmdb.SearchResult
	if req.Kind == plugin.KindShow {
		res, err = c.SearchTV(ctx, req.Query.Title, req.Query.Year)
	} else {
		res, err = c.SearchMovie(ctx, req.Query.Title, req.Query.Year)
	}
	if err != nil {
		return nil, wrap(err)
	}
	out := make([]plugin.SearchResult, 0, len(res))
	for _, r := range res {
		out = append(out, plugin.SearchResult{
			ID: strconv.Itoa(r.ID), Title: r.Title, OriginalTitle: r.OriginalTitle, Year: r.Year,
			Note: r.Overview, ImageURL: c.ImageURL(tmdb.SizePoster, r.PosterPath),
			URL: pageURL(req.Kind, r.ID), Votes: r.VoteCount,
		})
	}
	return out, nil
}

func pageURL(kind string, id int) string {
	path := "movie"
	if kind == plugin.KindShow {
		path = "tv"
	}
	return "https://www.themoviedb.org/" + path + "/" + strconv.Itoa(id)
}

// idOf vezme ID z požadavku, nebo z adresy stránky na themoviedb.org.
func idOf(req plugin.Request) (int, error) {
	raw := req.ID
	if raw == "" {
		u, err := url.Parse(req.URL)
		if err != nil {
			return 0, sdk.BadRequest("neplatná adresa")
		}
		m := pageRe.FindStringSubmatch(u.Path)
		if m == nil {
			return 0, sdk.BadRequest("adresa není stránka filmu ani seriálu na TMDB")
		}
		raw = m[2]
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, sdk.BadRequest("neplatné ID %q", raw)
	}
	return id, nil
}

func fetch(ctx context.Context, req plugin.Request) (any, error) {
	c, err := client(req)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req)
	if err != nil {
		return nil, err
	}
	var d *tmdb.Details
	if req.Kind == plugin.KindShow {
		d, err = c.TV(ctx, id)
	} else {
		d, err = c.Movie(ctx, id)
	}
	if err != nil {
		return nil, wrap(err)
	}
	out := plugin.VideoMetadata{
		ID: strconv.Itoa(d.ID), URL: pageURL(req.Kind, d.ID),
		Title: d.Title, OriginalTitle: d.OriginalTitle, Year: d.Year,
		Overview: d.Overview, Tagline: d.Tagline, Status: d.Status, RuntimeMin: d.RuntimeMin,
		Rating:      int(math.Round(d.Rating * 10)),
		PosterURL:   c.ImageURL(tmdb.SizePoster, d.PosterPath),
		BackdropURL: c.ImageURL(tmdb.SizeBackdrop, d.BackdropPath),
		ExternalIDs: map[string]string{"tmdb": strconv.Itoa(d.ID)},
		Seasons:     d.Seasons,
		Credits: plugin.VideoCredits{
			Directors: people(c, d.Directors), Writers: people(c, d.Writers),
			Creators: people(c, d.Creators), Cast: people(c, d.Cast),
		},
	}
	if d.IMDBID != "" {
		out.ExternalIDs["imdb"] = d.IMDBID
	}
	for _, g := range d.Genres {
		out.Genres = append(out.Genres, g.Name)
	}
	return out, nil
}

// people převede osoby z TMDB do protokolu pluginů (ID osoby a fotka).
func people(c *tmdb.Client, list []tmdb.Person) []plugin.Person {
	out := make([]plugin.Person, 0, len(list))
	for _, p := range list {
		pp := plugin.Person{Name: p.Name, Character: p.Character, ImageURL: c.ImageURL(tmdb.SizeProfile, p.ProfilePath)}
		if p.ID != 0 {
			pp.ID = strconv.Itoa(p.ID)
		}
		out = append(out, pp)
	}
	return out
}

func season(ctx context.Context, req plugin.Request) (*plugin.SeasonMetadata, error) {
	c, err := client(req)
	if err != nil {
		return nil, err
	}
	id, err := idOf(req)
	if err != nil {
		return nil, err
	}
	s, err := c.Season(ctx, id, req.Season)
	if err != nil {
		return nil, wrap(err)
	}
	out := &plugin.SeasonMetadata{Number: s.Number, Name: s.Name, Overview: s.Overview,
		PosterURL: c.ImageURL(tmdb.SizePoster, s.PosterPath), Episodes: []plugin.EpisodeMetadata{}}
	for _, e := range s.Episodes {
		out.Episodes = append(out.Episodes, plugin.EpisodeMetadata{
			Number: e.Number, Title: e.Name, Overview: e.Overview, AirDate: e.AirDate,
			StillURL: c.ImageURL(tmdb.SizeStill, e.StillPath), RuntimeMin: e.Runtime,
		})
	}
	return out, nil
}

func wrap(err error) error {
	switch {
	case errors.Is(err, tmdb.ErrNotFound):
		return sdk.NotFound("%v", err)
	case errors.Is(err, tmdb.ErrNoKey):
		return sdk.Errorf(plugin.ErrCodeUnavailable, "%v", err)
	case strings.Contains(err.Error(), "HTTP 429"):
		return sdk.Errorf(plugin.ErrCodeRateLimited, "%v", err)
	}
	return err
}
