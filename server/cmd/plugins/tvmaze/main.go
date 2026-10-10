// Plugin metadat TVmaze – seriály (anglicky, bez API klíče).
package main

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"

	"mediater/internal/core/plugin"
	"mediater/internal/core/plugin/sdk"
	"mediater/internal/core/version"
	"mediater/internal/sceniter/tvmaze"
)

var pageRe = regexp.MustCompile(`^/shows/(\d+)`)

func main() {
	c := tvmaze.New()
	sdk.Run(sdk.Plugin{
		Manifest: plugin.Manifest{
			Name:      "tvmaze",
			Title:     "TVmaze",
			Version:   version.String(),
			Kinds:     []string{plugin.KindShow},
			Languages: []string{"en"},
			URLPatterns: map[string][]string{
				plugin.KindShow: {`(?i)^https?://(www\.)?tvmaze\.com/shows/\d+`},
			},
			ImageHosts: []string{"static.tvmaze.com"},
			// Limit API je 20 požadavků za 10 s; detail řady jsou dva.
			MinIntervalMS: 1000,
		},
		Search: func(ctx context.Context, req plugin.Request) ([]plugin.SearchResult, error) {
			res, err := c.Search(ctx, req.Query.Title)
			return res, wrap(err)
		},
		Fetch: func(ctx context.Context, req plugin.Request) (any, error) {
			id, err := idOf(req)
			if err != nil {
				return nil, err
			}
			d, err := c.Show(ctx, id)
			return d, wrap(err)
		},
		Season: func(ctx context.Context, req plugin.Request) (*plugin.SeasonMetadata, error) {
			id, err := idOf(req)
			if err != nil {
				return nil, err
			}
			s, err := c.Season(ctx, id, req.Season)
			return s, wrap(err)
		},
	})
}

func idOf(req plugin.Request) (int, error) {
	raw := req.ID
	if raw == "" {
		u, err := url.Parse(req.URL)
		if err != nil {
			return 0, sdk.BadRequest("neplatná adresa")
		}
		m := pageRe.FindStringSubmatch(u.Path)
		if m == nil {
			return 0, sdk.BadRequest("adresa není stránka seriálu na TVmaze")
		}
		raw = m[1]
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, sdk.BadRequest("neplatné ID %q", raw)
	}
	return id, nil
}

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tvmaze.ErrNotFound):
		return sdk.NotFound("%v", err)
	case errors.Is(err, tvmaze.ErrRateLimited):
		return sdk.Errorf(plugin.ErrCodeRateLimited, "%v", err)
	}
	return err
}
