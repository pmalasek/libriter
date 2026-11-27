package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	coreservice "mediater/internal/core/service"
	corestorage "mediater/internal/core/storage"
	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
)

func (e *env) watching() *Watching {
	return NewWatching(e.store, coreservice.NewUser(corestorage.New(e.conn), nil))
}

func (e *env) historyRows(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	var n int
	if err := e.conn.QueryRow(`SELECT COUNT(*) FROM sc_watch_history WHERE user_id = ?1`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSaveProgressWritesWatchHistory(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	movieID, mediaID := e.movie(t, "Pelíšky")
	episodeID := e.episode(t, "Docent", 1, 2)

	now := time.Now().UTC().Truncate(time.Second)
	// Těsně po půlnoci by „před dvěma minutami“ spadlo do včerejška; čas
	// do 5 minut v budoucnosti služba přijme.
	if midnight := now.Truncate(24 * time.Hour); now.Before(midnight.Add(3 * time.Minute)) {
		now = midnight.Add(3 * time.Minute)
	}
	today := now.Format("2006-01-02")
	earlier, later := now.Add(-2*time.Minute), now.Add(-time.Minute)
	yesterday := now.Add(-24 * time.Hour)
	save := func(req ProgressRequest, at time.Time) {
		t.Helper()
		req.RecordedAt = &at
		if err := e.lib.SaveProgress(ctx, e.userID, req); err != nil && !errors.Is(err, storage.ErrStale) {
			t.Fatal(err)
		}
	}
	movie := ProgressRequest{MovieID: &movieID, MediaID: &mediaID, PositionSec: 100, DurationSec: 6000}

	movie.WatchedSeconds = 60
	save(movie, later)
	// Starší událost (jiné zařízení) pozici nepřepíše, do historie ale patří.
	movie.WatchedSeconds = 30
	save(movie, earlier)
	movie.WatchedSeconds = 45
	save(movie, yesterday)
	// Bez odkoukaných sekund (otevření titulu, ruční „zhlédnuto“) řádek nevzniká.
	movie.WatchedSeconds = 0
	save(movie, now)
	save(ProgressRequest{EpisodeID: &episodeID, PositionSec: 5}, now)
	if err := e.lib.MarkWatched(ctx, e.userID, model.KindEpisode, episodeID, true); err != nil {
		t.Fatal(err)
	}
	if n := e.historyRows(t, e.userID); n != 2 {
		t.Fatalf("řádků historie %d, chci 2 (film dnes a včera)", n)
	}
	// Strop na jeden zápis platí i pro historii.
	save(ProgressRequest{EpisodeID: &episodeID, PositionSec: 900, WatchedSeconds: 5000}, now)

	detail, err := e.watching().UserDetail(ctx, e.userID, WatchingDefaultDays)
	if err != nil {
		t.Fatal(err)
	}
	if detail.User.ID != e.userID || detail.User.DisplayName != "Divák" || detail.Days != WatchingDefaultDays {
		t.Fatalf("uživatel %+v, dní %d", detail.User, detail.Days)
	}
	if len(detail.Entries) != 3 {
		t.Fatalf("záznamy %+v", detail.Entries)
	}
	// Dnešek první, v něm naposledy sledovaný titul (díl) první.
	ep, mv, old := detail.Entries[0], detail.Entries[1], detail.Entries[2]
	if ep.Day != today || ep.Kind != model.KindEpisode || ep.EpisodeID == nil || *ep.EpisodeID != episodeID ||
		ep.ShowTitle == nil || *ep.ShowTitle != "Docent" || ep.Season == nil || *ep.Season != 1 ||
		ep.Episode == nil || *ep.Episode != 2 || ep.ShowID == nil || ep.MovieID != nil ||
		ep.Seconds != 600 {
		t.Errorf("díl: %+v", ep)
	}
	if mv.Day != today || mv.Kind != model.KindMovie || mv.MovieID == nil || *mv.MovieID != movieID ||
		mv.MovieTitle == nil || *mv.MovieTitle != "Pelíšky" || mv.EpisodeID != nil || mv.Seconds != 90 {
		t.Errorf("film: %+v", mv)
	}
	if !mv.FirstAt.Equal(earlier) || !mv.LastAt.Equal(later) {
		t.Errorf("film first/last %v %v, chci %v %v", mv.FirstAt, mv.LastAt, earlier, later)
	}
	if old.Day != yesterday.Format("2006-01-02") || old.Seconds != 45 {
		t.Errorf("včerejšek: %+v", old)
	}

	// Okno jednoho dne obsahuje jen dnešek.
	detail, err = e.watching().UserDetail(ctx, e.userID, 1)
	if err != nil || len(detail.Entries) != 2 {
		t.Fatalf("jeden den: %+v, %v", detail, err)
	}
}

func TestWatchHistorySurvivesProgressReset(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	movieID, _ := e.movie(t, "Amadeus")
	at := time.Now()
	if err := e.lib.SaveProgress(ctx, e.userID, ProgressRequest{MovieID: &movieID, PositionSec: 60, WatchedSeconds: 60, RecordedAt: &at}); err != nil {
		t.Fatal(err)
	}
	// „Označit jako neviděné“ smaže rozkoukanost, ne historii.
	if err := e.lib.MarkWatched(ctx, e.userID, model.KindMovie, movieID, false); err != nil {
		t.Fatal(err)
	}
	if p, err := e.store.GetProgress(ctx, e.userID, &movieID, nil); err == nil {
		t.Fatalf("rozkoukanost zůstala: %+v", p)
	}
	if n := e.historyRows(t, e.userID); n != 1 {
		t.Fatalf("řádků historie %d, chci 1", n)
	}
}

func TestWatchHistoryDeletedWithUser(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	movieID, _ := e.movie(t, "Amadeus")
	at := time.Now()
	if err := e.lib.SaveProgress(ctx, e.userID, ProgressRequest{MovieID: &movieID, PositionSec: 60, WatchedSeconds: 60, RecordedAt: &at}); err != nil {
		t.Fatal(err)
	}
	// Smazání posledního admina server odmítá; čtenáře smazat jde.
	users := coreservice.NewUser(corestorage.New(e.conn), nil)
	if _, err := users.Create(ctx, "Správce", "admin@example.com", "", "tajneheslo", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := users.Delete(ctx, e.userID); err != nil {
		t.Fatal(err)
	}
	if n := e.historyRows(t, e.userID); n != 0 {
		t.Fatalf("po smazání účtu zůstalo %d řádků historie", n)
	}
	if _, err := e.watching().UserDetail(ctx, e.userID, 30); !errors.Is(err, ErrNotFound) {
		t.Fatalf("smazaný účet: %v, chci ErrNotFound", err)
	}
}

func TestWatchingOverview(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	users := coreservice.NewUser(corestorage.New(e.conn), nil)
	idle, err := users.Create(ctx, "Aleš", "ales@example.com", "", "tajneheslo", "reader")
	if err != nil {
		t.Fatal(err)
	}
	movieID, _ := e.movie(t, "Amadeus")
	ep1, ep2 := e.episode(t, "Docent", 1, 1), e.episode(t, "Docent", 1, 2)
	at := time.Now().Add(-time.Hour)
	for _, req := range []ProgressRequest{
		{MovieID: &movieID, WatchedSeconds: 100},
		{EpisodeID: &ep1, WatchedSeconds: 200},
		{EpisodeID: &ep2, WatchedSeconds: 300},
	} {
		req.RecordedAt = &at
		if err := e.lib.SaveProgress(ctx, e.userID, req); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := e.watching().Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("řádky %+v", rows)
	}
	// Aktivní účet první, i když by abecedně byl druhý.
	got := rows[0]
	if got.UserID != e.userID || got.Seconds != 600 || got.Movies != 1 || got.Shows != 1 || got.Episodes != 2 ||
		got.LastWatchedAt == nil || !got.LastWatchedAt.Equal(at.UTC().Truncate(time.Second)) {
		t.Errorf("aktivní účet: %+v", got)
	}
	if rows[1].UserID != idle.ID || rows[1].LastWatchedAt != nil || rows[1].Seconds != 0 {
		t.Errorf("účet bez historie: %+v", rows[1])
	}
}

func TestWatchingUserDetailValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	w := e.watching()
	if _, err := w.UserDetail(ctx, uuid.New(), 30); !errors.Is(err, ErrNotFound) {
		t.Errorf("neznámý účet: %v", err)
	}
	for _, days := range []int{0, WatchingMaxDays + 1} {
		if _, err := w.UserDetail(ctx, e.userID, days); !errors.Is(err, ErrInvalid) {
			t.Errorf("days=%d: %v", days, err)
		}
	}
	detail, err := w.UserDetail(ctx, e.userID, WatchingMaxDays)
	if err != nil || detail.Entries == nil || len(detail.Entries) != 0 {
		t.Errorf("prázdná historie: %+v, %v", detail, err)
	}
}
