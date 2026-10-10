// Plugin metadat ČSFD (csfd.cz) – filmy a seriály v češtině.
package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"mediater/internal/core/plugin"
	"mediater/internal/core/plugin/sdk"
	"mediater/internal/core/version"
	"mediater/internal/sceniter/csfd"
)

func main() {
	c := csfd.New()
	sdk.Run(sdk.Plugin{
		Manifest: plugin.Manifest{
			Name:      "csfd",
			Title:     "ČSFD (csfd.cz)",
			Version:   version.String(),
			Kinds:     []string{plugin.KindMovie, plugin.KindShow},
			Languages: []string{"cs", "sk"},
			URLPatterns: map[string][]string{
				plugin.KindMovie: {`(?i)^https?://(www\.)?csfd\.(cz|sk)/film/\d+`},
				plugin.KindShow:  {`(?i)^https?://(www\.)?csfd\.(cz|sk)/film/\d+`},
			},
			ImageHosts: []string{"image.pmgstatic.com"},
			// Scraping cizího webu – slušný odstup mezi požadavky.
			MinIntervalMS: 2000,
		},
		Search: func(ctx context.Context, req plugin.Request) ([]plugin.SearchResult, error) {
			res, err := c.Search(ctx, req.Kind, req.Query.Title)
			return res, wrap(err)
		},
		Fetch: func(ctx context.Context, req plugin.Request) (any, error) {
			id, err := idOf(req)
			if err != nil {
				return nil, err
			}
			d, err := c.Film(ctx, id)
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

func idOf(req plugin.Request) (string, error) {
	if req.ID != "" {
		if _, err := strconv.Atoi(req.ID); err != nil {
			return "", sdk.BadRequest("neplatné ID %q", req.ID)
		}
		return req.ID, nil
	}
	if id, ok := csfd.IDFromURL(req.URL); ok {
		return id, nil
	}
	return "", sdk.BadRequest("adresa není stránka filmu na ČSFD")
}

func wrap(err error) error {
	var se *csfd.StatusError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, csfd.ErrNotFound):
		return sdk.NotFound("%v", err)
	case errors.As(err, &se) && se.Status == http.StatusTooManyRequests:
		return sdk.Errorf(plugin.ErrCodeRateLimited, "%v", err)
	}
	return err
}
