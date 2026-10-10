package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"mediater/internal/core/textfold"
	"mediater/internal/libriter/model"
)

// Stránkované výpisy knihovny. Řadí a filtrují se v SQL přes indexy
// z migrace libriter/001, takže stránka z konce velké knihovny stojí
// zhruba stejně jako ta první.

// sqlArgs skládá číslované parametry (?1, ?2, …) pro dotaz sestavovaný
// z volitelných podmínek.
type sqlArgs []any

func (a *sqlArgs) add(v any) string {
	*a = append(*a, v)
	return fmt.Sprintf("?%d", len(*a))
}

// in složí "?1, ?2, …" pro podmínku IN.
func (a *sqlArgs) in(ids []uuid.UUID) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = a.add(id)
	}
	return strings.Join(parts, ", ")
}

// maxSearchWords omezuje počet slov hledaného textu – každé slovo je jeden
// průchod tabulkou.
const maxSearchWords = 6

// searchWords rozdělí hledaný text na slova (nejvýš maxSearchWords).
func searchWords(q string) []string {
	words := strings.Fields(q)
	if len(words) > maxSearchWords {
		words = words[:maxSearchWords]
	}
	return words
}

// Klíče řazení knih (shodné s BookSortKey v libriter-shared).
const (
	BookSortTitle     = "title"
	BookSortAuthor    = "author"
	BookSortAdded     = "added"
	BookSortPublished = "published"
)

// BookFilter vybírá a řadí knihy do výpisu. Prázdné pole nefiltruje.
type BookFilter struct {
	Query    string // hledá v názvu a jménech autorů bez ohledu na diakritiku
	AuthorID uuid.UUID
	SeriesID uuid.UUID
	Language string
	IDs      []uuid.UUID
	Sort     string // BookSort*; neznámý klíč = podle názvu
	Desc     bool
}

// Pořadí knih pro jednotlivé klíče. Každé končí jedinečným id, aby bylo
// stránkování stabilní. Odpovídá indexům books_sort_*_idx; sestupné řazení
// je obrácení všech složek (index se projde pozpátku).
var bookOrder = map[string][]string{
	BookSortTitle:  {"b.sort_name", "b.series_position", "b.title", "b.id"},
	BookSortAuthor: {"(b.author_sort = '')", "b.author_sort", "b.sort_name", "b.series_position", "b.title", "b.id"},
	BookSortAdded:  {"b.created_at", "b.sort_name", "b.series_position", "b.title", "b.id"},
	BookSortPublished: {"(b.published_year IS NULL)", "b.published_year", "(b.author_sort = '')", "b.author_sort",
		"b.sort_name", "b.series_position", "b.title", "b.id"},
}

func orderBy(cols []string, desc bool) string {
	if !desc {
		return strings.Join(cols, ", ")
	}
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c + " DESC"
	}
	return strings.Join(out, ", ")
}

func (f BookFilter) where(args *sqlArgs) string {
	conds := []string{"1 = 1"}
	// Každé slovo hledaného textu musí sedět na název nebo na některého
	// z autorů – „capek valka“ najde Válku s mloky od Čapka.
	for _, word := range searchWords(f.Query) {
		p := args.add(textfold.LikePattern(word))
		conds = append(conds, fmt.Sprintf(`(fold(b.title) LIKE %[1]s ESCAPE '\'
			OR b.id IN (SELECT ba.book_id FROM book_authors ba
			            WHERE ba.author_id IN (SELECT a.id FROM authors a WHERE fold(a.name) LIKE %[1]s ESCAPE '\')))`, p))
	}
	if f.AuthorID != uuid.Nil {
		conds = append(conds, "b.id IN (SELECT book_id FROM book_authors WHERE author_id = "+args.add(f.AuthorID)+")")
	}
	if f.SeriesID != uuid.Nil {
		conds = append(conds, "b.series_id = "+args.add(f.SeriesID))
	}
	if f.Language != "" {
		conds = append(conds, "b.language = "+args.add(f.Language))
	}
	if f.IDs != nil {
		if len(f.IDs) == 0 {
			conds = append(conds, "0 = 1")
		} else {
			conds = append(conds, "b.id IN ("+args.in(f.IDs)+")")
		}
	}
	return strings.Join(conds, " AND ")
}

// ListBookSummaries vrátí stránku knih podle filtru a celkový počet knih,
// které filtru odpovídají.
func (s *Store) ListBookSummaries(ctx context.Context, f BookFilter, limit, offset int) ([]model.BookSummary, int, error) {
	var args sqlArgs
	where := f.where(&args)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM books b WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count books: %w", err)
	}
	if total == 0 || offset >= total {
		return []model.BookSummary{}, total, nil
	}

	order, ok := bookOrder[f.Sort]
	if !ok {
		order = bookOrder[BookSortTitle]
	}
	orderSQL := orderBy(order, f.Desc)
	if f.Sort == BookSortPublished && f.Desc {
		// Knihy bez roku zůstávají na konci i při sestupném řazení a při
		// shodě roku se řadí podle autora vzestupně (index books_sort_published_desc_idx).
		orderSQL = "(b.published_year IS NULL), b.published_year DESC, " + orderBy(order[2:], false)
	}
	q := `SELECT b.id, b.title, b.series_id, b.series_position, s.title, b.duration_seconds,
	             (SELECT COUNT(*) FROM chapters c WHERE c.book_id = b.id), b.cover_path, b.language, b.published_year, b.created_at, b.updated_at
	      FROM books b LEFT JOIN series s ON s.id = b.series_id
	      WHERE ` + where + `
	      ORDER BY ` + orderSQL + `
	      LIMIT ` + args.add(limit) + ` OFFSET ` + args.add(offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list books: %w", err)
	}
	defer rows.Close()

	books := []model.BookSummary{}
	for rows.Next() {
		var b model.BookSummary
		if err := rows.Scan(&b.ID, &b.Title, &b.SeriesID, &b.SeriesPosition, &b.SeriesTitle, &b.DurationSeconds,
			&b.ChapterCount, &b.CoverPath, &b.Language, &b.PublishedYear, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, 0, err
		}
		b.Authors = []model.AuthorRef{}
		books = append(books, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	ids := make([]uuid.UUID, len(books))
	for i := range books {
		ids[i] = books[i].ID
	}
	byBook, err := s.authorRefsByBook(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range books {
		if refs := byBook[books[i].ID]; refs != nil {
			books[i].Authors = refs
		}
	}
	return books, total, nil
}

// authorRefsByBook načte autory zadaných knih v pořadí podle position.
func (s *Store) authorRefsByBook(ctx context.Context, bookIDs []uuid.UUID) (map[uuid.UUID][]model.AuthorRef, error) {
	out := make(map[uuid.UUID][]model.AuthorRef, len(bookIDs))
	if len(bookIDs) == 0 {
		return out, nil
	}
	var args sqlArgs
	q := `SELECT ba.book_id, a.id, a.name FROM book_authors ba JOIN authors a ON a.id = ba.author_id
	      WHERE ba.book_id IN (` + args.in(bookIDs) + `) ORDER BY ba.book_id, ba.position`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("book authors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			bookID uuid.UUID
			a      model.AuthorRef
		)
		if err := rows.Scan(&bookID, &a.ID, &a.Name); err != nil {
			return nil, err
		}
		out[bookID] = append(out[bookID], a)
	}
	return out, rows.Err()
}

// Klíče řazení autorů (shodné s AuthorSortKey v libriter-shared).
const (
	AuthorSortLastName  = "last_name"
	AuthorSortFirstName = "first_name"
	AuthorSortBooks     = "books"
)

// AuthorFilter vybírá a řadí autory do výpisu. Ve výpisu jsou jen autoři,
// kteří mají v knihovně aspoň jednu knihu – ostatní zůstávají po
// přesunech a importech a jen by překáželi.
type AuthorFilter struct {
	Query string
	IDs   []uuid.UUID
	Sort  string // AuthorSort*; neznámý klíč = podle příjmení
	Desc  bool
	// WithoutBooks zahrne i autory bez knih (výběr autora při úpravě knihy).
	WithoutBooks bool
}

var authorOrder = map[string][]string{
	AuthorSortLastName: {"a.last_name", "a.first_name", "a.middle_name", "a.id"},
	AuthorSortFirstName: {"(CASE WHEN a.first_name = '' THEN a.last_name ELSE a.first_name END) COLLATE czech",
		"a.last_name", "a.first_name", "a.middle_name", "a.id"},
	AuthorSortBooks: {"book_count", "a.last_name", "a.first_name", "a.middle_name", "a.id"},
}

// ListAuthorSummaries vrátí stránku autorů a jejich celkový počet.
func (s *Store) ListAuthorSummaries(ctx context.Context, f AuthorFilter, limit, offset int) ([]model.AuthorSummary, int, error) {
	var args sqlArgs
	conds := []string{"1 = 1"}
	if !f.WithoutBooks {
		conds = append(conds, "EXISTS (SELECT 1 FROM book_authors ba WHERE ba.author_id = a.id)")
	}
	// Slova jména v libovolném pořadí – „capek karel“ i „karel c“.
	for _, word := range searchWords(f.Query) {
		conds = append(conds, `fold(a.name) LIKE `+args.add(textfold.LikePattern(word))+` ESCAPE '\'`)
	}
	if f.IDs != nil {
		if len(f.IDs) == 0 {
			conds = append(conds, "0 = 1")
		} else {
			conds = append(conds, "a.id IN ("+args.in(f.IDs)+")")
		}
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM authors a WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count authors: %w", err)
	}
	if total == 0 || offset >= total {
		return []model.AuthorSummary{}, total, nil
	}

	order, ok := authorOrder[f.Sort]
	if !ok {
		order = authorOrder[AuthorSortLastName]
	}
	cols := order
	if f.Sort == AuthorSortBooks {
		// Při shodě počtu drží katalogové pořadí bez ohledu na směr.
		cols = order[:1]
	}
	orderSQL := orderBy(cols, f.Desc)
	if f.Sort == AuthorSortBooks {
		orderSQL += ", " + orderBy(order[1:], false)
	}

	q := `SELECT a.id, a.first_name, a.middle_name, a.last_name, a.name, a.image_path, a.birth_year, a.death_year,
	             (SELECT COUNT(*) FROM book_authors ba WHERE ba.author_id = a.id) AS book_count
	      FROM authors a WHERE ` + where + `
	      ORDER BY ` + orderSQL + `
	      LIMIT ` + args.add(limit) + ` OFFSET ` + args.add(offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list authors: %w", err)
	}
	defer rows.Close()

	authors := []model.AuthorSummary{}
	for rows.Next() {
		var a model.AuthorSummary
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.Name, &a.ImagePath,
			&a.BirthYear, &a.DeathYear, &a.BookCount); err != nil {
			return nil, 0, err
		}
		authors = append(authors, a)
	}
	return authors, total, rows.Err()
}

// SeriesFilter vybírá série do výpisu (podle názvu). Prázdné série se
// stejně jako autoři bez knih nevypisují.
type SeriesFilter struct {
	Query string // hledá v názvu série a jménech jejích autorů
	IDs   []uuid.UUID
	// WithoutBooks zahrne i prázdné série (výběr série při úpravě knihy).
	WithoutBooks bool
}

// seriesCoverCount je počet prvních dílů, jejichž obálky nese karta série.
const seriesCoverCount = 3

// ListSeriesSummaries vrátí stránku sérií seřazených podle názvu a jejich
// celkový počet.
func (s *Store) ListSeriesSummaries(ctx context.Context, f SeriesFilter, limit, offset int) ([]model.SeriesSummary, int, error) {
	var args sqlArgs
	conds := []string{"1 = 1"}
	if !f.WithoutBooks {
		conds = append(conds, "EXISTS (SELECT 1 FROM books b WHERE b.series_id = sr.id)")
	}
	for _, word := range searchWords(f.Query) {
		p := args.add(textfold.LikePattern(word))
		conds = append(conds, fmt.Sprintf(`(fold(sr.title) LIKE %[1]s ESCAPE '\'
			OR sr.id IN (SELECT b.series_id FROM books b JOIN book_authors ba ON ba.book_id = b.id
			             WHERE b.series_id IS NOT NULL
			               AND ba.author_id IN (SELECT a.id FROM authors a WHERE fold(a.name) LIKE %[1]s ESCAPE '\')))`, p))
	}
	if f.IDs != nil {
		if len(f.IDs) == 0 {
			conds = append(conds, "0 = 1")
		} else {
			conds = append(conds, "sr.id IN ("+args.in(f.IDs)+")")
		}
	}
	where := strings.Join(conds, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM series sr WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count series: %w", err)
	}
	if total == 0 || offset >= total {
		return []model.SeriesSummary{}, total, nil
	}

	q := `SELECT sr.id, sr.title, (SELECT COUNT(*) FROM books b WHERE b.series_id = sr.id)
	      FROM series sr WHERE ` + where + `
	      ORDER BY sr.title, sr.id
	      LIMIT ` + args.add(limit) + ` OFFSET ` + args.add(offset)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list series: %w", err)
	}
	list := []model.SeriesSummary{}
	for rows.Next() {
		sr := model.SeriesSummary{Authors: []model.AuthorRef{}, Covers: []model.SeriesCover{}}
		if err := rows.Scan(&sr.ID, &sr.Title, &sr.BookCount); err != nil {
			rows.Close()
			return nil, 0, err
		}
		list = append(list, sr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if err := s.fillSeriesDetails(ctx, list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// fillSeriesDetails doplní sériím na stránce autory a první díly.
func (s *Store) fillSeriesDetails(ctx context.Context, list []model.SeriesSummary) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(list))
	index := make(map[uuid.UUID]*model.SeriesSummary, len(list))
	for i := range list {
		ids[i] = list[i].ID
		index[list[i].ID] = &list[i]
	}

	// Autoři série podle počtu dílů – u série psané ve dvou stojí vepředu
	// hlavní autor, host na jednom dílu za ním.
	var args sqlArgs
	q := `SELECT b.series_id, a.id, a.name, COUNT(*) AS n
	      FROM books b JOIN book_authors ba ON ba.book_id = b.id JOIN authors a ON a.id = ba.author_id
	      WHERE b.series_id IN (` + args.in(ids) + `)
	      GROUP BY b.series_id, a.id
	      ORDER BY b.series_id, n DESC, a.last_name, a.first_name, a.middle_name`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("series authors: %w", err)
	}
	for rows.Next() {
		var (
			seriesID uuid.UUID
			a        model.AuthorRef
			n        int
		)
		if err := rows.Scan(&seriesID, &a.ID, &a.Name, &n); err != nil {
			rows.Close()
			return err
		}
		if sr := index[seriesID]; sr != nil {
			sr.Authors = append(sr.Authors, a)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// První díly pro stoh obálek; díly bez pořadí jdou na konec.
	args = nil
	q = `SELECT series_id, id, title, series_position, cover_path, updated_at FROM (
	       SELECT b.series_id, b.id, b.title, b.series_position, b.cover_path, b.updated_at,
	              ROW_NUMBER() OVER (PARTITION BY b.series_id
	                                 ORDER BY b.series_position IS NULL, b.series_position, b.title) AS rn
	       FROM books b WHERE b.series_id IN (` + args.in(ids) + `))
	     WHERE rn <= ` + args.add(seriesCoverCount) + ` ORDER BY series_id, rn`
	rows, err = s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("series covers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			seriesID uuid.UUID
			c        model.SeriesCover
		)
		if err := rows.Scan(&seriesID, &c.ID, &c.Title, &c.SeriesPosition, &c.CoverPath, &c.UpdatedAt); err != nil {
			return err
		}
		if sr := index[seriesID]; sr != nil {
			sr.Covers = append(sr.Covers, c)
		}
	}
	return rows.Err()
}

// CoverPath vrátí jen cestu k obálce knihy – obálky se v mřížce načítají po
// desítkách a celá kniha s autory by byla zbytečná práce navíc.
func (s *Store) CoverPath(ctx context.Context, bookID uuid.UUID) (*string, error) {
	var path *string
	err := s.db.QueryRowContext(ctx, `SELECT cover_path FROM books WHERE id = ?1`, bookID).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return path, err
}

// AuthorImagePath vrátí jen cestu k fotce autora.
func (s *Store) AuthorImagePath(ctx context.Context, authorID uuid.UUID) (*string, error) {
	var path *string
	err := s.db.QueryRowContext(ctx, `SELECT image_path FROM authors WHERE id = ?1`, authorID).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return path, err
}
