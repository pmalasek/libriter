package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// MaxSyncEvents je strop na jednu dávku. Při zápisu po deseti sekundách
// poslechu je to necelých devadesát minut offline poslechu; delší pauza se
// pošle na víc dávek a do 1 MB limitu těla požadavku se pohodlně vejde.
const MaxSyncEvents = 500

// maxClockSkew je tolerance na rozcházející se hodiny zařízení. Razítko dál
// v budoucnu by knize zablokovalo všechny další zápisy jako "starší", proto se
// ořízne na serverové teď. Minulost se přijímá, jaká je – tam je offline
// poslech legitimní.
const maxClockSkew = 5 * time.Minute

// Stavy jedné události v odpovědi synchronizace.
const (
	// SyncApplied – pozice i deník se zapsaly.
	SyncApplied = "applied"
	// SyncStale – poslech se připsal, pozici ale drží novější zápis odjinud.
	SyncStale = "stale"
	// SyncDuplicate – tuhle událost server už jednou započetl.
	SyncDuplicate = "duplicate"
	// SyncRejected – událost neprošla kontrolou a nikdy neprojde; klient ji
	// má zahodit, ne posílat dokola.
	SyncRejected = "rejected"
)

// SyncEvent je jedno uložení pozice, které vzniklo na klientovi. Odpovídá
// jednomu volání PUT /sessions/{id}/position, jen s vlastním ID a razítkem.
type SyncEvent struct {
	ID              uuid.UUID
	SessionID       uuid.UUID
	BookID          uuid.UUID
	ChapterID       *uuid.UUID
	PositionSeconds int
	PlaybackSpeed   float64
	ListenedSeconds int
	Finished        bool
	BookFinished    bool
	RecordedAt      time.Time
}

// SyncResult je osud jedné události. Klient podle něj smaže svou frontu –
// smaže všechny vrácené id bez ohledu na stav, protože žádný z nich se
// opakováním nezlepší.
type SyncResult struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
	Error  string    `json:"error,omitempty"`
}

// Sync zpracuje dávku offline událostí z jednoho zařízení a vrátí osud každé
// z nich spolu s aktuálním stavem dotčených session.
//
// Události se zpracují v pořadí, ve kterém vznikly, každá ve vlastní
// transakci: jedna vadná tak nezhatí zbytek dávky. Opakované odeslání téže
// dávky (ztracená odpověď, restart aplikace) skončí na duplicate a deník
// poslechu se nenafoukne.
func (p *PlaySessionService) Sync(
	ctx context.Context,
	userID uuid.UUID,
	deviceID string,
	events []SyncEvent,
) ([]SyncResult, []model.PlaySession, error) {
	if len(events) > MaxSyncEvents {
		return nil, nil, fmt.Errorf("%w: dávka má %d událostí, nejvýš %d",
			ErrInvalidSetting, len(events), MaxSyncEvents)
	}

	if pruned, err := p.store.PruneSyncEvents(ctx); err != nil {
		// Úklid staré historie není důvod zahodit poslech; jen se zaznamená.
		slog.Warn("úklid sync_events selhal", "err", err)
	} else if pruned > 0 {
		slog.Debug("sync_events uklizeny", "smazáno", pruned)
	}

	ordered := make([]SyncEvent, len(events))
	copy(ordered, events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].RecordedAt.Before(ordered[j].RecordedAt)
	})

	results := make([]SyncResult, 0, len(ordered))
	touched := make([]uuid.UUID, 0, 4)
	seen := make(map[uuid.UUID]bool, 4)

	for _, event := range ordered {
		result := p.applyEvent(ctx, userID, deviceID, event)
		results = append(results, result)

		// Stav session se klientovi hodí i u duplicitní nebo staré události –
		// právě ta mu říká, že jinde platí něco jiného.
		if result.Status != SyncRejected && !seen[event.SessionID] {
			seen[event.SessionID] = true
			touched = append(touched, event.SessionID)
		}
	}

	sessions := make([]model.PlaySession, 0, len(touched))
	for _, id := range touched {
		session, err := p.store.GetPlaySession(ctx, userID, id)
		if errors.Is(err, storage.ErrNotFound) {
			continue // session mezitím zmizela; klient ji podle GET /sessions uklidí
		}
		if err != nil {
			return nil, nil, err
		}
		sessions = append(sessions, *session)
	}

	return results, sessions, nil
}

// applyEvent zapíše jednu událost a přeloží výsledek na stav pro klienta.
func (p *PlaySessionService) applyEvent(
	ctx context.Context,
	userID uuid.UUID,
	deviceID string,
	event SyncEvent,
) SyncResult {
	if event.ID == uuid.Nil {
		return SyncResult{ID: event.ID, Status: SyncRejected, Error: "chybí id události"}
	}

	in, err := p.syncPosition(ctx, event)
	if err != nil {
		return SyncResult{ID: event.ID, Status: SyncRejected, Error: err.Error()}
	}
	in.SyncEventID = &event.ID
	in.DeviceID = deviceID

	_, err = p.store.UpdatePlaySessionPosition(ctx, userID, event.SessionID, in)
	switch {
	case err == nil:
		return SyncResult{ID: event.ID, Status: SyncApplied}
	case errors.Is(err, storage.ErrDuplicateEvent):
		return SyncResult{ID: event.ID, Status: SyncDuplicate}
	case errors.Is(err, storage.ErrStalePosition):
		return SyncResult{ID: event.ID, Status: SyncStale}
	case errors.Is(err, storage.ErrNotFound):
		return SyncResult{ID: event.ID, Status: SyncRejected, Error: "poslech nenalezen"}
	case errors.Is(err, storage.ErrBookNotInSession):
		return SyncResult{ID: event.ID, Status: SyncRejected, Error: "kniha není součástí poslechu"}
	default:
		// Výpadek databáze není chyba klienta, ale odpověď musí mít stav.
		// Jde o rejected: klient událost zahodí, poslech přijde nejvýš o
		// deset sekund a další zápis je zase přesný.
		slog.Error("sync: zápis pozice", "user_id", userID, "event", event.ID, "err", err)
		return SyncResult{ID: event.ID, Status: SyncRejected, Error: "pozici se nepodařilo uložit"}
	}
}

// syncPosition převede událost na zápis pozice a ověří ji stejně jako
// SavePosition – kdyby se pravidla rozešla, mobil by uměl uložit něco, co web
// nesmí.
func (p *PlaySessionService) syncPosition(ctx context.Context, event SyncEvent) (storage.PlaySessionPosition, error) {
	if event.PositionSeconds < 0 {
		return storage.PlaySessionPosition{}, errors.New("záporná pozice")
	}
	if event.PlaybackSpeed < minPlaybackSpeed || event.PlaybackSpeed > maxPlaybackSpeed {
		return storage.PlaySessionPosition{}, errors.New("rychlost mimo rozsah")
	}
	if event.ChapterID != nil {
		chapter, err := p.store.GetChapterByID(ctx, *event.ChapterID)
		if errors.Is(err, storage.ErrNotFound) {
			return storage.PlaySessionPosition{}, errors.New("kapitola nenalezena")
		}
		if err != nil {
			return storage.PlaySessionPosition{}, err
		}
		if chapter.BookID != event.BookID {
			return storage.PlaySessionPosition{}, errors.New("kapitola nepatří ke knize")
		}
	}

	listened := event.ListenedSeconds
	if listened < 0 {
		listened = 0
	}
	if listened > maxListenedSecondsPerSave {
		listened = maxListenedSecondsPerSave
	}

	return storage.PlaySessionPosition{
		BookID:          event.BookID,
		ChapterID:       event.ChapterID,
		PositionSeconds: event.PositionSeconds,
		PlaybackSpeed:   event.PlaybackSpeed,
		Finished:        event.Finished,
		ListenedSeconds: listened,
		BookFinished:    event.BookFinished,
		RecordedAt:      clampRecordedAt(event.RecordedAt),
	}, nil
}

// clampRecordedAt ořízne razítko z budoucnosti. Telefon se špatně nastavenými
// hodinami by jinak jedinou událostí zablokoval všechny další zápisy a zapsal
// poslech do dne, který ještě nenastal.
func clampRecordedAt(at time.Time) time.Time {
	now := time.Now()
	if at.IsZero() || at.After(now.Add(maxClockSkew)) {
		return now
	}
	return at
}
