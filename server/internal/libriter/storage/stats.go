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
	// DuplicateAlbumBooks je hrubý odhad, kolik knih je v knihovně dvakrát:
	// stejný album tag ve dvou adresářích, z nichž ani jeden není podadresář
	// druhého. Přesnější rozbor dělá kontrola duplicit (ta umí i názvy bez
	// diakritiky a sousedící disky); tady jde jen o to upozornit.
	DuplicateAlbumBooks int `json:"duplicate_album_books"`
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
		  (SELECT COUNT(DISTINCT book_id) FROM chapters WHERE duration_seconds = 1),
		  (SELECT COUNT(*) FROM books b
		     WHERE b.album_tag IS NOT NULL AND b.album_tag <> ''
		       AND EXISTS (SELECT 1 FROM books o
		                    WHERE o.id <> b.id
		                      AND o.album_tag = b.album_tag
		                      AND o.file_path <> b.file_path
		                      -- Podadresáře jsou disky jedné knihy, ne druhá kopie.
		                      AND o.file_path NOT LIKE b.file_path || '/%'
		                      AND b.file_path NOT LIKE o.file_path || '/%')
		       -- Co někdo odmítl jako planý poplach, nemá dál svítit na Přehledu.
		       -- Odmítnutí platí, jen když je klíč pořád úplný: počet řádků musí
		       -- sedět s počtem ID v klíči (čárky + 1). Smazaná kniha svůj řádek
		       -- odnese kaskádou a odmítnutí tím pozbude platnosti.
		       AND NOT EXISTS (
		             SELECT 1 FROM duplicate_dismissals d
		             WHERE d.book_id = b.id
		               AND (SELECT COUNT(*) FROM duplicate_dismissals d2
		                     WHERE d2.group_key = d.group_key)
		                   = length(d.group_key) - length(replace(d.group_key, ',', '')) + 1))`

	var st LibraryStats
	err := s.db.QueryRowContext(ctx, q).Scan(
		&st.Books, &st.Authors, &st.Series, &st.Users, &st.Chapters,
		&st.TotalDurationSeconds, &st.BooksWithoutCover, &st.BooksWithoutDescription,
		&st.PlaceholderChapters, &st.BooksWithPlaceholderChapters, &st.DuplicateAlbumBooks,
	)
	if err != nil {
		return LibraryStats{}, fmt.Errorf("library stats: %w", err)
	}
	return st, nil
}
