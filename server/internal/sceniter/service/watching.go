package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	coremodel "mediater/internal/core/model"
	coreservice "mediater/internal/core/service"
	"mediater/internal/sceniter/storage"
)

// Kolik dní zpět vrací detail uživatele, když si klient neřekne jinak, a
// kolik nejvýš.
const (
	WatchingDefaultDays = 90
	WatchingMaxDays     = 365
)

// UserDirectory je část účtů jádra, kterou statistika sledování potřebuje.
// Úložiště Sceniteru tabulku users nečte (viz storage/store.go), jména a
// e-maily proto dodá jádro (coreservice.UserService).
type UserDirectory interface {
	List(ctx context.Context) ([]coremodel.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*coremodel.User, error)
}

// Watching čte historii sledování pro administraci. Jen čtení – zapisuje
// se při ukládání pozice (storage.SaveProgress).
type Watching struct {
	store *storage.Store
	users UserDirectory
}

func NewWatching(store *storage.Store, users UserDirectory) *Watching {
	return &Watching{store: store, users: users}
}

// WatchingUser je účet, jak ho ukazuje statistika sledování.
type WatchingUser struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
}

func watchingUser(u coremodel.User) WatchingUser {
	return WatchingUser{ID: u.ID, DisplayName: u.DisplayName, Email: u.Email, Role: u.Role}
}

// WatchingSummary je řádek přehledu sledování: jeden uživatel a součty jeho
// historie. Účet, který zatím nic nesledoval, má nuly a last_watched_at chybí.
type WatchingSummary struct {
	UserID        uuid.UUID  `json:"user_id"`
	DisplayName   string     `json:"display_name"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	Seconds       int64      `json:"seconds_watched"`
	Movies        int        `json:"movies"`
	Shows         int        `json:"shows"`
	Episodes      int        `json:"episodes"`
	LastWatchedAt *time.Time `json:"last_watched_at,omitempty"`
}

// WatchingDetail je historie jednoho uživatele za posledních Days dní.
type WatchingDetail struct {
	User    WatchingUser                `json:"user"`
	Days    int                         `json:"days"`
	Entries []storage.WatchHistoryEntry `json:"entries"`
}

// Overview vrátí přehled sledování všech uživatelů: nejdřív naposledy
// aktivní, účty bez historie na konci podle jména.
func (w *Watching) Overview(ctx context.Context) ([]WatchingSummary, error) {
	users, err := w.users.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("seznam uživatelů: %w", err)
	}
	sums, err := w.store.WatchSummaries(ctx)
	if err != nil {
		return nil, err
	}
	byUser := make(map[uuid.UUID]storage.WatchSummary, len(sums))
	for _, s := range sums {
		byUser[s.UserID] = s
	}

	out := make([]WatchingSummary, 0, len(users))
	for _, u := range users {
		row := WatchingSummary{UserID: u.ID, DisplayName: u.DisplayName, Email: u.Email, Role: u.Role}
		if s, ok := byUser[u.ID]; ok {
			row.Seconds, row.Movies, row.Shows, row.Episodes = s.Seconds, s.Movies, s.Shows, s.Episodes
			last := s.LastAt
			row.LastWatchedAt = &last
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].LastWatchedAt, out[j].LastWatchedAt
		switch {
		case a != nil && b != nil && !a.Equal(*b):
			return a.After(*b)
		case (a == nil) != (b == nil):
			return a != nil
		}
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out, nil
}

// UserDetail vrátí historii jednoho uživatele za posledních `days` dní.
// Mimo rozsah 1–WatchingMaxDays vrací ErrInvalid, neznámý účet ErrNotFound.
func (w *Watching) UserDetail(ctx context.Context, userID uuid.UUID, days int) (*WatchingDetail, error) {
	if days < 1 || days > WatchingMaxDays {
		return nil, fmt.Errorf("%w: počet dní musí být 1–%d", ErrInvalid, WatchingMaxDays)
	}
	u, err := w.users.GetByID(ctx, userID)
	if errors.Is(err, coreservice.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("uživatel: %w", err)
	}
	entries, err := w.store.WatchHistory(ctx, userID, days)
	if err != nil {
		return nil, err
	}
	return &WatchingDetail{User: watchingUser(*u), Days: days, Entries: entries}, nil
}
