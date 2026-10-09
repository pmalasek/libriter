package storage

import (
	"context"
	"database/sql"
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrNotFound se vrátí, když záznam v DB neexistuje.
var ErrNotFound = errors.New("not found")

// ErrConflict se vrátí při porušení unique constraint.
var ErrConflict = errors.New("conflict")

// ErrLoginConflict se vrátí, když je přihlašovací jméno už obsazené jiným
// uživatelem (u uživatelů odlišuje obsazený login od obsazeného e-mailu).
var ErrLoginConflict = errors.New("login conflict")

// ErrLastAdmin se vrátí při pokusu odebrat roli nebo smazat účet posledního
// administrátora. Bez admina by knihovnu nešlo spravovat jinak než přes CLI.
var ErrLastAdmin = errors.New("last admin")

// Store sdružuje DB operace jádra: uživatelé, role, nastavení, audit a
// idempotence synchronizačních událostí. Produkty mají vlastní Store, který
// tento embeduje.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB vrátí sdílené spojení – produktové storage nad ním staví vlastní dotazy.
func (s *Store) DB() *sql.DB { return s.db }

// Querier pokrývá *sql.DB i *sql.Tx – dotaz tak lze spustit uvnitř
// transakce i mimo ni.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// querier je interní zkratka pro Querier.
type querier = Querier

// IsUniqueViolation vrátí true, pokud chyba pochází z porušení
// UNIQUE nebo PRIMARY KEY omezení v SQLite.
func IsUniqueViolation(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	code := se.Code()
	return code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}

// IsForeignKeyViolation vrátí true, pokud chyba pochází z porušení
// cizího klíče (např. mazání autora, který má v knihovně knihy).
func IsForeignKeyViolation(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	return se.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

// RowsAffected vrátí počet ovlivněných řádků; při chybě 0.
func RowsAffected(res sql.Result) int64 {
	n, err := res.RowsAffected()
	if err != nil {
		return 0
	}
	return n
}
