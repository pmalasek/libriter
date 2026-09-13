package metadata_test

// Živý smoke test zdrojů metadat. Chodí na cizí weby a API, takže se
// ve výchozím stavu přeskakuje – spouští se ručně, když je podezření,
// že se některý zdroj rozbil:
//
//	LIBRITER_LIVE_METADATA=1 go test ./internal/metadata/ -run Live -v
//
// Scrapery stojí na HTML cizích webů a to se mění bez ohlášení; tenhle test
// je nejrychlejší způsob, jak zjistit který zdroj zlobí.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"libriter/internal/metadata"
	"libriter/internal/metadata/cbdb"
	"libriter/internal/metadata/databazeknih"
	"libriter/internal/metadata/googlebooks"
	"libriter/internal/metadata/openlibrary"
)

func TestLiveProviders(t *testing.T) {
	if os.Getenv("LIBRITER_LIVE_METADATA") == "" {
		t.Skip("živý test zdrojů – zapne se LIBRITER_LIVE_METADATA=1")
	}

	providers := []metadata.Provider{
		databazeknih.NewClient(),
		cbdb.NewClient(),
		openlibrary.NewClient(),
		googlebooks.NewClient(os.Getenv("GOOGLE_BOOKS_API_KEY")),
	}

	for _, provider := range providers {
		t.Run(provider.Name(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			results, err := provider.Search(ctx, "Válka s mloky")
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(results) == 0 {
				t.Fatal("Search nevrátil nic – zdroj je nejspíš rozbitý")
			}
			t.Logf("nalezeno %d výsledků", len(results))
			for _, r := range results[:min(3, len(results))] {
				t.Logf("  %q | autor %q | rok %d | %s", r.Title, r.Author, r.Year, r.URL)
			}

			if results[0].Title == "" {
				t.Error("první výsledek nemá název")
			}
			// Bez autora nejde z víc stejných názvů vybrat ten správný.
			if results[0].Author == "" {
				t.Error("první výsledek nemá autora")
			}
			if !provider.Supports(results[0].URL) {
				t.Fatalf("zdroj nepozná vlastní URL: %s", results[0].URL)
			}

			meta, err := provider.FetchByURL(ctx, results[0].URL)
			if err != nil {
				t.Fatalf("FetchByURL(%s): %v", results[0].URL, err)
			}
			if meta.Title == "" {
				t.Error("detail nemá název")
			}
			t.Logf("detail: %q | autor %q | rok %d | popis %d znaků | obálka %q",
				meta.Title, meta.Author, meta.Year, len(meta.Description), meta.CoverURL)
		})
	}
}

// TestLiveAuthorProviders ověří zdroje, které umí i autory (Google Books
// autory jako samostatné záznamy nemá, ten tu chybí schválně).
func TestLiveAuthorProviders(t *testing.T) {
	if os.Getenv("LIBRITER_LIVE_METADATA") == "" {
		t.Skip("živý test zdrojů – zapne se LIBRITER_LIVE_METADATA=1")
	}

	providers := []metadata.AuthorProvider{
		databazeknih.NewClient(),
		cbdb.NewClient(),
		openlibrary.NewClient(),
	}

	for _, provider := range providers {
		t.Run(provider.Name(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			results, err := provider.SearchAuthors(ctx, "Karel Čapek")
			if err != nil {
				t.Fatalf("SearchAuthors: %v", err)
			}
			if len(results) == 0 {
				t.Fatal("SearchAuthors nevrátil nic – zdroj je nejspíš rozbitý")
			}
			for _, r := range results[:min(3, len(results))] {
				t.Logf("  %q | %s | %s", r.Name, r.Note, r.URL)
			}

			// Najdeme Čapka, ne prvního jmenovce – ať je co stahovat.
			target := results[0]
			for _, r := range results {
				if strings.Contains(r.Name, "Čapek") {
					target = r
					break
				}
			}

			if !provider.SupportsAuthorURL(target.URL) {
				t.Fatalf("zdroj nepozná vlastní URL autora: %s", target.URL)
			}

			meta, err := provider.FetchAuthorByURL(ctx, target.URL)
			if err != nil {
				t.Fatalf("FetchAuthorByURL(%s): %v", target.URL, err)
			}
			if meta.Name == "" {
				t.Error("detail autora nemá jméno")
			}
			t.Logf("detail: %q | roky %d–%d | životopis %d znaků | fotka %q",
				meta.Name, meta.BirthYear, meta.DeathYear, len(meta.Bio), meta.ImageURL)

			// Fotka není samozřejmost – OpenLibrary má u známých autorů
			// spoustu duplicitních, prázdných záznamů. Když ale adresa přijde,
			// musí projít allowlistem, jinak by ji nešlo stáhnout.
			if meta.ImageURL != "" && !provider.SupportsImageURL(meta.ImageURL) {
				t.Errorf("zdroj nepovoluje vlastní adresu fotky: %s", meta.ImageURL)
			}
		})
	}
}
