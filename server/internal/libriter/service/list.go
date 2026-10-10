package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"mediater/internal/libriter/model"
	"mediater/internal/libriter/storage"

	"github.com/google/uuid"
)

// ListPage vrátí stránku knih podle filtru a počet všech, které mu odpovídají.
func (b *BookService) ListPage(ctx context.Context, f storage.BookFilter, limit, offset int) ([]model.BookSummary, int, error) {
	return b.store.ListBookSummaries(ctx, f, limit, offset)
}

// CoverPath vrátí cestu k obálce knihy (relativní ke COVER_ROOT).
func (b *BookService) CoverPath(ctx context.Context, id uuid.UUID) (*string, error) {
	return notFoundAs(b.store.CoverPath(ctx, id))
}

// ListPage vrátí stránku autorů a jejich celkový počet.
func (a *AuthorService) ListPage(ctx context.Context, f storage.AuthorFilter, limit, offset int) ([]model.AuthorSummary, int, error) {
	return a.store.ListAuthorSummaries(ctx, f, limit, offset)
}

// ImagePath vrátí cestu k fotce autora (relativní k AUTHOR_IMAGE_ROOT).
func (a *AuthorService) ImagePath(ctx context.Context, id uuid.UUID) (*string, error) {
	return notFoundAs(a.store.AuthorImagePath(ctx, id))
}

// ListPage vrátí stránku sérií a jejich celkový počet.
func (s *SeriesService) ListPage(ctx context.Context, f storage.SeriesFilter, limit, offset int) ([]model.SeriesSummary, int, error) {
	return s.store.ListSeriesSummaries(ctx, f, limit, offset)
}

func notFoundAs[T any](v T, err error) (T, error) {
	if errors.Is(err, storage.ErrNotFound) {
		return v, ErrNotFound
	}
	return v, err
}

// Synchronizace knihovny do zařízení (GET /sync/changes).

// tombstoneTTL je, jak dlouho se drží záznamy o smazaných knihách, autorech
// a sériích. Zařízení offline déle stáhne knihovnu znovu celou.
const tombstoneTTL = 90 * 24 * time.Hour

// pruneEvery omezuje úklid tombstonů – stačí jednou za čas, ne při každém dotazu.
const pruneEvery = time.Hour

// LibrarySyncService vydává změny knihovny od posledního rev zařízení.
type LibrarySyncService struct {
	store *storage.Store

	mu         sync.Mutex
	lastPruned time.Time
}

func NewLibrarySync(store *storage.Store) *LibrarySyncService {
	return &LibrarySyncService{store: store}
}

// Changes vrátí nejvýš limit změn od since (0 = celá knihovna).
func (l *LibrarySyncService) Changes(ctx context.Context, userID uuid.UUID, since int64, limit int) (*storage.SyncChanges, error) {
	l.prune(ctx)
	return l.store.SyncChanges(ctx, userID, since, limit)
}

func (l *LibrarySyncService) prune(ctx context.Context) {
	l.mu.Lock()
	due := time.Since(l.lastPruned) >= pruneEvery
	if due {
		l.lastPruned = time.Now()
	}
	l.mu.Unlock()
	if !due {
		return
	}
	if n, err := l.store.PruneDeletedItems(ctx, time.Now().Add(-tombstoneTTL)); err != nil {
		// Neuklizené tombstony jen zabírají místo; synchronizaci neblokují.
		slog.Warn("úklid smazaných záznamů selhal", "err", err)
	} else if n > 0 {
		slog.Debug("smazané záznamy uklizeny", "počet", n)
	}
}
