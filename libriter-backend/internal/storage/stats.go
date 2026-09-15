package storage

import (
	"context"
	"fmt"
)

// LibraryStats je přehled knihovny pro administraci. Kromě prostých počtů
// obsahuje i „co je potřeba doplnit“ – knihy bez obálky či popisu a kapitoly
// s placeholder délkou 1 s, které vzniknou, když chybí ffprobe.
type LibraryStats struct {
	Books                        int   `json:"books"`
	Authors                      int   `json:"authors"`
	Series                       int   `json:"series"`
	Users                        int   `json:"users"`
	Chapters                     int   `json:"chapters"`
	TotalDurationSeconds         int64 `json:"total_duration_seconds"`
	BooksWithoutCover            int   `json:"books_without_cover"`
	BooksWithoutDescription      int   `json:"books_without_description"`
	PlaceholderChapters          int   `json:"placeholder_chapters"`
	BooksWithPlaceholderChapters int   `json:"books_with_placeholder_chapters"`
}

// LibraryStats spočítá přehled jedním dotazem – jednotlivé počty jsou
// skalární poddotazy, takže se databáze otevírá jen jednou.
func (s *Store) LibraryStats(ctx context.Context) (LibraryStats, error) {
	const q = `
		SELECT
		  (SELECT COUNT(*) FROM books),
		  (SELECT COUNT(*) FROM authors),
		  (SELECT COUNT(*) FROM series),
		  (SELECT COUNT(*) FROM users),
		  (SELECT COUNT(*) FROM chapters),
		  (SELECT COALESCE(SUM(duration_seconds), 0) FROM books),
		  (SELECT COUNT(*) FROM books WHERE cover_path IS NULL OR cover_path = ''),
		  (SELECT COUNT(*) FROM books WHERE description IS NULL OR trim(description) = ''),
		  (SELECT COUNT(*) FROM chapters WHERE duration_seconds = 1),
		  (SELECT COUNT(DISTINCT book_id) FROM chapters WHERE duration_seconds = 1)`

	var st LibraryStats
	err := s.db.QueryRowContext(ctx, q).Scan(
		&st.Books, &st.Authors, &st.Series, &st.Users, &st.Chapters,
		&st.TotalDurationSeconds, &st.BooksWithoutCover, &st.BooksWithoutDescription,
		&st.PlaceholderChapters, &st.BooksWithPlaceholderChapters,
	)
	if err != nil {
		return LibraryStats{}, fmt.Errorf("library stats: %w", err)
	}
	return st, nil
}
