package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"mediater/internal/core/httpx"
	"mediater/internal/libriter/service"
	"mediater/internal/libriter/storage"
)

// Parametry stránkovaných výpisů (?limit= zapíná stránkování, viz
// httpx.ParsePage):
//
//	q    – hledaný text (bez ohledu na diakritiku)
//	sort – klíč řazení, dir=desc obrací směr
//	ids  – jen záznamy s těmito ID, oddělené čárkou (nejvýš httpx.MaxPageLimit)

// listIDs přečte ?ids=; nil = parametr chybí (nefiltruje), prázdný řez =
// parametr je prázdný (nic neodpovídá).
func listIDs(r *http.Request) ([]uuid.UUID, error) {
	q := r.URL.Query()
	if !q.Has("ids") {
		return nil, nil
	}
	ids := []uuid.UUID{}
	for _, part := range strings.Split(q.Get("ids"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := uuid.Parse(part)
		if err != nil {
			return nil, fmt.Errorf("neplatné ID v ids: %q", part)
		}
		ids = append(ids, id)
	}
	if len(ids) > httpx.MaxPageLimit {
		return nil, fmt.Errorf("ids: nejvýš %d položek", httpx.MaxPageLimit)
	}
	return ids, nil
}

// optionalUUID přečte nepovinné ID z parametru dotazu (uuid.Nil = chybí).
func optionalUUID(r *http.Request, name string) (uuid.UUID, error) {
	v := strings.TrimSpace(r.URL.Query().Get(name))
	if v == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil, fmt.Errorf("neplatné %s", name)
	}
	return id, nil
}

func listDesc(r *http.Request) bool {
	return strings.EqualFold(r.URL.Query().Get("dir"), "desc")
}

// bookFilter: q, sort (title|author|added|published), dir, author_id,
// series_id, language, ids.
func bookFilter(r *http.Request) (storage.BookFilter, error) {
	q := r.URL.Query()
	f := storage.BookFilter{
		Query:    q.Get("q"),
		Sort:     q.Get("sort"),
		Desc:     listDesc(r),
		Language: normalizeLanguage(q.Get("language")),
	}
	var err error
	if f.AuthorID, err = optionalUUID(r, "author_id"); err != nil {
		return f, err
	}
	if f.SeriesID, err = optionalUUID(r, "series_id"); err != nil {
		return f, err
	}
	f.IDs, err = listIDs(r)
	return f, err
}

// authorFilter: q, sort (last_name|first_name|books), dir, ids, all=1
// (i autoři bez knih – výběr autora při úpravě knihy).
func authorFilter(r *http.Request) (storage.AuthorFilter, error) {
	q := r.URL.Query()
	f := storage.AuthorFilter{
		Query:        q.Get("q"),
		Sort:         q.Get("sort"),
		Desc:         listDesc(r),
		WithoutBooks: q.Get("all") == "1",
	}
	var err error
	f.IDs, err = listIDs(r)
	return f, err
}

// seriesFilter: q, ids, all=1 (i prázdné série – výběr série při úpravě knihy).
func seriesFilter(r *http.Request) (storage.SeriesFilter, error) {
	q := r.URL.Query()
	f := storage.SeriesFilter{
		Query:        q.Get("q"),
		WithoutBooks: q.Get("all") == "1",
	}
	var err error
	f.IDs, err = listIDs(r)
	return f, err
}

// Velikost dávky GET /sync/changes.
const (
	defaultSyncLimit = 500
	maxSyncLimit     = 2000
)

// SyncHandler vydává změny knihovny pro zařízení s lokální kopií (mobil).
type SyncHandler struct {
	svc *service.LibrarySyncService
}

func NewSync(svc *service.LibrarySyncService) *SyncHandler {
	return &SyncHandler{svc: svc}
}

// GET /api/v1/sync/changes?since=<rev>&limit=<n>
//
// Změny knih, autorů, sérií a stavu poslechu přihlášeného uživatele od rev
// since, seřazené podle rev. Bez since (nebo s full_resync v odpovědi)
// jde o celou knihovnu. Klient opakuje dotaz s since = rev z odpovědi,
// dokud je more true. Popis protokolu: docs/sync.md.
func (h *SyncHandler) Changes(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var since int64
	if v := q.Get("since"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "validation.invalid", "neplatné since")
			return
		}
		since = n
	}
	limit := defaultSyncLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "validation.invalid", "neplatný limit")
			return
		}
		limit = min(n, maxSyncLimit)
	}

	changes, err := h.svc.Changes(r.Context(), userID, since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání změn knihovny")
		return
	}
	writeJSON(w, http.StatusOK, changes)
}
