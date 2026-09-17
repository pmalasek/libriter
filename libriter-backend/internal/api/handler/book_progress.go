package handler

import (
	"errors"
	"net/http"

	"libriter/internal/service"
)

// GET /api/v1/books/progress
//
// Stav knih přihlášeného uživatele: co má rozposlouchané a co doposlechnuté.
// Knihovna si tím označí dlaždice, takže se to načítá jedním seznamem pro
// všechny knihy, ne po knize.
func (h *BookHandler) ListProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}

	progress, err := h.svc.ListProgress(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání stavu knih")
		return
	}
	writeJSON(w, http.StatusOK, progress)
}

// PUT /api/v1/books/{id}/progress
//
// Ruční označení knihy za doposlechnutou (finished: true) nebo návrat mezi
// neposlechnuté (finished: false) – například po omylem doposlechnuté knize
// nebo naopak u knihy poslechnuté jinde. Uloženou pozici v poslechu to nemění.
func (h *BookHandler) SetProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		Finished bool `json:"finished"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	var err error
	if req.Finished {
		err = h.svc.MarkFinished(r.Context(), userID, id)
	} else {
		err = h.svc.ResetProgress(r.Context(), userID, id)
	}

	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "kniha nenalezena")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "stav knihy se nepodařilo uložit")
	default:
		writeJSON(w, http.StatusOK, map[string]any{"finished": req.Finished})
	}
}

// DELETE /api/v1/books/{id}/progress
//
// Vrátí knihu mezi neposlechnuté – totéž co PUT s finished: false.
func (h *BookHandler) ResetProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := callerID(w, r)
	if !ok {
		return
	}
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	switch err := h.svc.ResetProgress(r.Context(), userID, id); {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "kniha nenalezena")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "stav knihy se nepodařilo smazat")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
