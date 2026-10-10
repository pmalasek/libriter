package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"mediater/internal/libriter/model"
)

// Inkrementální synchronizace knihovny (GET /sync/changes). Každá změna
// knihy, autora, série a stavu poslechu dostává z čítače sync_rev pořadové
// číslo rev (triggery v migraci libriter/001); klient si pamatuje poslední
// rev a stahuje jen novější změny.

// DeletedItem je smazaný záznam, který klient musí odstranit u sebe.
type DeletedItem struct {
	Kind string    `json:"kind"` // book | author | series | progress
	ID   uuid.UUID `json:"id"`   // u progress ID knihy
}

// SyncChanges je jedna dávka změn od zadaného rev.
type SyncChanges struct {
	// Rev se pošle jako since v dalším dotazu.
	Rev int64 `json:"rev"`
	// More říká, že za Rev čekají další změny – klient se má hned zeptat znovu.
	More bool `json:"more"`
	// FullResync: historie smazání od since už není úplná (nebo since chybí).
	// Klient zahodí svá data a uloží si tuhle dávku a další jako celou knihovnu.
	FullResync bool                 `json:"full_resync"`
	Books      []model.Book         `json:"books"`
	Authors    []model.Author       `json:"authors"`
	Series     []model.Series       `json:"series"`
	Progress   []model.BookProgress `json:"progress"`
	Deleted    []DeletedItem        `json:"deleted"`
}

// syncRow je jedna změna s rev; fetch ji po sloučení uloží do výsledku.
type syncRow struct {
	rev   int64
	store func(*SyncChanges)
}

// SyncChanges vrátí nejvýš limit změn s rev větším než since. since = 0
// znamená celou knihovnu. Čte v jedné transakci, takže dávka je konzistentní
// snímek i při souběžném scanu.
func (s *Store) SyncChanges(ctx context.Context, userID uuid.UUID, since int64, limit int) (*SyncChanges, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("sync changes: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var current, minRev int64
	if err := tx.QueryRowContext(ctx, `SELECT rev, min_rev FROM sync_rev WHERE id = 1`).Scan(&current, &minRev); err != nil {
		return nil, fmt.Errorf("sync changes: rev: %w", err)
	}

	out := &SyncChanges{
		Books: []model.Book{}, Authors: []model.Author{}, Series: []model.Series{},
		Progress: []model.BookProgress{}, Deleted: []DeletedItem{},
	}
	if since <= 0 || since < minRev || since > current {
		// Bez since, s historií, která už byla uklizena, nebo s rev z jiné
		// databáze (obnova ze zálohy) se stahuje všechno znovu.
		out.FullResync = true
		since = 0
	}

	// Z každé tabulky limit+1 řádků: po sloučení podle rev se vezme prvních
	// limit a zbytek říká, jestli jsou další.
	var rows []syncRow
	fetchers := []func(context.Context, *sql.Tx, int64, int) ([]syncRow, error){
		syncBooks,
		syncAuthors,
		syncSeries,
		func(ctx context.Context, tx *sql.Tx, since int64, n int) ([]syncRow, error) {
			return syncProgress(ctx, tx, userID, since, n)
		},
	}
	if since > 0 {
		// Při úplné synchronizaci klient začíná od prázdna – smazané nepotřebuje.
		fetchers = append(fetchers, func(ctx context.Context, tx *sql.Tx, since int64, n int) ([]syncRow, error) {
			return syncDeleted(ctx, tx, userID, since, n)
		})
	}
	for _, fetch := range fetchers {
		part, err := fetch(ctx, tx, since, limit+1)
		if err != nil {
			return nil, err
		}
		rows = append(rows, part...)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].rev < rows[j].rev })
	if len(rows) > limit {
		rows = rows[:limit]
		out.More = true
	}
	for _, r := range rows {
		r.store(out)
	}
	out.Rev = current
	if out.More {
		out.Rev = rows[len(rows)-1].rev
	}

	// Knihy nesou autory jako /books; doplní se jedním dotazem za dávku.
	if err := fillSyncBookAuthors(ctx, tx, out.Books); err != nil {
		return nil, err
	}
	return out, nil
}

func syncBooks(ctx context.Context, tx *sql.Tx, since int64, n int) ([]syncRow, error) {
	q := `SELECT ` + bookColumns + `, rev FROM books WHERE rev > ?1 ORDER BY rev LIMIT ?2`
	rows, err := tx.QueryContext(ctx, q, since, n)
	if err != nil {
		return nil, fmt.Errorf("sync books: %w", err)
	}
	defer rows.Close()
	var out []syncRow
	for rows.Next() {
		var (
			b   model.Book
			rev int64
		)
		if err := rows.Scan(&b.ID, &b.SeriesID, &b.SeriesPosition, &b.Title, &b.Narrator,
			&b.DurationSeconds, &b.FilePath, &b.CoverPath, &b.Language, &b.Description,
			&b.InternalRating, &b.PublishedYear, &b.CreatedAt, &b.UpdatedAt, &b.AlbumTag,
			&b.ChapterCount, &rev); err != nil {
			return nil, err
		}
		out = append(out, syncRow{rev: rev, store: func(c *SyncChanges) { c.Books = append(c.Books, b) }})
	}
	return out, rows.Err()
}

func syncAuthors(ctx context.Context, tx *sql.Tx, since int64, n int) ([]syncRow, error) {
	q := `SELECT ` + authorColumns + `, rev FROM authors WHERE rev > ?1 ORDER BY rev LIMIT ?2`
	rows, err := tx.QueryContext(ctx, q, since, n)
	if err != nil {
		return nil, fmt.Errorf("sync authors: %w", err)
	}
	defer rows.Close()
	var out []syncRow
	for rows.Next() {
		var (
			a   model.Author
			rev int64
		)
		if err := rows.Scan(&a.ID, &a.FirstName, &a.MiddleName, &a.LastName, &a.Name,
			&a.Bio, &a.ImagePath, &a.BirthYear, &a.DeathYear, &a.CreatedAt, &rev); err != nil {
			return nil, err
		}
		out = append(out, syncRow{rev: rev, store: func(c *SyncChanges) { c.Authors = append(c.Authors, a) }})
	}
	return out, rows.Err()
}

func syncSeries(ctx context.Context, tx *sql.Tx, since int64, n int) ([]syncRow, error) {
	q := `SELECT id, title, description, created_at, rev FROM series WHERE rev > ?1 ORDER BY rev LIMIT ?2`
	rows, err := tx.QueryContext(ctx, q, since, n)
	if err != nil {
		return nil, fmt.Errorf("sync series: %w", err)
	}
	defer rows.Close()
	var out []syncRow
	for rows.Next() {
		var (
			sr  model.Series
			rev int64
		)
		if err := rows.Scan(&sr.ID, &sr.Title, &sr.Description, &sr.CreatedAt, &rev); err != nil {
			return nil, err
		}
		out = append(out, syncRow{rev: rev, store: func(c *SyncChanges) { c.Series = append(c.Series, sr) }})
	}
	return out, rows.Err()
}

func syncProgress(ctx context.Context, tx *sql.Tx, userID uuid.UUID, since int64, n int) ([]syncRow, error) {
	const q = `SELECT book_id, started_at, finished_at, updated_at, rev FROM book_progress
		WHERE user_id = ?1 AND rev > ?2 ORDER BY rev LIMIT ?3`
	rows, err := tx.QueryContext(ctx, q, userID, since, n)
	if err != nil {
		return nil, fmt.Errorf("sync progress: %w", err)
	}
	defer rows.Close()
	var out []syncRow
	for rows.Next() {
		var (
			p        model.BookProgress
			finished sql.NullTime
			rev      int64
		)
		if err := rows.Scan(&p.BookID, &p.StartedAt, &finished, &p.UpdatedAt, &rev); err != nil {
			return nil, err
		}
		p.UserID = userID
		if finished.Valid {
			t := finished.Time
			p.FinishedAt = &t
		}
		out = append(out, syncRow{rev: rev, store: func(c *SyncChanges) { c.Progress = append(c.Progress, p) }})
	}
	return out, rows.Err()
}

func syncDeleted(ctx context.Context, tx *sql.Tx, userID uuid.UUID, since int64, n int) ([]syncRow, error) {
	// Smazaný stav poslechu patří jen jeho uživateli; cizí se neposílá.
	const q = `SELECT rev, kind, id FROM deleted_items
		WHERE rev > ?1 AND (kind <> 'progress' OR user_id = ?2) ORDER BY rev LIMIT ?3`
	rows, err := tx.QueryContext(ctx, q, since, userID, n)
	if err != nil {
		return nil, fmt.Errorf("sync deleted: %w", err)
	}
	defer rows.Close()
	var out []syncRow
	for rows.Next() {
		var (
			d   DeletedItem
			rev int64
		)
		if err := rows.Scan(&rev, &d.Kind, &d.ID); err != nil {
			return nil, err
		}
		out = append(out, syncRow{rev: rev, store: func(c *SyncChanges) { c.Deleted = append(c.Deleted, d) }})
	}
	return out, rows.Err()
}

// fillSyncBookAuthors doplní knihám dávky jejich autory (celé záznamy jako /books).
func fillSyncBookAuthors(ctx context.Context, tx *sql.Tx, books []model.Book) error {
	if len(books) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(books))
	for i := range books {
		ids[i] = books[i].ID
	}
	var args sqlArgs
	q := `SELECT ba.book_id, a.id, a.first_name, a.middle_name, a.last_name, a.name,
	             a.bio, a.image_path, a.birth_year, a.death_year, a.created_at
	      FROM book_authors ba JOIN authors a ON a.id = ba.author_id
	      WHERE ba.book_id IN (` + args.in(ids) + `) ORDER BY ba.book_id, ba.position`
	rows, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("sync book authors: %w", err)
	}
	defer rows.Close()
	byBook := make(map[uuid.UUID][]model.Author, len(books))
	for rows.Next() {
		var (
			bookID uuid.UUID
			a      model.Author
		)
		if err := rows.Scan(&bookID, &a.ID, &a.FirstName, &a.MiddleName, &a.LastName,
			&a.Name, &a.Bio, &a.ImagePath, &a.BirthYear, &a.DeathYear, &a.CreatedAt); err != nil {
			return err
		}
		byBook[bookID] = append(byBook[bookID], a)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range books {
		books[i].Authors = byBook[books[i].ID]
	}
	return nil
}

// PruneDeletedItems smaže tombstony starší než before. Klient, který se
// naposledy synchronizoval před nejnovějším smazaným, pak dostane full_resync.
func (s *Store) PruneDeletedItems(ctx context.Context, before time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("prune deleted items: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var maxRev sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT MAX(rev) FROM deleted_items WHERE deleted_at < ?1`,
		sqliteTime(before)).Scan(&maxRev); err != nil {
		return 0, fmt.Errorf("prune deleted items: %w", err)
	}
	if !maxRev.Valid {
		return 0, nil
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM deleted_items WHERE rev <= ?1`, maxRev.Int64)
	if err != nil {
		return 0, fmt.Errorf("prune deleted items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sync_rev SET min_rev = MAX(min_rev, ?1) WHERE id = 1`, maxRev.Int64); err != nil {
		return 0, fmt.Errorf("prune deleted items: min_rev: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("prune deleted items: commit: %w", err)
	}
	return rowsAffected(res), nil
}
