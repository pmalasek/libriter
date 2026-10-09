package storage

import (
	"database/sql"
	"errors"

	corestorage "mediater/internal/core/storage"
)

// Chyby jádra pod původními jmény – errors.Is funguje, jde o tytéž hodnoty.
var (
	ErrNotFound      = corestorage.ErrNotFound
	ErrConflict      = corestorage.ErrConflict
	ErrLoginConflict = corestorage.ErrLoginConflict
	ErrLastAdmin     = corestorage.ErrLastAdmin
)

// ErrChapterSetMismatch vrací ReorderChapters, když poslaný seznam ID
// neodpovídá kapitolám knihy (chybějící, duplicitní nebo cizí kapitola).
var ErrChapterSetMismatch = errors.New("chapter set mismatch")

// Store sdružuje DB operace knihovny audioknih. Embeduje Store jádra, takže
// volání jako store.GetUserByID zůstávají beze změny.
type Store struct {
	*corestorage.Store
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{Store: corestorage.New(db), db: db}
}

// Core vrátí Store jádra pro služby jádra (uživatelé, auth, audit).
func (s *Store) Core() *corestorage.Store { return s.Store }

type querier = corestorage.Querier

var (
	isUniqueViolation     = corestorage.IsUniqueViolation
	isForeignKeyViolation = corestorage.IsForeignKeyViolation
	rowsAffected          = corestorage.RowsAffected
	insertSyncEvent       = corestorage.InsertSyncEvent
)

type scanner = corestorage.RowScanner

var sqliteTime = corestorage.SQLiteTime
var ErrDuplicateEvent = corestorage.ErrDuplicateEvent
