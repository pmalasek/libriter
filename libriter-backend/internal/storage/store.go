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

// ErrLastAdmin se vrátí při pokusu odebrat roli nebo smazat účet posledního
// administrátora. Bez admina by knihovnu nešlo spravovat jinak než přes CLI.
var ErrLastAdmin = errors.New("last admin")

// ErrChapterSetMismatch vrací ReorderChapters, když poslaný seznam ID
// neodpovídá kapitolám knihy (chybějící, duplicitní nebo cizí kapitola).
var ErrChapterSetMismatch = errors.New("chapter set mismatch")

// Store sdružuje všechny DB operace.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// querier pokrývá *sql.DB i *sql.Tx – dotaz tak lze spustit uvnitř
// transakce i mimo ni.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// isUniqueViolation vrátí true, pokud chyba pochází z porušení
// UNIQUE nebo PRIMARY KEY omezení v SQLite.
func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	code := se.Code()
	return code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}

// isForeignKeyViolation vrátí true, pokud chyba pochází z porušení
// cizího klíče (např. mazání autora, který má v knihovně knihy).
func isForeignKeyViolation(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	return se.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY
}

// rowsAffected vrátí počet ovlivněných řádků; při chybě 0.
func rowsAffected(res sql.Result) int64 {
	n, err := res.RowsAffected()
	if err != nil {
		return 0
	}
	return n
}
