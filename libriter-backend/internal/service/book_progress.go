package service

import (
	"context"
	"errors"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// ListProgress vrátí stav knih přihlášeného uživatele – co má rozposlouchané
// a co doposlechnuté. Knihovna si podle toho označí dlaždice.
func (b *BookService) ListProgress(ctx context.Context, userID uuid.UUID) ([]model.BookProgress, error) {
	return b.store.ListBookProgress(ctx, userID)
}

// MarkFinished označí knihu za doposlechnutou ručně. Existenci knihy ověřuje
// dřív, než zapíše – jinak by cizí ID prošlo jako platné.
func (b *BookService) MarkFinished(ctx context.Context, userID, bookID uuid.UUID) error {
	if _, err := b.store.GetBook(ctx, bookID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if err := b.store.SetBookFinished(ctx, userID, bookID); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// ResetProgress vrátí knihu mezi neposlechnuté – oprava chybného označení.
// Uloženou pozici v poslechu to nemění.
func (b *BookService) ResetProgress(ctx context.Context, userID, bookID uuid.UUID) error {
	if _, err := b.store.GetBook(ctx, bookID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return b.store.DeleteBookProgress(ctx, userID, bookID)
}
