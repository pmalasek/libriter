package metadata

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeProvider je zdroj s předem danou odpovědí.
type fakeProvider struct {
	name    string
	host    string
	author  string
	results []SearchResult
	err     error
	calls   *int
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Supports(rawURL string) bool {
	return f.host != "" && HostMatches(rawURL, f.host)
}

func (f *fakeProvider) Search(context.Context, SearchQuery) ([]SearchResult, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.results, f.err
}

func (f *fakeProvider) FetchByURL(context.Context, string) (*BookMetadata, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &BookMetadata{Title: f.name + " kniha", Author: f.author}, nil
}

func hit(title string) []SearchResult { return []SearchResult{{Title: title}} }

func TestChainSearchFallback(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("scraper se rozbil")

	t.Run("první zdroj s výsledkem vyhrává", func(t *testing.T) {
		var secondCalls int
		chain := NewChain(
			&fakeProvider{name: "prvni", results: hit("A")},
			&fakeProvider{name: "druhy", results: hit("B"), calls: &secondCalls},
		)

		got, err := chain.Search(ctx, SearchQuery{Title: "dotaz"})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(got) != 1 || got[0].Title != "A" {
			t.Errorf("výsledky = %v, chtěno A", got)
		}
		if got[0].Source != "prvni" {
			t.Errorf("source = %q, chtěno %q", got[0].Source, "prvni")
		}
		if secondCalls != 0 {
			t.Errorf("druhý zdroj se volal %dx, neměl vůbec", secondCalls)
		}
	})

	t.Run("selhání i prázdno se přeskočí", func(t *testing.T) {
		chain := NewChain(
			&fakeProvider{name: "rozbity", err: boom},
			&fakeProvider{name: "prazdny"},
			&fakeProvider{name: "funkcni", results: hit("C")},
		)

		got, err := chain.Search(ctx, SearchQuery{Title: "dotaz"})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(got) != 1 || got[0].Title != "C" || got[0].Source != "funkcni" {
			t.Errorf("výsledky = %v", got)
		}
	})

	t.Run("selhaly všechny -> chyba", func(t *testing.T) {
		chain := NewChain(
			&fakeProvider{name: "a", err: boom},
			&fakeProvider{name: "b", err: boom},
		)

		if _, err := chain.Search(ctx, SearchQuery{Title: "dotaz"}); !errors.Is(err, boom) {
			t.Errorf("err = %v, chtěna původní chyba", err)
		}
	})

	t.Run("všichni odpověděli prázdno -> prázdný výsledek bez chyby", func(t *testing.T) {
		chain := NewChain(&fakeProvider{name: "a"}, &fakeProvider{name: "b"})

		got, err := chain.Search(ctx, SearchQuery{Title: "dotaz"})
		if err != nil {
			t.Errorf("err = %v, chtěno nil (kniha prostě nikde není)", err)
		}
		if len(got) != 0 {
			t.Errorf("výsledky = %v, chtěno prázdno", got)
		}
	})

	t.Run("prázdný řetězec nepadá", func(t *testing.T) {
		got, err := NewChain().Search(ctx, SearchQuery{Title: "dotaz"})
		if err != nil || len(got) != 0 {
			t.Errorf("got = %v, err = %v", got, err)
		}
	})
}

func TestChainFetchByURLRoutesBySource(t *testing.T) {
	ctx := context.Background()
	chain := NewChain(
		&fakeProvider{name: "cesky", host: "example.cz"},
		&fakeProvider{name: "svetovy", host: "example.org"},
	)

	meta, err := chain.FetchByURL(ctx, "https://www.example.org/kniha-1")
	if err != nil {
		t.Fatalf("FetchByURL: %v", err)
	}
	if meta.Source != "svetovy" {
		t.Errorf("source = %q, chtěno %q", meta.Source, "svetovy")
	}

	// Cizí adresa nepatří nikomu – zároveň to je allowlist pro handler.
	if _, err := chain.FetchByURL(ctx, "https://zlo.example.net/x"); !errors.Is(err, ErrNoProvider) {
		t.Errorf("err = %v, chtěno ErrNoProvider", err)
	}
	if chain.Supports("https://zlo.example.net/x") {
		t.Error("Supports pustil cizí adresu")
	}
}

func TestBuildChainRespectsConfig(t *testing.T) {
	factories := map[string]Factory{
		"a": func() Provider { return &fakeProvider{name: "a"} },
		"b": func() Provider { return &fakeProvider{name: "b"} },
		"c": func() Provider { return &fakeProvider{name: "c"} },
	}

	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{"pořadí z konfigurace", []string{"c", "a"}, []string{"c", "a"}},
		{"vynechaný zdroj je vypnutý", []string{"b"}, []string{"b"}},
		{"prázdno vypne metadata úplně", nil, []string{}},
		{"neznámé jméno se přeskočí", []string{"a", "preklep", "b"}, []string{"a", "b"}},
		{"duplicita se nevolá dvakrát", []string{"a", "a", "b"}, []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildChain(tt.names, factories).Providers()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Providers() = %v, chtěno %v", got, tt.want)
			}
		})
	}
}

func TestHostMatches(t *testing.T) {
	tests := []struct {
		rawURL string
		host   string
		want   bool
	}{
		{"https://www.databazeknih.cz/prehled-knihy/x-1", "databazeknih.cz", true},
		{"https://databazeknih.cz/x", "databazeknih.cz", true},
		{"http://databazeknih.cz/x", "databazeknih.cz", true},
		{"https://DATABAZEKNIH.CZ/x", "databazeknih.cz", true},
		// Podvržené domény nesmí projít.
		{"https://databazeknih.cz.zlo.net/x", "databazeknih.cz", false},
		{"https://zlodatabazeknih.cz/x", "databazeknih.cz", false},
		{"https://openlibrary.org/works/OL1W", "databazeknih.cz", false},
		// Jiné schéma než http(s) je taky mimo.
		{"file:///etc/passwd", "databazeknih.cz", false},
		{"javascript:alert(1)", "databazeknih.cz", false},
		{"", "databazeknih.cz", false},
	}

	for _, tt := range tests {
		if got := HostMatches(tt.rawURL, tt.host); got != tt.want {
			t.Errorf("HostMatches(%q, %q) = %v, chtěno %v", tt.rawURL, tt.host, got, tt.want)
		}
	}
}

// fakeAuthorProvider je zdroj, který umí i autory – vrací pevně dané jméno.
type fakeAuthorProvider struct {
	fakeProvider
	authorName string
}

func (f *fakeAuthorProvider) SupportsAuthorURL(rawURL string) bool { return f.Supports(rawURL) }
func (f *fakeAuthorProvider) SupportsImageURL(string) bool         { return false }

func (f *fakeAuthorProvider) SearchAuthors(context.Context, string) ([]AuthorSearchResult, error) {
	return nil, nil
}

func (f *fakeAuthorProvider) FetchAuthorByURL(context.Context, string) (*AuthorMetadata, error) {
	return &AuthorMetadata{Name: f.authorName}, nil
}

func TestChainFetchAuthorSplitsName(t *testing.T) {
	tests := []struct {
		name                string
		full                string
		first, middle, last string
	}{
		{"křestní a příjmení", "Karel Čapek", "Karel", "", "Čapek"},
		{"tři části", "Jan Amos Komenský", "Jan", "Amos", "Komenský"},
		{"obrácený tvar", "Komenský, Jan Amos", "Jan", "Amos", "Komenský"},
		{"jednoslovné", "Homér", "", "", "Homér"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chain := NewChain(&fakeAuthorProvider{
				fakeProvider: fakeProvider{name: "dk", host: "databazeknih.cz"},
				authorName:   tt.full,
			})

			meta, err := chain.FetchAuthorByURL(context.Background(), "https://www.databazeknih.cz/autori/x-1")
			if err != nil {
				t.Fatalf("FetchAuthorByURL: %v", err)
			}
			got := [3]string{meta.FirstName, meta.MiddleName, meta.LastName}
			want := [3]string{tt.first, tt.middle, tt.last}
			if got != want {
				t.Errorf("rozdělení %q = %v, chtěno %v", tt.full, got, want)
			}
			if meta.Source != "dk" {
				t.Errorf("source = %q, chtěno dk", meta.Source)
			}
		})
	}
}

// Autory rozebírá Chain, ne jednotlivé zdroje – klient tak dostane jména
// rozdělená na části stejně, jako se ukládají v knihovně.
func TestChainFetchByURLSplitsAuthors(t *testing.T) {
	chain := NewChain(&fakeProvider{name: "cesky", host: "example.cz", author: "Karel Čapek, Josef Čapek"})

	meta, err := chain.FetchByURL(context.Background(), "https://example.cz/kniha-1")
	if err != nil {
		t.Fatalf("FetchByURL: %v", err)
	}

	want := []BookAuthor{
		{Name: "Karel Čapek", FirstName: "Karel", LastName: "Čapek"},
		{Name: "Josef Čapek", FirstName: "Josef", LastName: "Čapek"},
	}
	if !reflect.DeepEqual(meta.Authors, want) {
		t.Errorf("authors = %+v, chtěno %+v", meta.Authors, want)
	}
}

func TestSplitAuthors(t *testing.T) {
	tests := []struct {
		author string
		want   []string
	}{
		{"Douglas Adams", []string{"Douglas Adams"}},
		// Čárka odděluje autory, ale ve tvaru „Příjmení, Křestní“ ne.
		{"Čapek, Karel", []string{"Karel Čapek"}},
		{"Terry Pratchett & Neil Gaiman", []string{"Terry Pratchett", "Neil Gaiman"}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.author, func(t *testing.T) {
			var got []string
			for _, a := range SplitAuthors(tt.author) {
				got = append(got, a.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("jména = %v, chtěno %v", got, tt.want)
			}
		})
	}
}

func TestAuthorMatches(t *testing.T) {
	tests := []struct {
		result, wanted string
		want           bool
	}{
		{"Samuel Bjørk", "Samuel Bjørk", true},
		// Zdroje píšou jména různě a u víc autorů je hledaný jen jeden z nich.
		{"Bjørk, Samuel", "Samuel Bjørk", true},
		{"František Kotleta, Kristýna Sněgoňová", "Kristýna Sněgoňová", true},
		{"Robert Merle", "Samuel Bjørk", false},
		// Diakritika se musí shodovat, jinak by „Bjork“ prošel jako „Bjørk“.
		{"Samuel Bjork", "Samuel Bjørk", false},
		{"Robert Merle", "", false},
	}

	for _, tt := range tests {
		if got := AuthorMatches(tt.result, tt.wanted); got != tt.want {
			t.Errorf("AuthorMatches(%q, %q) = %v", tt.result, tt.wanted, got)
		}
	}
}

// Zdroj, který hledá jen v názvech, vrátí jmenovce v libovolném pořadí –
// knihy hledaného autora patří nahoru, zbytek si pořadí drží.
func TestRankByAuthor(t *testing.T) {
	results := []SearchResult{
		{Title: "Ostrov", Author: "Robert Merle"},
		{Title: "Ostrov", Author: "Samuel Bjørk"},
		{Title: "Ostrov", Author: "Aldous Huxley"},
	}

	got := RankByAuthor(results, "Samuel Bjørk")
	want := []string{"Samuel Bjørk", "Robert Merle", "Aldous Huxley"}
	for i, author := range want {
		if got[i].Author != author {
			t.Errorf("pořadí[%d] = %q, chtěno %q", i, got[i].Author, author)
		}
	}

	// Bez autora se pořadí nemění.
	if got := RankByAuthor(results, ""); got[0].Author != "Robert Merle" {
		t.Errorf("bez autora se pořadí změnilo: %+v", got)
	}
}
