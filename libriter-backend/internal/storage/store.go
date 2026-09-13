package storage

import (
	"database/sql"
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrNotFound se vrátí, když záznam v DB neexistuje.
var ErrNotFound = errors.New("not found")

// ErrConflict se vrátí při porušení unique constraint.
var ErrConflict = errors.New("conflict")

// Store sdružuje všechny DB operace.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
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

// rowsAffected vrátí počet ovlivněných řádků; při chybě 0.
func rowsAffected(res sql.Result) int64 {
	n, err := res.RowsAffected()
	if err != nil {
		return 0
	}
	return n
}
