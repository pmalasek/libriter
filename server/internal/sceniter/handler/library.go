package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"mediater/internal/core/imagestore"
	"mediater/internal/core/plugin"
	coreservice "mediater/internal/core/service"
	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/scanner"
	"mediater/internal/sceniter/service"
	"mediater/internal/sceniter/storage"
)

// GET /sceniter/images/{name}
func (h *Handler) Image(w http.ResponseWriter, r *http.Request) {
	path, ok := imagestore.Resolve(h.ImageRoot, chi.URLParam(r, "name"))
	if !ok {
		writeError(w, http.StatusNotFound, "image.not_found", "obrázek nenalezen")
		return
	}
	// Název obrázku se při změně nemění, klient přidává ?v=updated_at.
	w.Header().Set("Cache-Control", "public, max-age=604800")
	http.ServeFile(w, r, path)
}

// GET /sceniter/home
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	home, err := h.Library.Home(r.Context(), userID(r))
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, home)
}

// libraryQuery přečte ?library=<uuid>; prázdné = všechny knihovny.
func libraryQuery(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	v := r.URL.Query().Get("library")
	if v == "" {
		return uuid.Nil, true
	}
	id, err := uuid.Parse(v)
	if err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_id", "neplatné ID knihovny")
		return uuid.Nil, false
	}
	return id, true
}

// GET /sceniter/movies?library=&q=&genre=&sort=title|year|added|rating
func (h *Handler) Movies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lib, ok := libraryQuery(w, r)
	if !ok {
		return
	}
	movies, err := h.Store.ListMovies(r.Context(), storage.MovieFilter{Library: lib, Query: q.Get("q"), Genre: q.Get("genre"), Sort: q.Get("sort")})
	if writeStoreError(w, err, "movie.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, movies)
}

// GET /sceniter/movies/{id}
func (h *Handler) Movie(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	m, err := h.Library.Movie(r.Context(), userID(r), id)
	if writeStoreError(w, err, "movie.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// GET /sceniter/shows?library=&q=&genre=&sort=
func (h *Handler) Shows(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lib, ok := libraryQuery(w, r)
	if !ok {
		return
	}
	shows, err := h.Store.ListShows(r.Context(), storage.ShowFilter{Library: lib, Query: q.Get("q"), Genre: q.Get("genre"), Sort: q.Get("sort")})
	if writeStoreError(w, err, "show.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, shows)
}

// GET /sceniter/shows/{id}
func (h *Handler) Show(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	s, err := h.Library.Show(r.Context(), userID(r), id)
	if writeStoreError(w, err, "show.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// GET /sceniter/episodes/{id} – díl se soubory, rozkoukaností a dalším dílem.
func (h *Handler) Episode(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	e, err := h.Store.GetEpisode(ctx, id)
	if writeStoreError(w, err, "episode.not_found") {
		return
	}
	files, err := h.Store.ListMedia(ctx, nil, &id)
	if writeStoreError(w, err, "episode.not_found") {
		return
	}
	progress, err := h.Store.GetProgress(ctx, userID(r), nil, &id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		writeStoreError(w, err, "episode.not_found")
		return
	}
	next, err := h.Store.NextEpisode(ctx, id)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		writeStoreError(w, err, "episode.not_found")
		return
	}
	show, err := h.Store.GetShow(ctx, e.ShowID)
	if writeStoreError(w, err, "show.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"episode": e, "show": show, "files": files, "progress": progress, "next": next,
	})
}

// GET /sceniter/genres
func (h *Handler) Genres(w http.ResponseWriter, r *http.Request) {
	g, err := h.Store.Genres(r.Context())
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, g)
}

// GET /sceniter/search?q= – filmy a seriály podle názvu.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"movies": []model.Movie{}, "shows": []model.Show{}})
		return
	}
	movies, err := h.Store.ListMovies(r.Context(), storage.MovieFilter{Query: q, Limit: 30})
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	shows, err := h.Store.ListShows(r.Context(), storage.ShowFilter{Query: q, Limit: 30})
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"movies": movies, "shows": shows})
}

// ---- rozkoukanost ----

// GET /sceniter/progress?in_progress=1
func (h *Handler) ListProgress(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.ListProgress(r.Context(), userID(r), storage.ProgressFilter{
		InProgress: r.URL.Query().Get("in_progress") == "1", Limit: 500,
	})
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// PUT /sceniter/progress – pozice z přehrávače. 409 progress.stale znamená,
// že jiné zařízení zapsalo novější pozici; klient ji má převzít.
func (h *Handler) SaveProgress(w http.ResponseWriter, r *http.Request) {
	var req service.ProgressRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	err := h.Library.SaveProgress(r.Context(), userID(r), req)
	switch {
	case errors.Is(err, storage.ErrStale):
		writeError(w, http.StatusConflict, "progress.stale", "na serveru je novější pozice")
	case errors.Is(err, storage.ErrDuplicateEvent):
		w.WriteHeader(http.StatusNoContent)
	case writeStoreError(w, err, "sceniter.not_found"):
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// POST /sceniter/progress/sync – dávka pozic z offline zařízení (budoucí TV
// klienti). Každá událost má event_id; opakovaná dávka se nezapočte dvakrát.
func (h *Handler) SyncProgress(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string                    `json:"device_id"`
		Events   []service.ProgressRequest `json:"events"`
	}
	if err := readJSON(r, &req); err != nil || len(req.Events) > 500 {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku (max. 500 událostí)")
		return
	}
	type result struct {
		EventID *uuid.UUID `json:"event_id"`
		Status  string     `json:"status"`
		Error   string     `json:"error,omitempty"`
	}
	out := make([]result, 0, len(req.Events))
	for _, ev := range req.Events {
		if ev.DeviceID == "" {
			ev.DeviceID = req.DeviceID
		}
		res := result{EventID: ev.EventID, Status: "applied"}
		if ev.EventID == nil {
			res.Status, res.Error = "rejected", "chybí event_id"
			out = append(out, res)
			continue
		}
		err := h.Library.SaveProgress(r.Context(), userID(r), ev)
		switch {
		case err == nil:
		case errors.Is(err, storage.ErrStale):
			res.Status = "stale"
		case errors.Is(err, storage.ErrDuplicateEvent):
			res.Status = "duplicate"
		case errors.Is(err, service.ErrInvalid), errors.Is(err, storage.ErrNotFound):
			res.Status, res.Error = "rejected", err.Error()
		default:
			writeStoreError(w, err, "sceniter.not_found")
			return
		}
		out = append(out, res)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

// PUT /sceniter/progress/{kind}/{id}/watched – označit jako zhlédnuté.
func (h *Handler) MarkWatched(w http.ResponseWriter, r *http.Request) {
	h.setWatched(w, r, true)
}

// DELETE /sceniter/progress/{kind}/{id} – označit jako nezhlédnuté.
func (h *Handler) ResetProgress(w http.ResponseWriter, r *http.Request) {
	h.setWatched(w, r, false)
}

func (h *Handler) setWatched(w http.ResponseWriter, r *http.Request, watched bool) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if writeStoreError(w, h.Library.MarkWatched(r.Context(), userID(r), chi.URLParam(r, "kind"), id, watched), "sceniter.not_found") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- seznam a hodnocení ----

// GET /sceniter/watchlist
func (h *Handler) Watchlist(w http.ResponseWriter, r *http.Request) {
	list, err := h.Library.Watchlist(r.Context(), userID(r))
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// titleRef přečte {kind}/{id} pro film nebo seriál.
func titleRef(w http.ResponseWriter, r *http.Request) (movieID, showID *uuid.UUID, ok bool) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return nil, nil, false
	}
	switch chi.URLParam(r, "kind") {
	case model.KindMovie:
		return &id, nil, true
	case model.KindShow:
		return nil, &id, true
	}
	writeError(w, http.StatusBadRequest, "validation.invalid", "druh musí být movie nebo show")
	return nil, nil, false
}

// PUT|DELETE /sceniter/watchlist/{kind}/{id}
func (h *Handler) SetWatchlist(w http.ResponseWriter, r *http.Request) {
	movieID, showID, ok := titleRef(w, r)
	if !ok {
		return
	}
	err := h.Store.SetWatchlist(r.Context(), userID(r), movieID, showID, r.Method == http.MethodPut)
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PUT /sceniter/ratings/{kind}/{id}  {"rating": 1–10 | null}
func (h *Handler) SetRating(w http.ResponseWriter, r *http.Request) {
	movieID, showID, ok := titleRef(w, r)
	if !ok {
		return
	}
	var req struct {
		Rating *int `json:"rating"`
	}
	if err := readJSON(r, &req); err != nil || (req.Rating != nil && (*req.Rating < 1 || *req.Rating > 10)) {
		writeError(w, http.StatusBadRequest, "validation.invalid", "hodnocení musí být 1–10 nebo null")
		return
	}
	if writeStoreError(w, h.Store.SetRating(r.Context(), userID(r), movieID, showID, req.Rating), "sceniter.not_found") {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- úpravy a párování (editor) ----

// PATCH /sceniter/movies/{id}, /sceniter/shows/{id}
func (h *Handler) PatchTitle(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var p storage.TitlePatch
	if err := readJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	if p.Title != nil && strings.TrimSpace(*p.Title) == "" {
		writeError(w, http.StatusBadRequest, "validation.invalid", "název nesmí být prázdný")
		return
	}
	kind := kindFromPath(r)
	if writeStoreError(w, h.Store.PatchTitle(r.Context(), kind, id, p), kind+".not_found") {
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: AuditEdit, TargetType: kind, TargetID: id.String(), Details: p})
	w.WriteHeader(http.StatusNoContent)
}

// GET /sceniter/metadata/search?kind=movie|show&q=&year=&provider=&language=&title=<id>
//
// Zdroj a jazyk, které klient neuvede, se vezmou z knihovny titulu (title);
// bez titulu je výchozí zdroj první nainstalovaný, který druh umí.
func (h *Handler) MetadataSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	q := strings.TrimSpace(query.Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "validation.invalid", "chybí dotaz")
		return
	}
	kind := query.Get("kind")
	if kind != model.KindShow {
		kind = model.KindMovie
	}
	year := 0
	if y := intQuery(r, "year"); y != nil {
		year = *y
	}
	provider, lang := query.Get("provider"), query.Get("language")
	if t := query.Get("title"); t != "" && (provider == "" || lang == "") {
		if id, err := uuid.Parse(t); err == nil {
			if lib, err := h.Matcher.LibraryOf(r.Context(), kind, id); err == nil {
				if provider == "" && lib.Provider != nil {
					provider = *lib.Provider
				}
				if lang == "" {
					lang = lib.Language
				}
			}
		}
	}
	if provider == "" {
		for _, p := range h.metadataProviders() {
			if contains(p.Kinds, kind) {
				provider = p.Name
				break
			}
		}
	}
	res, err := h.Matcher.Search(r.Context(), provider, kind, lang, q, year)
	if h.writeMetadataError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"provider": provider, "results": res})
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (h *Handler) writeMetadataError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, service.ErrMetadataDisabled), errors.Is(err, plugin.ErrUnknownPlugin),
		errors.Is(err, plugin.ErrUnsupported):
		writeError(w, http.StatusServiceUnavailable, "sceniter.metadata_disabled", "zdroj metadat není nastaven nebo nainstalován")
	case plugin.IsNotFound(err):
		writeError(w, http.StatusNotFound, "sceniter.metadata_not_found", "titul u zdroje nenalezen")
	case errors.Is(err, storage.ErrConflict):
		writeError(w, http.StatusConflict, "sceniter.metadata_conflict", err.Error())
	default:
		writeError(w, http.StatusBadGateway, "sceniter.metadata_failed", "zdroj metadat neodpovídá: "+err.Error())
	}
	return true
}

// PUT /sceniter/movies/{id}/match, /sceniter/shows/{id}/match  {"provider": "tmdb", "id": "603"}
func (h *Handler) Match(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Provider string `json:"provider"`
		ID       string `json:"id"`
	}
	if err := readJSON(r, &req); err != nil || strings.TrimSpace(req.Provider) == "" || strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, "validation.invalid", "chybí provider nebo id")
		return
	}
	kind := kindFromPath(r)
	result := id
	var err error
	if kind == model.KindShow {
		err = h.Matcher.MatchShow(r.Context(), id, req.Provider, req.ID, model.MatchManual)
	} else {
		result, err = h.Matcher.MatchMovie(r.Context(), id, req.Provider, req.ID, model.MatchManual)
	}
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, kind+".not_found", "nenalezeno")
		return
	}
	if h.writeMetadataError(w, err) {
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: AuditMatch, TargetType: kind,
		TargetID: result.String(), Details: map[string]string{"provider": req.Provider, "id": req.ID}})
	writeJSON(w, http.StatusOK, map[string]string{"id": result.String()})
}

// DELETE /sceniter/movies/{id}/match – zruší spárování.
func (h *Handler) Unmatch(w http.ResponseWriter, r *http.Request) {
	h.setMatchState(w, r, model.MatchUnmatched, AuditUnmatch)
}

// PUT /sceniter/movies/{id}/ignore – titul se nebude párovat automaticky.
func (h *Handler) Ignore(w http.ResponseWriter, r *http.Request) {
	h.setMatchState(w, r, model.MatchIgnored, AuditUnmatch)
}

func (h *Handler) setMatchState(w http.ResponseWriter, r *http.Request, state, action string) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	kind := kindFromPath(r)
	if writeStoreError(w, h.Store.SetMatchState(r.Context(), kind, id, state), kind+".not_found") {
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: action, TargetType: kind,
		TargetID: id.String(), Details: map[string]string{"state": state}})
	w.WriteHeader(http.StatusNoContent)
}

// ---- administrace ----

// GET /sceniter/admin/scanner
func (h *Handler) ScannerStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Scanner.Status())
}

// POST /sceniter/admin/scanner/rescan
func (h *Handler) Rescan(w http.ResponseWriter, r *http.Request) {
	if err := h.Scanner.Rescan(); errors.Is(err, scanner.ErrScanRunning) {
		writeError(w, http.StatusConflict, "scanner.running", "kontrola knihovny už běží")
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: AuditRescan, TargetType: auditTargetLib})
	writeJSON(w, http.StatusAccepted, h.Scanner.Status())
}

// POST /sceniter/admin/metadata/match-all – spustí automatické párování.
func (h *Handler) MatchAll(w http.ResponseWriter, r *http.Request) {
	if !h.Matcher.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "sceniter.metadata_disabled", "není nainstalovaný žádný zdroj metadat")
		return
	}
	h.Matcher.Kick()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

// GET /sceniter/admin/library/unmatched
func (h *Handler) Unmatched(w http.ResponseWriter, r *http.Request) {
	movies, err := h.Store.ListMovies(r.Context(), storage.MovieFilter{MatchState: model.MatchUnmatched})
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	shows, err := h.Store.ListShows(r.Context(), storage.ShowFilter{MatchState: model.MatchUnmatched})
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"movies": movies, "shows": shows})
}

// GET /sceniter/admin/stats
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	st, err := h.Store.Stats(r.Context())
	if writeStoreError(w, err, "sceniter.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"library":    st,
		"scanner":    h.Scanner.Status(),
		"transcodes": len(h.HLS.List()),
		"metadata":   h.Matcher.Enabled(),
	})
}

// GET /sceniter/admin/transcode
func (h *Handler) TranscodeSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.HLS.List())
}

// DELETE /sceniter/admin/transcode/{session}
func (h *Handler) StopTranscode(w http.ResponseWriter, r *http.Request) {
	if !h.HLS.Stop(chi.URLParam(r, "session")) {
		writeError(w, http.StatusNotFound, "transcode.not_found", "převod neexistuje")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
