package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"libriter/internal/api/middleware"
	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

type PlaySessionHandler struct {
	svc *service.PlaySessionService
}

func NewPlaySession(svc *service.PlaySessionService) *PlaySessionHandler {
	return &PlaySessionHandler{svc: svc}
}

// GET /api/v1/sessions
//
// Přehrávač si odtud bere seznam rozposlouchaných session i s pozicemi;
// názvy knih a obálky si rozhraní dohledá ve své cache knihovny.
func (h *PlaySessionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}

	sessions, err := h.svc.List(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání poslechů")
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

// GET /api/v1/sessions/{id}
func (h *PlaySessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	session, err := h.svc.Get(r.Context(), userID, id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "poslech nenalezen")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "chyba při načítání poslechu")
	default:
		writeJSON(w, http.StatusOK, session)
	}
}

// POST /api/v1/sessions
//
// Založí poslech knihy, série nebo vlastního seznamu. U knihy a série vrací
// 200 s existující rozposlouchanou session, 201 jen u opravdu nové.
func (h *PlaySessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}

	var req struct {
		Kind      string   `json:"kind"`
		BookID    string   `json:"book_id"`
		SeriesID  string   `json:"series_id"`
		Title     string   `json:"title"`
		BookIDs   []string `json:"book_ids"`
		SeriesIDs []string `json:"series_ids"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	var (
		session *model.PlaySession
		created bool
		err     error
	)
	switch req.Kind {
	case model.PlaySessionBook:
		bookID, perr := parseUUIDStr(req.BookID, "book_id")
		if perr != nil {
			writeError(w, http.StatusBadRequest, perr.Error())
			return
		}
		session, created, err = h.svc.StartBook(r.Context(), userID, bookID)

	case model.PlaySessionSeries:
		seriesID, perr := parseUUIDStr(req.SeriesID, "series_id")
		if perr != nil {
			writeError(w, http.StatusBadRequest, perr.Error())
			return
		}
		session, created, err = h.svc.StartSeries(r.Context(), userID, seriesID)

	case model.PlaySessionList:
		bookIDs, seriesIDs, perr := parseSelection(req.BookIDs, req.SeriesIDs)
		if perr != nil {
			writeError(w, http.StatusBadRequest, perr.Error())
			return
		}
		session, err = h.svc.StartList(r.Context(), userID, strings.TrimSpace(req.Title), bookIDs, seriesIDs)
		created = true

	default:
		writeError(w, http.StatusBadRequest, "kind musí být book, series nebo list")
		return
	}

	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "kniha nebo série nenalezena")
	case errors.Is(err, service.ErrEmptySession):
		writeError(w, http.StatusBadRequest, "poslech nemá žádné knihy k přehrání")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "poslech se nepodařilo založit")
	case created:
		writeJSON(w, http.StatusCreated, session)
	default:
		writeJSON(w, http.StatusOK, session)
	}
}

// PUT /api/v1/sessions/{id}/position
//
// Zapisuje se každých pár sekund poslechu a při každé změně, aby šlo
// pokračovat na jiném zařízení tam, kde poslech skončil.
func (h *PlaySessionHandler) SavePosition(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		BookID          string  `json:"book_id"`
		ChapterID       string  `json:"chapter_id"`
		PositionSeconds int     `json:"position_seconds"`
		PlaybackSpeed   float64 `json:"playback_speed"`
		Finished        bool    `json:"finished"`
		// Sekundy obsahu od minulého zápisu; jdou do deníku poslechu.
		ListenedSeconds int `json:"listened_seconds"`
		// Doposlechnutá poslední kapitola téhle knihy – posílá se před
		// přechodem na další knihu poslechu.
		BookFinished bool `json:"book_finished"`
		// Čas vzniku pozice na klientovi a zařízení, ze kterého přišla.
		// Webový přehrávač je neposílá a server dosadí své "teď"; mobil je
		// vyplňuje, aby offline dávka nepřebila novější pozici odjinud.
		RecordedAt *time.Time `json:"recorded_at"`
		DeviceID   string     `json:"device_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	bookID, err := parseUUIDStr(req.BookID, "book_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	in := storage.PlaySessionPosition{
		BookID:          bookID,
		PositionSeconds: req.PositionSeconds,
		PlaybackSpeed:   req.PlaybackSpeed,
		Finished:        req.Finished,
		ListenedSeconds: req.ListenedSeconds,
		BookFinished:    req.BookFinished,
		DeviceID:        req.DeviceID,
	}
	if req.RecordedAt != nil {
		in.RecordedAt = *req.RecordedAt
	}
	if req.ChapterID != "" {
		chapterID, err := parseUUIDStr(req.ChapterID, "chapter_id")
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		in.ChapterID = &chapterID
	}

	session, err := h.svc.SavePosition(r.Context(), userID, id, in)
	switch {
	// Zastaralý zápis není chyba: session se vrací taková, jaká na serveru
	// platí, a klient se podle ní srovná.
	case errors.Is(err, service.ErrStalePosition):
		writeJSON(w, http.StatusOK, session)
	case errors.Is(err, service.ErrInvalidSetting):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrBookNotInSession):
		writeError(w, http.StatusBadRequest, "kniha není součástí tohoto poslechu")
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "poslech nenalezen")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "pozici se nepodařilo uložit")
	default:
		writeJSON(w, http.StatusOK, session)
	}
}

// syncEventRequest je jedna položka dávky – totéž co tělo PUT
// /sessions/{id}/position, jen s vlastním ID a session přímo v události.
type syncEventRequest struct {
	ID              string    `json:"id"`
	SessionID       string    `json:"session_id"`
	BookID          string    `json:"book_id"`
	ChapterID       string    `json:"chapter_id"`
	PositionSeconds int       `json:"position_seconds"`
	PlaybackSpeed   float64   `json:"playback_speed"`
	ListenedSeconds int       `json:"listened_seconds"`
	Finished        bool      `json:"finished"`
	BookFinished    bool      `json:"book_finished"`
	RecordedAt      time.Time `json:"recorded_at"`
}

// POST /api/v1/sessions/sync
//
// Přijme dávku pozic, které vznikly, když klient neměl spojení. Každá událost
// má vlastní ID a čas vzniku: podle ID se pozná opakovaně poslaná dávka,
// podle času se rozhodne, jestli pozici přepsat, nebo nechat tu novější
// z jiného zařízení. Odpověď nese osud každé události a aktuální stav
// dotčených session.
func (h *PlaySessionHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}

	var req struct {
		DeviceID string             `json:"device_id"`
		Events   []syncEventRequest `json:"events"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	events, err := parseSyncEvents(req.Events)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	results, sessions, err := h.svc.Sync(r.Context(), userID, strings.TrimSpace(req.DeviceID), events)
	switch {
	case errors.Is(err, service.ErrInvalidSetting):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "dávku se nepodařilo zpracovat")
	default:
		writeJSON(w, http.StatusOK, map[string]any{
			"results":  results,
			"sessions": sessions,
		})
	}
}

// parseSyncEvents převede dávku na typy service vrstvy. Vadné UUID shodí celý
// požadavek: to není chyba jedné události, ale rozbitý klient.
func parseSyncEvents(raw []syncEventRequest) ([]service.SyncEvent, error) {
	events := make([]service.SyncEvent, 0, len(raw))
	for i, item := range raw {
		event := service.SyncEvent{
			PositionSeconds: item.PositionSeconds,
			PlaybackSpeed:   item.PlaybackSpeed,
			ListenedSeconds: item.ListenedSeconds,
			Finished:        item.Finished,
			BookFinished:    item.BookFinished,
			RecordedAt:      item.RecordedAt,
		}

		for _, field := range []struct {
			name  string
			value string
			into  *uuid.UUID
		}{
			{"id", item.ID, &event.ID},
			{"session_id", item.SessionID, &event.SessionID},
			{"book_id", item.BookID, &event.BookID},
		} {
			parsed, err := parseUUIDStr(field.value, field.name)
			if err != nil {
				return nil, fmt.Errorf("událost %d: %w", i, err)
			}
			*field.into = parsed
		}

		if item.ChapterID != "" {
			chapterID, err := parseUUIDStr(item.ChapterID, "chapter_id")
			if err != nil {
				return nil, fmt.Errorf("událost %d: %w", i, err)
			}
			event.ChapterID = &chapterID
		}
		events = append(events, event)
	}
	return events, nil
}

// POST /api/v1/sessions/{id}/items
func (h *PlaySessionHandler) AddItems(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		BookIDs   []string `json:"book_ids"`
		SeriesIDs []string `json:"series_ids"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	bookIDs, seriesIDs, err := parseSelection(req.BookIDs, req.SeriesIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	session, err := h.svc.AddItems(r.Context(), userID, id, bookIDs, seriesIDs)
	switch {
	case errors.Is(err, service.ErrEmptySession):
		writeError(w, http.StatusBadRequest, "nebyla vybrána žádná kniha")
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "poslech, kniha nebo série nenalezena")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "knihy se nepodařilo přidat")
	default:
		writeJSON(w, http.StatusOK, session)
	}
}

// DELETE /api/v1/sessions/{id}
func (h *PlaySessionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	err := h.svc.Delete(r.Context(), userID, id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "poslech nenalezen")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "poslech se nepodařilo smazat")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// callerID vytáhne přihlášeného uživatele z kontextu.
func callerID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "chybí autorizační token")
		return uuid.UUID{}, false
	}
	return userID, true
}

func parseSelection(bookIDs, seriesIDs []string) ([]uuid.UUID, []uuid.UUID, error) {
	books, err := parseUUIDs(bookIDs, "book_ids")
	if err != nil {
		return nil, nil, err
	}
	series, err := parseUUIDs(seriesIDs, "series_ids")
	if err != nil {
		return nil, nil, err
	}
	return books, series, nil
}
