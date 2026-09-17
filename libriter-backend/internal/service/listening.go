package service

import (
	"context"
	"errors"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// Kolik dní zpět vrací detail uživatele rozepsaných po dnech. Delší historie
// zůstává v součtech po knihách; tabulku po dnech už by nikdo nepřečetl.
const listeningDetailDays = 90

// ListeningService čte poslech uživatelů pro administraci. Jen čtení –
// zapisuje se při ukládání pozice (PlaySessionService).
type ListeningService struct {
	store *storage.Store
}

func NewListening(store *storage.Store) *ListeningService {
	return &ListeningService{store: store}
}

// ListeningDetail je vše, co administrace ukazuje o poslechu jednoho uživatele.
type ListeningDetail struct {
	User     *model.User                  `json:"user"`
	Sessions []model.PlaySession          `json:"sessions"`
	Progress []model.BookProgress         `json:"progress"`
	Books    []storage.ListeningBookTotal `json:"books"`
	Days     []storage.ListeningDay       `json:"days"`
}

// Overview vrátí přehled poslechu všech uživatelů.
func (l *ListeningService) Overview(ctx context.Context) ([]storage.ListeningSummary, error) {
	return l.store.ListListeningSummaries(ctx)
}

// UserDetail vrátí poslechy, stav knih a deník jednoho uživatele.
func (l *ListeningService) UserDetail(ctx context.Context, userID uuid.UUID) (*ListeningDetail, error) {
	user, err := l.store.GetUserByID(ctx, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	sessions, err := l.store.ListPlaySessions(ctx, userID)
	if err != nil {
		return nil, err
	}
	// ListPlaySessions vrací nil, když uživatel nic neposlouchá – rozhraní
	// čeká pole, ne null.
	if sessions == nil {
		sessions = []model.PlaySession{}
	}

	progress, err := l.store.ListBookProgress(ctx, userID)
	if err != nil {
		return nil, err
	}
	books, err := l.store.ListListeningBookTotals(ctx, userID)
	if err != nil {
		return nil, err
	}
	days, err := l.store.ListListeningDays(ctx, userID, listeningDetailDays)
	if err != nil {
		return nil, err
	}

	return &ListeningDetail{
		User:     user,
		Sessions: sessions,
		Progress: progress,
		Books:    books,
		Days:     days,
	}, nil
}
