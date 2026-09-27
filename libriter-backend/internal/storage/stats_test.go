package storage

import (
	"context"
	"testing"

	"libriter/internal/model"

	"github.com/google/uuid"
)

func TestLibraryStats(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}

	description := "Popis knihy"
	withMeta, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           "S popisem",
		DurationSeconds: 3600,
		FilePath:        "capek/s-popisem",
		Language:        "cs",
		Description:     &description,
		CoverPath:       strPtr("cover.jpg"),
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}

	bare, err := store.CreateBook(ctx, BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           "Bez ničeho",
		DurationSeconds: 1800,
		FilePath:        "capek/bez-niceho",
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}

	// Kapitola s délkou 1 s je placeholder – vznikne, když chybí ffprobe.
	if _, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: bare.ID, Position: 1, Title: "01", FilePath: "capek/bez-niceho/01.mp3", DurationSeconds: 1,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}
	if _, err := store.UpsertChapter(ctx, ChapterInput{
		BookID: withMeta.ID, Position: 1, Title: "01", FilePath: "capek/s-popisem/01.mp3", DurationSeconds: 1800,
	}); err != nil {
		t.Fatalf("UpsertChapter: %v", err)
	}

	if _, err := store.CreateUser(ctx, "Admin", "admin@example.com", "", "hash", model.RoleAdmin); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	stats, err := store.LibraryStats(ctx)
	if err != nil {
		t.Fatalf("LibraryStats: %v", err)
	}

	checks := []struct {
		name string
		got  int
		want int
	}{
		{"knihy", stats.Books, 2},
		{"autoři", stats.Authors, 1},
		{"série", stats.Series, 0},
		{"uživatelé", stats.Users, 1},
		{"kapitoly", stats.Chapters, 2},
		{"bez obálky", stats.BooksWithoutCover, 1},
		{"bez popisu", stats.BooksWithoutDescription, 1},
		{"placeholder kapitoly", stats.PlaceholderChapters, 1},
		{"knihy s placeholderem", stats.BooksWithPlaceholderChapters, 1},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, chtěno %d", c.name, c.got, c.want)
		}
	}
	if stats.TotalDurationSeconds != 5400 {
		t.Errorf("celková délka = %d, chtěno 5400", stats.TotalDurationSeconds)
	}
}

func strPtr(s string) *string { return &s }

// Dlaždice „možné duplikáty“ počítá knihy se stejným album tagem ve dvou
// nesouvisejících adresářích. Disky jedné knihy (podadresáře) se nepočítají.
func TestLibraryStatsCountsDuplicateAlbums(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Daniel", Last: "Cole"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}

	create := func(title, dir, albumTag string) {
		t.Helper()
		in := BookInput{
			AuthorIDs:       []uuid.UUID{authors[0].ID},
			Title:           title,
			DurationSeconds: 3600,
			FilePath:        dir,
			Language:        "cs",
		}
		if albumTag != "" {
			in.AlbumTag = &albumTag
		}
		if _, err := store.CreateBook(ctx, in); err != nil {
			t.Fatalf("CreateBook: %v", err)
		}
	}

	// Dvě kopie téže knihy v nesouvisejících adresářích → obě se počítají.
	create("Osamělý mrtvý muž", "cole/osamely", "Osamělý mrtvý muž")
	create("Osamělý mrtvý muž", "nove/osamely-2", "Osamělý mrtvý muž")
	// Disk téže knihy v podadresáři → duplicita to není.
	create("Loutkář", "cole/loutkar", "Loutkář")
	create("Loutkář", "cole/loutkar/CD2", "Loutkář")
	// Kniha bez album tagu do počtu nepatří.
	create("Bez tagu", "cole/bez-tagu", "")

	stats, err := store.LibraryStats(ctx)
	if err != nil {
		t.Fatalf("LibraryStats: %v", err)
	}
	if stats.DuplicateAlbumBooks != 2 {
		t.Errorf("možné duplikáty = %d, chtěny 2", stats.DuplicateAlbumBooks)
	}
}

// Odmítnutá skupina („není to duplicita“) nesmí dál svítit na Přehledu –
// jinak by dlaždice říkala něco jiného než karta v Knihovně.
func TestLibraryStatsIgnoresDismissedDuplicates(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}

	tag := "Krakatit"
	var ids []uuid.UUID
	for _, dir := range []string{"vydani-a/krakatit", "vydani-b/krakatit"} {
		b, err := store.CreateBook(ctx, BookInput{
			AuthorIDs:       []uuid.UUID{authors[0].ID},
			Title:           "Krakatit",
			DurationSeconds: 3600,
			FilePath:        dir,
			AlbumTag:        &tag,
			Language:        "cs",
		})
		if err != nil {
			t.Fatalf("CreateBook: %v", err)
		}
		ids = append(ids, b.ID)
	}

	if stats, err := store.LibraryStats(ctx); err != nil {
		t.Fatalf("LibraryStats: %v", err)
	} else if stats.DuplicateAlbumBooks != 2 {
		t.Fatalf("před odmítnutím = %d, chtěny 2", stats.DuplicateAlbumBooks)
	}

	key := DuplicateGroupKey(ids)
	if err := store.DismissDuplicate(ctx, key, ids, nil); err != nil {
		t.Fatalf("DismissDuplicate: %v", err)
	}

	if stats, err := store.LibraryStats(ctx); err != nil {
		t.Fatalf("LibraryStats: %v", err)
	} else if stats.DuplicateAlbumBooks != 0 {
		t.Errorf("po odmítnutí = %d, chtěna 0", stats.DuplicateAlbumBooks)
	}

	// Neúplný klíč (kniha ze skupiny zmizela) odmítnutí ruší.
	if _, err := store.db.ExecContext(ctx,
		`DELETE FROM duplicate_dismissals WHERE book_id = ?1`, ids[0]); err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	if stats, err := store.LibraryStats(ctx); err != nil {
		t.Fatalf("LibraryStats: %v", err)
	} else if stats.DuplicateAlbumBooks != 2 {
		t.Errorf("po rozpadu klíče = %d, chtěny 2", stats.DuplicateAlbumBooks)
	}
}
