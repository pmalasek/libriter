package storage

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound se vrátí, když záznam v DB neexistuje.
var ErrNotFound = errors.New("not found")

// ErrConflict se vrátí při porušení unique constraint.
var ErrConflict = errors.New("conflict")

// Store sdružuje všechny DB operace.
type Store struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}
