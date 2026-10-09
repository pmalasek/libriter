// Package storage obsahuje SQL dotazy Sceniteru nad tabulkami sc_*.
// Na tabulky jádra sahá jen přes cizí klíče na users(id).
package storage

import (
	"database/sql"
	"strings"

	corestorage "mediater/internal/core/storage"
)

var (
	ErrNotFound = corestorage.ErrNotFound
	ErrConflict = corestorage.ErrConflict
)

// Store sdružuje DB operace videoknihovny.
type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

type querier = corestorage.Querier

var isUniqueViolation = corestorage.IsUniqueViolation

// likePattern připraví hodnotu pro LIKE … ESCAPE '\' (hledání podřetězce).
func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.TrimSpace(q)) + "%"
}

// nullStr převede prázdný řetězec na NULL.
func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// nullInt převede nulu na NULL.
func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

func ptrString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func ptrInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func ptrInt64(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

func ptrFloat(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}
