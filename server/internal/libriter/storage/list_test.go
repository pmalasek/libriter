package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"mediater/internal/libriter/model"
)

type listFixture struct {
	store  *Store
	capek  model.Author
	nemcov model.Author
	series *model.Series
	books  map[string]uuid.UUID
}

// newListFixture: dvě knihy Čapka, série Babičky (dva díly, druhý první
// v abecedě), kniha bez autora.
func newListFixture(t *testing.T) *listFixture {
	t.Helper()
	ctx := context.Background()
	store := newTestStore(t)
	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{
		{First: "Karel", Last: "Čapek"},
		{First: "Božena", Last: "Němcová"},
	})
	if err != nil {
		t.Fatal(err)
	}
	series, err := store.CreateSeries(ctx, "Babička", nil)
	if err != nil {
		t.Fatal(err)
	}
	f := &listFixture{store: store, capek: authors[0], nemcov: authors[1], series: series, books: map[string]uuid.UUID{}}

	pos := func(n int16) *int16 { return &n }
	year := func(n int) *int { return &n }
	add := func(title string, in BookInput) {
		in.Title = title
		in.DurationSeconds = 60
		in.FilePath = "x/" + title
		in.Language = "cs"
		b, err := store.CreateBook(ctx, in)
		if err != nil {
			t.Fatalf("CreateBook %s: %v", title, err)
		}
		f.books[title] = b.ID
	}
	add("Válka s mloky", BookInput{AuthorIDs: []uuid.UUID{f.capek.ID}, PublishedYear: year(1936)})
	add("Zahradníkův rok", BookInput{AuthorIDs: []uuid.UUID{f.capek.ID}, PublishedYear: year(1929)})
	add("Zima", BookInput{AuthorIDs: []uuid.UUID{f.nemcov.ID}, SeriesID: &series.ID, SeriesPosition: pos(2)})
	add("Jaro", BookInput{AuthorIDs: []uuid.UUID{f.nemcov.ID}, SeriesID: &series.ID, SeriesPosition: pos(1)})
	add("Anonym", BookInput{})
	return f
}

func titles(books []model.BookSummary) []string {
	out := make([]string, len(books))
	for i, b := range books {
		out[i] = b.Title
	}
	return out
}

func TestListBookSummariesSort(t *testing.T) {
	f := newListFixture(t)
	ctx := context.Background()

	cases := []struct {
		filter BookFilter
		want   string
	}{
		// Díly série stojí pod názvem série v pořadí dílů.
		{BookFilter{Sort: BookSortTitle}, "[Anonym Jaro Zima Válka s mloky Zahradníkův rok]"},
		{BookFilter{Sort: BookSortTitle, Desc: true}, "[Zahradníkův rok Válka s mloky Zima Jaro Anonym]"},
		// Kniha bez autora jde na konec.
		{BookFilter{Sort: BookSortAuthor}, "[Válka s mloky Zahradníkův rok Jaro Zima Anonym]"},
		// Bez roku vydání na konec.
		{BookFilter{Sort: BookSortPublished}, "[Zahradníkův rok Válka s mloky Jaro Zima Anonym]"},
		// I sestupně jsou knihy bez roku na konci.
		{BookFilter{Sort: BookSortPublished, Desc: true}, "[Válka s mloky Zahradníkův rok Jaro Zima Anonym]"},
	}
	for _, c := range cases {
		books, total, err := f.store.ListBookSummaries(ctx, c.filter, 100, 0)
		if err != nil {
			t.Fatalf("%+v: %v", c.filter, err)
		}
		if total != 5 {
			t.Errorf("%+v: total %d", c.filter, total)
		}
		if got := fmt.Sprint(titles(books)); got != c.want {
			t.Errorf("%+v:\n dostal %s\n chci   %s", c.filter, got, c.want)
		}
	}
}

func TestListBookSummariesFilterAndPage(t *testing.T) {
	f := newListFixture(t)
	ctx := context.Background()

	// Hledání bez diakritiky v názvu i ve jménu autora.
	books, total, err := f.store.ListBookSummaries(ctx, BookFilter{Query: "capek"}, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(books) != 2 {
		t.Errorf("hledání 'capek': %v (total %d)", titles(books), total)
	}
	books, _, _ = f.store.ListBookSummaries(ctx, BookFilter{Query: "capek valka"}, 100, 0)
	if fmt.Sprint(titles(books)) != "[Válka s mloky]" {
		t.Errorf("hledání podle autora a názvu: %v", titles(books))
	}
	books, _, _ = f.store.ListBookSummaries(ctx, BookFilter{Query: "ZAHRADNIK"}, 100, 0)
	if fmt.Sprint(titles(books)) != "[Zahradníkův rok]" {
		t.Errorf("hledání v názvu: %v", titles(books))
	}

	// Stránka uprostřed a za koncem.
	books, total, _ = f.store.ListBookSummaries(ctx, BookFilter{}, 2, 2)
	if total != 5 || fmt.Sprint(titles(books)) != "[Zima Válka s mloky]" {
		t.Errorf("stránka 2: %v total %d", titles(books), total)
	}
	books, total, _ = f.store.ListBookSummaries(ctx, BookFilter{}, 2, 10)
	if total != 5 || len(books) != 0 {
		t.Errorf("za koncem: %v total %d", titles(books), total)
	}

	// Filtry autora, série a výběr podle ID.
	books, _, _ = f.store.ListBookSummaries(ctx, BookFilter{AuthorID: f.capek.ID}, 100, 0)
	if len(books) != 2 || books[0].Authors[0].Name != "Karel Čapek" {
		t.Errorf("autor: %+v", books)
	}
	books, _, _ = f.store.ListBookSummaries(ctx, BookFilter{SeriesID: f.series.ID}, 100, 0)
	if fmt.Sprint(titles(books)) != "[Jaro Zima]" || books[0].SeriesTitle == nil || *books[0].SeriesTitle != "Babička" {
		t.Errorf("série: %v", titles(books))
	}
	books, _, _ = f.store.ListBookSummaries(ctx, BookFilter{IDs: []uuid.UUID{f.books["Zima"], uuid.New()}}, 100, 0)
	if fmt.Sprint(titles(books)) != "[Zima]" {
		t.Errorf("ids: %v", titles(books))
	}
	books, _, _ = f.store.ListBookSummaries(ctx, BookFilter{IDs: []uuid.UUID{}}, 100, 0)
	if len(books) != 0 {
		t.Errorf("prázdné ids: %v", titles(books))
	}
}

// Klíče řazení udržují triggery: přejmenování série a autora se projeví hned.
func TestListBookSummariesFollowsRenames(t *testing.T) {
	f := newListFixture(t)
	ctx := context.Background()

	if _, err := f.store.UpdateSeries(ctx, f.series.ID, "Zzz série", nil); err != nil {
		t.Fatal(err)
	}
	books, _, _ := f.store.ListBookSummaries(ctx, BookFilter{Sort: BookSortTitle}, 100, 0)
	if got := fmt.Sprint(titles(books)); got != "[Anonym Válka s mloky Zahradníkův rok Jaro Zima]" {
		t.Errorf("po přejmenování série: %s", got)
	}

	if _, err := f.store.UpdateAuthor(ctx, f.capek.ID, AuthorInput{Name: model.AuthorName{First: "Karel", Last: "Žák"}}); err != nil {
		t.Fatal(err)
	}
	books, _, _ = f.store.ListBookSummaries(ctx, BookFilter{Sort: BookSortAuthor}, 100, 0)
	if got := fmt.Sprint(titles(books)); got != "[Jaro Zima Válka s mloky Zahradníkův rok Anonym]" {
		t.Errorf("po přejmenování autora: %s", got)
	}
}

func TestListAuthorAndSeriesSummaries(t *testing.T) {
	f := newListFixture(t)
	ctx := context.Background()

	// Autor bez knih se nevypisuje.
	if _, err := f.store.CreateAuthor(ctx, AuthorInput{Name: model.AuthorName{First: "Nikdo", Last: "Prázdný"}}); err != nil {
		t.Fatal(err)
	}
	authors, total, err := f.store.ListAuthorSummaries(ctx, AuthorFilter{}, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || authors[0].Name != "Karel Čapek" || authors[0].BookCount != 2 {
		t.Errorf("autoři: %+v total %d", authors, total)
	}
	if _, total, _ := f.store.ListAuthorSummaries(ctx, AuthorFilter{Query: "capek karel"}, 100, 0); total != 1 {
		t.Errorf("slova jména v jiném pořadí: total %d", total)
	}
	authors, total, _ = f.store.ListAuthorSummaries(ctx, AuthorFilter{WithoutBooks: true, Query: "prazdny"}, 100, 0)
	if total != 1 || authors[0].BookCount != 0 {
		t.Errorf("autor bez knih: %+v", authors)
	}

	list, total, err := f.store.ListSeriesSummaries(ctx, SeriesFilter{}, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || list[0].BookCount != 2 || len(list[0].Authors) != 1 || len(list[0].Covers) != 2 {
		t.Fatalf("série: %+v", list)
	}
	if list[0].Covers[0].Title != "Jaro" {
		t.Errorf("první díl ve stohu: %s", list[0].Covers[0].Title)
	}
	// Série se najde i podle autora.
	if _, total, _ := f.store.ListSeriesSummaries(ctx, SeriesFilter{Query: "nemcova"}, 100, 0); total != 1 {
		t.Errorf("hledání série podle autora: total %d", total)
	}
}
