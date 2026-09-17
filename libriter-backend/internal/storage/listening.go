package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ListeningSummary je řádek přehledu poslechu v administraci – jeden
// uživatel se vším, co se o jeho poslechu dá spočítat jedním dotazem.
type ListeningSummary struct {
	UserID           uuid.UUID `json:"user_id"`
	DisplayName      string    `json:"display_name"`
	Email            string    `json:"email"`
	OpenSessions     int       `json:"open_sessions"`
	FinishedSessions int       `json:"finished_sessions"`
	FinishedBooks    int       `json:"finished_books"`
	SecondsListened  int64     `json:"seconds_listened"`
	// LastListenedAt je poslední aktivita – zápis pozice nebo deníku;
	// chybí u účtu, který zatím nic nepustil.
	LastListenedAt *time.Time `json:"last_listened_at,omitempty"`
}

// ListeningBookTotal je součet deníku za jednu knihu uživatele.
type ListeningBookTotal struct {
	BookID          uuid.UUID `json:"book_id"`
	SecondsListened int64     `json:"seconds_listened"`
	FirstAt         time.Time `json:"first_at"`
	LastAt          time.Time `json:"last_at"`
}

// ListeningDay je jeden řádek deníku: kniha a den.
type ListeningDay struct {
	Day             string    `json:"day"`
	BookID          uuid.UUID `json:"book_id"`
	SecondsListened int64     `json:"seconds_listened"`
}

// addListeningLog připíše sekundy ke dni, ve kterém poslech proběhl (UTC).
// Volá se uvnitř transakce zápisu pozice, proto bere querier místo *Store.
//
// Dnem je datum z `at`, ne ze serverových hodin: dávka z telefonu, který byl
// přes noc offline, patří do včerejška. Webový přehrávač posílá zápisy hned,
// takže mu `at` vychází na serverové "teď" jako dřív.
func addListeningLog(ctx context.Context, q querier, userID, bookID uuid.UUID, seconds int, at time.Time) error {
	const query = `
		INSERT INTO listening_log (user_id, book_id, day, seconds_listened, first_at, last_at)
		VALUES (?1, ?2, date(?4), ?3, ?4, ?4)
		ON CONFLICT (user_id, book_id, day) DO UPDATE SET
		  seconds_listened = seconds_listened + excluded.seconds_listened,
		  first_at         = MIN(first_at, excluded.first_at),
		  last_at          = MAX(last_at, excluded.last_at)`

	if _, err := q.ExecContext(ctx, query, userID, bookID, seconds, sqliteTime(at)); err != nil {
		if isForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("add listening log: %w", err)
	}
	return nil
}

// ListListeningSummaries vrátí přehled poslechu všech uživatelů: nejdřív
// naposledy aktivní, účty bez poslechu na konci podle jména.
func (s *Store) ListListeningSummaries(ctx context.Context) ([]ListeningSummary, error) {
	const q = `
		SELECT u.id, u.display_name, u.email,
		  (SELECT COUNT(*) FROM play_sessions ps WHERE ps.user_id = u.id AND ps.finished_at IS NULL),
		  (SELECT COUNT(*) FROM play_sessions ps WHERE ps.user_id = u.id AND ps.finished_at IS NOT NULL),
		  (SELECT COUNT(*) FROM book_progress bp WHERE bp.user_id = u.id AND bp.finished_at IS NOT NULL),
		  (SELECT COALESCE(SUM(seconds_listened), 0) FROM listening_log l WHERE l.user_id = u.id),
		  NULLIF(MAX(
		    COALESCE((SELECT MAX(updated_at) FROM play_sessions ps WHERE ps.user_id = u.id), ''),
		    COALESCE((SELECT MAX(last_at)    FROM listening_log l  WHERE l.user_id = u.id), '')
		  ), '') AS last_listened_at
		FROM users u
		ORDER BY last_listened_at IS NULL, last_listened_at DESC, u.display_name`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list listening summaries: %w", err)
	}
	defer rows.Close()

	summaries := []ListeningSummary{}
	for rows.Next() {
		var (
			sum  ListeningSummary
			last sql.NullString
		)
		if err := rows.Scan(&sum.UserID, &sum.DisplayName, &sum.Email,
			&sum.OpenSessions, &sum.FinishedSessions, &sum.FinishedBooks,
			&sum.SecondsListened, &last); err != nil {
			return nil, fmt.Errorf("list listening summaries: %w", err)
		}
		if last.Valid {
			t, err := parseSQLiteTime(last.String)
			if err != nil {
				return nil, fmt.Errorf("list listening summaries: last_listened_at: %w", err)
			}
			sum.LastListenedAt = &t
		}
		summaries = append(summaries, sum)
	}
	return summaries, rows.Err()
}

// ListListeningBookTotals vrátí součty deníku uživatele po knihách, od
// naposledy poslouchané.
func (s *Store) ListListeningBookTotals(ctx context.Context, userID uuid.UUID) ([]ListeningBookTotal, error) {
	const q = `
		SELECT book_id, SUM(seconds_listened), MIN(first_at), MAX(last_at)
		FROM listening_log
		WHERE user_id = ?1
		GROUP BY book_id
		ORDER BY MAX(last_at) DESC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list listening book totals: %w", err)
	}
	defer rows.Close()

	totals := []ListeningBookTotal{}
	for rows.Next() {
		var (
			total       ListeningBookTotal
			first, last string
		)
		if err := rows.Scan(&total.BookID, &total.SecondsListened, &first, &last); err != nil {
			return nil, fmt.Errorf("list listening book totals: %w", err)
		}
		if total.FirstAt, err = parseSQLiteTime(first); err != nil {
			return nil, fmt.Errorf("list listening book totals: first_at: %w", err)
		}
		if total.LastAt, err = parseSQLiteTime(last); err != nil {
			return nil, fmt.Errorf("list listening book totals: last_at: %w", err)
		}
		totals = append(totals, total)
	}
	return totals, rows.Err()
}

// ListListeningDays vrátí deník uživatele za posledních `days` dní, od
// nejnovějšího dne a v něm od nejposlouchanější knihy.
func (s *Store) ListListeningDays(ctx context.Context, userID uuid.UUID, days int) ([]ListeningDay, error) {
	const q = `
		SELECT day, book_id, seconds_listened
		FROM listening_log
		WHERE user_id = ?1 AND day >= date('now', ?2)
		ORDER BY day DESC, seconds_listened DESC`

	rows, err := s.db.QueryContext(ctx, q, userID, fmt.Sprintf("-%d days", days))
	if err != nil {
		return nil, fmt.Errorf("list listening days: %w", err)
	}
	defer rows.Close()

	entries := []ListeningDay{}
	for rows.Next() {
		var entry ListeningDay
		if err := rows.Scan(&entry.Day, &entry.BookID, &entry.SecondsListened); err != nil {
			return nil, fmt.Errorf("list listening days: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Formáty, ve kterých SQLite vrací DATETIME z výrazu: CURRENT_TIMESTAMP bez
// zóny a hodnoty zapsané driverem i se zlomkem sekundy a zónou.
var sqliteTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04:05.999999999-07:00",
	time.RFC3339Nano,
}

// parseSQLiteTime čte čas z výrazu jako MAX(updated_at). Driver převádí na
// time.Time jen sloupce deklarované jako DATETIME; výraz deklaraci nemá, a
// tak přijde jako text.
func parseSQLiteTime(value string) (time.Time, error) {
	for _, layout := range sqliteTimeLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("neznámý formát času %q", value)
}
