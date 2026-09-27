package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"libriter/internal/metadata"
)

// stubProvider vrací jeden výsledek a pamatuje si poslední dotaz.
type stubProvider struct {
	name string
	last metadata.SearchQuery
}

func (p *stubProvider) Name() string         { return p.name }
func (p *stubProvider) Supports(string) bool { return false }
func (p *stubProvider) FetchByURL(context.Context, string) (*metadata.BookMetadata, error) {
	return nil, metadata.ErrNoProvider
}
func (p *stubProvider) Search(_ context.Context, q metadata.SearchQuery) ([]metadata.SearchResult, error) {
	p.last = q
	return []metadata.SearchResult{{Title: q.Title, URL: "https://example.com/" + p.name}}, nil
}

// Hledání vybírá řetězec podle parametru language a jazyk posílá zdroji dál.
func TestMetadataSearchUsesLanguageChain(t *testing.T) {
	cs := &stubProvider{name: "ceskyzdroj"}
	en := &stubProvider{name: "anglickyzdroj"}
	registry := metadata.NewRegistry(map[string]metadata.Factory{
		cs.name: func(metadata.ProviderConfig) metadata.Provider { return cs },
		en.name: func(metadata.ProviderConfig) metadata.Provider { return en },
	})
	registry.Rebuild(metadata.Profiles{
		Default:    []string{cs.name},
		ByLanguage: map[string][]string{"en": {en.name}},
	}, metadata.ProviderConfig{})
	h := NewMetadata(registry)

	tests := []struct {
		query      string
		wantSource string
		wantLang   string
	}{
		{"?q=Dune&language=EN", en.name, "en"},
		{"?q=Dune&language=cs", cs.name, "cs"},
		{"?q=Dune", cs.name, ""},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.Search(rec, httptest.NewRequest(http.MethodGet, "/metadata/search"+tt.query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", tt.query, rec.Code, rec.Body)
		}

		var results []metadata.SearchResult
		if err := json.NewDecoder(rec.Body).Decode(&results); err != nil {
			t.Fatalf("%s: %v", tt.query, err)
		}
		if len(results) != 1 || results[0].Source != tt.wantSource {
			t.Errorf("%s: výsledky = %+v, chtěn zdroj %s", tt.query, results, tt.wantSource)
		}

		p := cs
		if tt.wantSource == en.name {
			p = en
		}
		if p.last.Language != tt.wantLang {
			t.Errorf("%s: zdroj dostal jazyk %q, chtěno %q", tt.query, p.last.Language, tt.wantLang)
		}
	}
}
