package metadata

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"mediater/internal/core/plugin"
)

// Zdroje knižních metadat běží jako pluginy (samostatné programy, viz
// balíček core/plugin). Tento adaptér je zapojí do Chain a Registry jako
// běžné poskytovatele – zbytek Libriteru o pluginech neví.

// IDProvider umí stáhnout knihu podle čísla záznamu u zdroje. Používá ho
// endpoint /metadata/book/{id} (číselná ID databazeknih.cz).
type IDProvider interface {
	FetchByID(ctx context.Context, id int) (*BookMetadata, error)
}

// PluginFactories vrátí továrny pro všechny nainstalované pluginy, které umí
// knihy. Pluginy, které umí i autory, se zapojí jako AuthorProvider.
func PluginFactories(host *plugin.Host) map[string]Factory {
	out := map[string]Factory{}
	if host == nil {
		return out
	}
	for _, p := range host.ByKind(plugin.KindBook) {
		base := pluginProvider{host: host, p: p}
		if p.Manifest.Supports(plugin.KindAuthor) {
			out[p.Name()] = func(ProviderConfig) Provider { return &pluginAuthorProvider{base} }
		} else {
			out[p.Name()] = func(ProviderConfig) Provider { return &base }
		}
	}
	return out
}

type pluginProvider struct {
	host *plugin.Host
	p    *plugin.Plugin
}

func (pp *pluginProvider) Name() string { return pp.p.Name() }

func (pp *pluginProvider) Supports(rawURL string) bool {
	return pp.p.SupportsURL(plugin.KindBook, rawURL)
}

func (pp *pluginProvider) SupportsCoverURL(rawURL string) bool {
	return pp.p.SupportsImageURL(rawURL)
}

func (pp *pluginProvider) Search(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	res, err := pp.host.Search(ctx, pp.Name(), plugin.Request{
		Kind: plugin.KindBook, Language: q.Language,
		Query: &plugin.Query{Title: q.Title, Author: q.Author},
	})
	if err != nil {
		return nil, pluginErr(err)
	}
	out := make([]SearchResult, 0, len(res))
	for _, r := range res {
		id, _ := strconv.Atoi(r.ID)
		out = append(out, SearchResult{ID: id, Title: r.Title, Author: r.Author, Year: r.Year, URL: r.URL})
	}
	return out, nil
}

func (pp *pluginProvider) FetchByURL(ctx context.Context, rawURL string) (*BookMetadata, error) {
	var meta BookMetadata
	if err := pp.host.Fetch(ctx, pp.Name(), plugin.Request{Kind: plugin.KindBook, URL: rawURL}, &meta); err != nil {
		return nil, pluginErr(err)
	}
	return &meta, nil
}

func (pp *pluginProvider) FetchByID(ctx context.Context, id int) (*BookMetadata, error) {
	var meta BookMetadata
	if err := pp.host.Fetch(ctx, pp.Name(), plugin.Request{Kind: plugin.KindBook, ID: strconv.Itoa(id)}, &meta); err != nil {
		return nil, pluginErr(err)
	}
	return &meta, nil
}

type pluginAuthorProvider struct{ pluginProvider }

func (pp *pluginAuthorProvider) SupportsAuthorURL(rawURL string) bool {
	return pp.p.SupportsURL(plugin.KindAuthor, rawURL)
}

func (pp *pluginAuthorProvider) SupportsImageURL(rawURL string) bool {
	return pp.p.SupportsImageURL(rawURL)
}

func (pp *pluginAuthorProvider) SearchAuthors(ctx context.Context, query string) ([]AuthorSearchResult, error) {
	res, err := pp.host.Search(ctx, pp.Name(), plugin.Request{
		Kind: plugin.KindAuthor, Query: &plugin.Query{Title: query},
	})
	if err != nil {
		return nil, pluginErr(err)
	}
	out := make([]AuthorSearchResult, 0, len(res))
	for _, r := range res {
		id, _ := strconv.Atoi(r.ID)
		out = append(out, AuthorSearchResult{ID: id, Name: r.Title, Note: r.Note,
			BirthYear: r.BirthYear, DeathYear: r.DeathYear, URL: r.URL})
	}
	return out, nil
}

func (pp *pluginAuthorProvider) FetchAuthorByURL(ctx context.Context, rawURL string) (*AuthorMetadata, error) {
	var meta AuthorMetadata
	if err := pp.host.Fetch(ctx, pp.Name(), plugin.Request{Kind: plugin.KindAuthor, URL: rawURL}, &meta); err != nil {
		return nil, pluginErr(err)
	}
	return &meta, nil
}

// pluginErr převede chybu pluginu na StatusError, podle kterého handlery
// rozlišují „nenalezeno“ a „limit“ stejně jako u vestavěných zdrojů.
func pluginErr(err error) error {
	var pe *plugin.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case plugin.ErrCodeNotFound:
			return &StatusError{Status: http.StatusNotFound, URL: pe.Plugin}
		case plugin.ErrCodeRateLimited:
			return &StatusError{Status: http.StatusTooManyRequests, URL: pe.Plugin}
		}
	}
	return err
}
