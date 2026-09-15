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

	if _, err := store.CreateUser(ctx, "Admin", "admin@example.com", "hash", model.RoleAdmin); err != nil {
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
