package handler

import (
	"errors"
	"net/http"

	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/service"
)

// similarLimit je počet titulů v polici „Podobné tituly“.
const similarLimit = 12

// GET /sceniter/people/{id} – osoba a její filmy a seriály v knihovně.
func (h *Handler) Person(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	p, err := h.Store.GetPerson(r.Context(), id)
	if writeStoreError(w, err, "person.not_found") {
		return
	}
	credits, err := h.Store.PersonCredits(r.Context(), id)
	if writeStoreError(w, err, "person.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, model.PersonDetail{Person: *p, Credits: credits})
}

// GET /sceniter/movies/{id}/similar
func (h *Handler) SimilarMovies(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	list, err := h.Store.SimilarMovies(r.Context(), id, similarLimit)
	if writeStoreError(w, err, "movie.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GET /sceniter/shows/{id}/similar
func (h *Handler) SimilarShows(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	list, err := h.Store.SimilarShows(r.Context(), id, similarLimit)
	if writeStoreError(w, err, "show.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// POST /sceniter/admin/metadata/refresh-all – znovu načte metadata všech
// spárovaných titulů (na pozadí).
func (h *Handler) RefreshAll(w http.ResponseWriter, r *http.Request) {
	switch err := h.Matcher.RefreshAll(); {
	case errors.Is(err, service.ErrMetadataDisabled):
		writeError(w, http.StatusServiceUnavailable, "sceniter.metadata_disabled", "není nainstalovaný žádný zdroj metadat")
	case errors.Is(err, service.ErrRefreshRunning):
		writeError(w, http.StatusConflict, "sceniter.refresh_running", err.Error())
	case err != nil:
		writeStoreError(w, err, "sceniter.not_found")
	default:
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
	}
}

// GET /sceniter/admin/metadata/refresh – průběh obnovy metadat.
func (h *Handler) RefreshStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.Matcher.RefreshStatusNow())
}
