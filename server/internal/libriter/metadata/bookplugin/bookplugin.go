// Package bookplugin obalí vestavěného poskytovatele knižních metadat do
// pluginu (viz core/plugin). Každý zdroj má vlastní binárku
// cmd/plugins/<jméno>, která jen zavolá Run.
package bookplugin

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"mediater/internal/core/plugin"
	"mediater/internal/core/plugin/sdk"
	"mediater/internal/libriter/metadata"
)

// IDFetcher umí knihu podle čísla záznamu (databazeknih.cz).
type IDFetcher interface {
	FetchBook(ctx context.Context, id int) (*metadata.BookMetadata, error)
}

// Run spustí plugin. newProvider dostane konfiguraci z požadavku (API klíč)
// a vrátí poskytovatele; proces obslouží jediný příkaz, takže vzniká pokaždé.
func Run(m plugin.Manifest, newProvider func(cfg map[string]string) metadata.Provider) {
	sdk.Run(Plugin(m, newProvider))
}

// Plugin sestaví obsluhu příkazů (testy volají sdk.Serve přímo).
func Plugin(m plugin.Manifest, newProvider func(cfg map[string]string) metadata.Provider) sdk.Plugin {
	if m.Kinds == nil {
		m.Kinds = []string{plugin.KindBook}
		if _, ok := newProvider(nil).(metadata.AuthorProvider); ok {
			m.Kinds = append(m.Kinds, plugin.KindAuthor)
		}
	}
	return sdk.Plugin{
		Manifest: m,
		Search: func(ctx context.Context, req plugin.Request) ([]plugin.SearchResult, error) {
			p := newProvider(req.Config)
			if req.Kind == plugin.KindAuthor {
				ap, ok := p.(metadata.AuthorProvider)
				if !ok {
					return nil, sdk.BadRequest("zdroj autory neumí")
				}
				res, err := ap.SearchAuthors(ctx, req.Query.Title)
				if err != nil {
					return nil, providerErr(err)
				}
				out := make([]plugin.SearchResult, 0, len(res))
				for _, r := range res {
					out = append(out, plugin.SearchResult{ID: idString(r.ID), URL: r.URL, Title: r.Name,
						Note: r.Note, BirthYear: r.BirthYear, DeathYear: r.DeathYear})
				}
				return out, nil
			}
			res, err := p.Search(ctx, metadata.SearchQuery{Title: req.Query.Title, Author: req.Query.Author, Language: req.Language})
			if err != nil {
				return nil, providerErr(err)
			}
			out := make([]plugin.SearchResult, 0, len(res))
			for _, r := range res {
				out = append(out, plugin.SearchResult{ID: idString(r.ID), URL: r.URL, Title: r.Title,
					Author: r.Author, Year: r.Year})
			}
			return out, nil
		},
		Fetch: func(ctx context.Context, req plugin.Request) (any, error) {
			p := newProvider(req.Config)
			if req.Kind == plugin.KindAuthor {
				ap, ok := p.(metadata.AuthorProvider)
				if !ok {
					return nil, sdk.BadRequest("zdroj autory neumí")
				}
				if req.URL == "" || !ap.SupportsAuthorURL(req.URL) {
					return nil, sdk.BadRequest("autor se stahuje podle URL zdroje")
				}
				meta, err := ap.FetchAuthorByURL(ctx, req.URL)
				return meta, providerErr(err)
			}
			if req.URL != "" {
				if !p.Supports(req.URL) {
					return nil, sdk.BadRequest("adresa nepatří tomuto zdroji")
				}
				meta, err := p.FetchByURL(ctx, req.URL)
				return meta, providerErr(err)
			}
			f, ok := p.(IDFetcher)
			if !ok {
				return nil, sdk.BadRequest("zdroj neumí stahovat podle ID")
			}
			id, err := strconv.Atoi(req.ID)
			if err != nil || id <= 0 {
				return nil, sdk.BadRequest("neplatné ID %q", req.ID)
			}
			meta, err := f.FetchBook(ctx, id)
			return meta, providerErr(err)
		},
	}
}

func idString(id int) string {
	if id == 0 {
		return ""
	}
	return strconv.Itoa(id)
}

// providerErr převede chybu zdroje na chybu protokolu.
func providerErr(err error) error {
	if err == nil {
		return nil
	}
	switch metadata.StatusOf(err) {
	case http.StatusNotFound, http.StatusGone:
		return sdk.NotFound("%v", err)
	case http.StatusTooManyRequests:
		return sdk.Errorf(plugin.ErrCodeRateLimited, "%v", err)
	}
	if errors.Is(err, metadata.ErrNoProvider) {
		return sdk.BadRequest("%v", err)
	}
	return err
}
