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
	"mediater/internal/sceniter/scanner"
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
	// DetachableFiles jsou soubory, které se k filmu přisloučily podle
	// metadat a lze je oddělit (viz scanner.DetachFile).
	DetachableFiles []uuid.UUID     `json:"detachable_files"`
	User            model.UserState `json:"user"`
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
	detachable := []uuid.UUID{}
	if len(files) > 1 {
		for _, f := range files {
			if scanner.MovieKeyOf(f.FilePath) != m.SourceKey {
				detachable = append(detachable, f.ID)
			}
		}
	}
	return &MovieDetail{Movie: *m, Files: files, DetachableFiles: detachable, User: st}, nil
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

// SeasonInfo je řada v detailu seriálu. Díly nenese – u dlouhých seriálů
// jde o stovky dílů s popisy; načítají se po řadách (Library.Season).
type SeasonInfo struct {
	ID         uuid.UUID `json:"id"`
	Number     int       `json:"number"`
	Title      *string   `json:"title,omitempty"`
	PosterPath *string   `json:"poster_path,omitempty"`
	// EpisodeCount je počet dílů se souborem, WatchedCount z nich dokoukaných.
	EpisodeCount int `json:"episode_count"`
	WatchedCount int `json:"watched_count"`
}

// ShowDetail je seriál s řadami, stavem uživatele a dílem, který je na řadě.
type ShowDetail struct {
	model.Show
	Seasons []SeasonInfo    `json:"seasons"`
	User    model.UserState `json:"user"`
	// UpNext je díl, kterým má uživatel pokračovat (rozkoukaný, nebo první
	// nezhlédnutý za posledním zhlédnutým, nebo první díl).
	UpNext *EpisodeWithProgress `json:"up_next,omitempty"`
}

func (l *Library) Show(ctx context.Context, userID, id uuid.UUID) (*ShowDetail, error) {
	sh, err := l.store.GetShow(ctx, id)
	if err != nil {
		return nil, err
	}
	seasons, flat, latest, err := l.showEpisodes(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	out := &ShowDetail{Show: *sh, Seasons: []SeasonInfo{}}
	i := 0
	for _, se := range seasons {
		info := SeasonInfo{ID: se.ID, Number: se.Number, Title: se.Title, PosterPath: se.PosterPath, EpisodeCount: len(se.Episodes)}
		for range se.Episodes {
			if ep := flat[i]; ep.Progress != nil && ep.Progress.Completed {
				info.WatchedCount++
			}
			i++
		}
		out.Seasons = append(out.Seasons, info)
	}
	out.UpNext = upNext(flat, latest)

	st, err := l.store.UserState(ctx, userID, nil, &id)
	if err != nil {
		return nil, err
	}
	out.User = st
	return out, nil
}

// Season vrátí jednu řadu seriálu s díly a jejich rozkoukaností.
func (l *Library) Season(ctx context.Context, userID, showID uuid.UUID, number int) (*SeasonDetail, error) {
	seasons, err := l.store.ListSeasons(ctx, showID)
	if err != nil {
		return nil, err
	}
	for _, se := range seasons {
		if se.Number != number {
			continue
		}
		progress, err := l.store.ListProgress(ctx, userID, storage.ProgressFilter{ShowID: &showID})
		if err != nil {
			return nil, err
		}
		byEpisode := map[uuid.UUID]*model.WatchProgress{}
		for i := range progress {
			if p := &progress[i]; p.EpisodeID != nil {
				byEpisode[*p.EpisodeID] = p
			}
		}
		sd := &SeasonDetail{Season: se, Episodes: make([]EpisodeWithProgress, 0, len(se.Episodes))}
		sd.Season.Episodes = nil
		for _, e := range se.Episodes {
			sd.Episodes = append(sd.Episodes, EpisodeWithProgress{Episode: e, Progress: byEpisode[e.ID]})
		}
		return sd, nil
	}
	return nil, storage.ErrNotFound
}

// showEpisodes vrátí řady seriálu, všechny jeho díly v pořadí řad a čísel
// i s rozkoukaností uživatele, a naposledy změněný záznam rozkoukanosti.
func (l *Library) showEpisodes(ctx context.Context, userID, showID uuid.UUID) ([]model.Season, []EpisodeWithProgress, *model.WatchProgress, error) {
	seasons, err := l.store.ListSeasons(ctx, showID)
	if err != nil {
		return nil, nil, nil, err
	}
	progress, err := l.store.ListProgress(ctx, userID, storage.ProgressFilter{ShowID: &showID})
	if err != nil {
		return nil, nil, nil, err
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
	var flat []EpisodeWithProgress
	for _, se := range seasons {
		for _, e := range se.Episodes {
			flat = append(flat, EpisodeWithProgress{Episode: e, Progress: byEpisode[e.ID]})
		}
	}
	return seasons, flat, latest, nil
}

// upNext vybere díl, kterým uživatel pokračuje. Rozkoukaný díl (naposledy
// změněný, nedokoukaný) má přednost. Jinak je to první nezhlédnutý díl za
// posledním zhlédnutým v pořadí seriálu – nezáleží tedy na tom, v jakém
// pořadí uživatel díly zhlédl nebo označil, ani zda se k některému vrátil.
// Bez jakékoli rozkoukanosti je to první díl; dokoukaný seriál dává nil.
func upNext(episodes []EpisodeWithProgress, latest *model.WatchProgress) *EpisodeWithProgress {
	if len(episodes) == 0 {
		return nil
	}
	if latest != nil && !latest.Completed {
		for i := range episodes {
			if episodes[i].ID == *latest.EpisodeID {
				return &episodes[i]
			}
		}
	}
	lastWatched := -1
	for i := range episodes {
		if p := episodes[i].Progress; p != nil && p.Completed {
			lastWatched = i
		}
	}
	for i := lastWatched + 1; i < len(episodes); i++ {
		if p := episodes[i].Progress; p == nil || !p.Completed {
			return &episodes[i]
		}
	}
	return nil // seriál dokoukaný
}

// Home je obsah domovské stránky.
type Home struct {
	Continue []model.ContinueItem `json:"continue"`
	// Libraries jsou řádky „nově přidané“ – jeden za každou zapnutou knihovnu
	// v nastaveném pořadí; knihovna filmů plní Movies, seriálů Shows (seřazené
	// podle nejnovějšího přidaného dílu).
	Libraries []HomeLibrary   `json:"libraries"`
	Watchlist []WatchlistItem `json:"watchlist"`
}

// HomeLibrary je řádek domovské stránky za jednu knihovnu.
type HomeLibrary struct {
	Library model.Library `json:"library"`
	Movies  []model.Movie `json:"movies"`
	Shows   []model.Show  `json:"shows"`
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
		row := HomeLibrary{Library: lib, Movies: []model.Movie{}, Shows: []model.Show{}}
		if lib.Kind == model.KindShow {
			row.Shows, err = l.store.ListShows(ctx, storage.ShowFilter{Library: lib.ID, Sort: "added", Limit: 20})
		} else {
			row.Movies, err = l.store.ListMovies(ctx, storage.MovieFilter{Library: lib.ID, Sort: "added", Limit: 20})
		}
		if err != nil {
			return nil, err
		}
		if len(row.Movies)+len(row.Shows) > 0 {
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
			// Stejné pravidlo jako v detailu seriálu, ať karta a tlačítko
			// „Přehrát“ ukazují tentýž díl.
			_, flat, latest, err := l.showEpisodes(ctx, userID, e.ShowID)
			if err != nil {
				return nil, err
			}
			next := upNext(flat, latest)
			if next == nil || next.MediaID == nil {
				continue // seriál dokoukaný, nebo další díl zatím nemá soubor
			}
			item = model.ContinueItem{Kind: model.KindEpisode, Episode: &next.Episode, Progress: next.Progress, Next: next.Progress == nil}
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
