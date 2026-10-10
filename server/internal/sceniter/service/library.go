// Package service skládá data videoknihovny pro API: detaily titulů se
// stavem uživatele, domovskou stránku a rozkoukanost.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
)

var (
	ErrNotFound = storage.ErrNotFound
	// ErrInvalid znamená neplatný vstup od klienta.
	ErrInvalid = errors.New("neplatný požadavek")
)

// Library je čtecí a uživatelská část videoknihovny.
type Library struct {
	store *storage.Store
}

func NewLibrary(store *storage.Store) *Library { return &Library{store: store} }

// MovieDetail je film se soubory a stavem uživatele.
type MovieDetail struct {
	model.Movie
	Files []model.MediaFile `json:"files"`
	User  model.UserState   `json:"user"`
}

func (l *Library) Movie(ctx context.Context, userID, id uuid.UUID) (*MovieDetail, error) {
	m, err := l.store.GetMovie(ctx, id)
	if err != nil {
		return nil, err
	}
	files, err := l.store.ListMedia(ctx, &m.ID, nil)
	if err != nil {
		return nil, err
	}
	st, err := l.store.UserState(ctx, userID, &m.ID, nil)
	if err != nil {
		return nil, err
	}
	return &MovieDetail{Movie: *m, Files: files, User: st}, nil
}

// EpisodeWithProgress je díl s rozkoukaností uživatele.
type EpisodeWithProgress struct {
	model.Episode
	Progress *model.WatchProgress `json:"progress,omitempty"`
}

// SeasonDetail je řada s díly a jejich rozkoukaností.
type SeasonDetail struct {
	model.Season
	Episodes []EpisodeWithProgress `json:"episodes"`
}

// ShowDetail je seriál s řadami, stavem uživatele a dílem, který je na řadě.
type ShowDetail struct {
	model.Show
	Seasons []SeasonDetail  `json:"seasons"`
	User    model.UserState `json:"user"`
	// UpNext je díl, kterým má uživatel pokračovat (rozkoukaný, nebo další
	// po naposledy dokoukaném, nebo první díl).
	UpNext *EpisodeWithProgress `json:"up_next,omitempty"`
}

func (l *Library) Show(ctx context.Context, userID, id uuid.UUID) (*ShowDetail, error) {
	sh, err := l.store.GetShow(ctx, id)
	if err != nil {
		return nil, err
	}
	seasons, err := l.store.ListSeasons(ctx, id)
	if err != nil {
		return nil, err
	}
	progress, err := l.store.ListProgress(ctx, userID, storage.ProgressFilter{ShowID: &id})
	if err != nil {
		return nil, err
	}
	byEpisode := map[uuid.UUID]*model.WatchProgress{}
	var latest *model.WatchProgress
	for i := range progress {
		p := &progress[i]
		if p.EpisodeID != nil {
			byEpisode[*p.EpisodeID] = p
			if latest == nil { // seřazeno od naposledy změněné
				latest = p
			}
		}
	}

	out := &ShowDetail{Show: *sh, Seasons: []SeasonDetail{}}
	var flat []EpisodeWithProgress
	for _, se := range seasons {
		sd := SeasonDetail{Season: se, Episodes: []EpisodeWithProgress{}}
		sd.Season.Episodes = nil
		for _, e := range se.Episodes {
			ep := EpisodeWithProgress{Episode: e, Progress: byEpisode[e.ID]}
			sd.Episodes = append(sd.Episodes, ep)
			flat = append(flat, ep)
		}
		out.Seasons = append(out.Seasons, sd)
	}
	out.UpNext = upNext(flat, latest)

	st, err := l.store.UserState(ctx, userID, nil, &id)
	if err != nil {
		return nil, err
	}
	out.User = st
	return out, nil
}

// upNext vybere díl, kterým uživatel pokračuje: naposledy sledovaný, pokud
// není dokoukaný; jinak další po něm; bez historie první díl.
func upNext(episodes []EpisodeWithProgress, latest *model.WatchProgress) *EpisodeWithProgress {
	if len(episodes) == 0 {
		return nil
	}
	if latest == nil {
		return &episodes[0]
	}
	for i := range episodes {
		if episodes[i].ID != *latest.EpisodeID {
			continue
		}
		if !latest.Completed {
			return &episodes[i]
		}
		if i+1 < len(episodes) {
			return &episodes[i+1]
		}
		return nil // seriál dokoukaný
	}
	return &episodes[0]
}

// Home je obsah domovské stránky.
type Home struct {
	Continue []model.ContinueItem `json:"continue"`
	// Libraries jsou řádky „nově přidané“ – jeden za každou zapnutou knihovnu
	// v nastaveném pořadí; knihovna filmů plní Movies, seriálů Episodes.
	Libraries []HomeLibrary   `json:"libraries"`
	Watchlist []WatchlistItem `json:"watchlist"`
}

// HomeLibrary je řádek domovské stránky za jednu knihovnu.
type HomeLibrary struct {
	Library  model.Library   `json:"library"`
	Movies   []model.Movie   `json:"movies"`
	Episodes []model.Episode `json:"episodes"`
}

func (l *Library) Home(ctx context.Context, userID uuid.UUID) (*Home, error) {
	cont, err := l.Continue(ctx, userID, 20)
	if err != nil {
		return nil, err
	}
	libs, err := l.store.ListLibraries(ctx, true)
	if err != nil {
		return nil, err
	}
	rows := make([]HomeLibrary, 0, len(libs))
	for _, lib := range libs {
		row := HomeLibrary{Library: lib, Movies: []model.Movie{}, Episodes: []model.Episode{}}
		if lib.Kind == model.KindShow {
			row.Episodes, err = l.store.RecentEpisodes(ctx, lib.ID, 20)
		} else {
			row.Movies, err = l.store.ListMovies(ctx, storage.MovieFilter{Library: lib.ID, Sort: "added", Limit: 20})
		}
		if err != nil {
			return nil, err
		}
		if len(row.Movies)+len(row.Episodes) > 0 {
			rows = append(rows, row)
		}
	}
	wl, err := l.Watchlist(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Home{Continue: cont, Libraries: rows, Watchlist: wl}, nil
}

// Continue vrátí „Pokračovat ve sledování“: rozkoukané filmy a díly a u
// seriálů, kde je poslední sledovaný díl dokoukaný, díl následující.
// Každý seriál je v seznamu nejvýš jednou.
func (l *Library) Continue(ctx context.Context, userID uuid.UUID, limit int) ([]model.ContinueItem, error) {
	all, err := l.store.ListProgress(ctx, userID, storage.ProgressFilter{Limit: 200})
	if err != nil {
		return nil, err
	}
	out := []model.ContinueItem{}
	seenShow := map[uuid.UUID]bool{}
	for i := range all {
		if len(out) >= limit {
			break
		}
		p := all[i]
		if p.MovieID != nil {
			if p.Completed || p.PositionSec <= 0 {
				continue
			}
			m, err := l.store.GetMovie(ctx, *p.MovieID)
			if errors.Is(err, storage.ErrNotFound) || (err == nil && m.MediaID == nil) {
				continue
			}
			if err != nil {
				return nil, err
			}
			out = append(out, model.ContinueItem{Kind: model.KindMovie, Movie: m, Progress: &p})
			continue
		}
		e, err := l.store.GetEpisode(ctx, *p.EpisodeID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if seenShow[e.ShowID] {
			continue
		}
		seenShow[e.ShowID] = true
		item := model.ContinueItem{Kind: model.KindEpisode, Episode: e, Progress: &p}
		if p.Completed {
			next, err := l.store.NextEpisode(ctx, e.ID)
			if errors.Is(err, storage.ErrNotFound) {
				continue // seriál dokoukaný
			}
			if err != nil {
				return nil, err
			}
			np, err := l.store.GetProgress(ctx, userID, nil, &next.ID)
			if err != nil && !errors.Is(err, storage.ErrNotFound) {
				return nil, err
			}
			if np != nil && np.Completed {
				continue
			}
			item = model.ContinueItem{Kind: model.KindEpisode, Episode: next, Progress: np, Next: np == nil}
		} else if e.MediaID == nil {
			continue
		}
		show, err := l.store.GetShow(ctx, e.ShowID)
		if err != nil {
			return nil, err
		}
		item.Show = show
		out = append(out, item)
	}
	return out, nil
}

// WatchlistItem je film nebo seriál ze seznamu „chci vidět“.
type WatchlistItem struct {
	Kind    string       `json:"kind"`
	Movie   *model.Movie `json:"movie,omitempty"`
	Show    *model.Show  `json:"show,omitempty"`
	AddedAt time.Time    `json:"added_at"`
}

func (l *Library) Watchlist(ctx context.Context, userID uuid.UUID) ([]WatchlistItem, error) {
	entries, err := l.store.ListWatchlist(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := []WatchlistItem{}
	for _, e := range entries {
		item := WatchlistItem{AddedAt: e.AddedAt}
		if e.MovieID != nil {
			m, err := l.store.GetMovie(ctx, *e.MovieID)
			if err != nil {
				continue
			}
			item.Kind, item.Movie = model.KindMovie, m
		} else {
			s, err := l.store.GetShow(ctx, *e.ShowID)
			if err != nil {
				continue
			}
			item.Kind, item.Show = model.KindShow, s
		}
		out = append(out, item)
	}
	return out, nil
}

// Hranice dokoukání: 90 % délky nebo méně než 3 minuty do konce (titulky).
const (
	completedRatio   = 0.90
	completedTailSec = 180
	// maxFutureSkew: razítko z budoucnosti (špatné hodiny klienta) se ořízne.
	maxFutureSkew = 5 * time.Minute
)

// ProgressRequest je zápis pozice z přehrávače.
type ProgressRequest struct {
	MovieID        *uuid.UUID `json:"movie_id"`
	EpisodeID      *uuid.UUID `json:"episode_id"`
	MediaID        *uuid.UUID `json:"media_id"`
	PositionSec    float64    `json:"position_sec"`
	DurationSec    float64    `json:"duration_sec"`
	WatchedSeconds int        `json:"watched_seconds"`
	Completed      *bool      `json:"completed"`
	DeviceID       string     `json:"device_id"`
	RecordedAt     *time.Time `json:"recorded_at"`
	EventID        *uuid.UUID `json:"event_id"`
}

// SaveProgress ověří a zapíše pozici. Bez recorded_at platí čas serveru.
// Vrací storage.ErrStale, když na serveru je novější pozice.
func (l *Library) SaveProgress(ctx context.Context, userID uuid.UUID, req ProgressRequest) error {
	if (req.MovieID == nil) == (req.EpisodeID == nil) {
		return fmt.Errorf("%w: zadejte právě jedno z movie_id a episode_id", ErrInvalid)
	}
	if req.PositionSec < 0 || req.DurationSec < 0 {
		return fmt.Errorf("%w: záporná pozice", ErrInvalid)
	}
	now := time.Now()
	at := now
	if req.RecordedAt != nil {
		at = *req.RecordedAt
		if at.After(now.Add(maxFutureSkew)) {
			at = now
		}
	}
	completed := false
	if req.Completed != nil {
		completed = *req.Completed
	} else if req.DurationSec > 0 {
		completed = req.PositionSec >= req.DurationSec*completedRatio ||
			req.DurationSec-req.PositionSec < completedTailSec
	}
	return l.store.SaveProgress(ctx, userID, storage.ProgressInput{
		MovieID: req.MovieID, EpisodeID: req.EpisodeID, MediaID: req.MediaID,
		PositionSec: req.PositionSec, DurationSec: req.DurationSec, Completed: completed,
		DeviceID: req.DeviceID, RecordedAt: at, WatchedSeconds: req.WatchedSeconds, EventID: req.EventID,
	})
}

// MarkWatched označí film nebo díl za dokoukaný (watched=true), nebo
// smaže jeho rozkoukanost.
func (l *Library) MarkWatched(ctx context.Context, userID uuid.UUID, kind string, id uuid.UUID, watched bool) error {
	var movieID, episodeID *uuid.UUID
	switch kind {
	case model.KindMovie:
		if _, err := l.store.GetMovie(ctx, id); err != nil {
			return err
		}
		movieID = &id
	case model.KindEpisode:
		if _, err := l.store.GetEpisode(ctx, id); err != nil {
			return err
		}
		episodeID = &id
	default:
		return fmt.Errorf("%w: neznámý druh %q", ErrInvalid, kind)
	}
	if !watched {
		return l.store.DeleteProgress(ctx, userID, movieID, episodeID)
	}
	err := l.store.SaveProgress(ctx, userID, storage.ProgressInput{
		MovieID: movieID, EpisodeID: episodeID, Completed: true, RecordedAt: time.Now(),
	})
	if errors.Is(err, storage.ErrStale) {
		return nil
	}
	return err
}
