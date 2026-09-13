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
			t.Logf("nalezeno %d výsledků, první: %q (%s)",
				len(results), results[0].Title, results[0].URL)

			if results[0].Title == "" {
				t.Error("první výsledek nemá název")
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
