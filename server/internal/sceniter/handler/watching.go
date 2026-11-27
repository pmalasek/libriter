package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"mediater/internal/sceniter/service"
)

// GET /sceniter/admin/watching  (admin)
//
// Přehled sledování všech uživatelů. Čtení se neaudituje – zapisují se jen
// zásahy, které něco mění.
func (h *Handler) WatchingOverview(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Watching.Overview(r.Context())
	if err != nil {
		slog.Error("sceniter: přehled sledování", "err", err)
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání historie sledování")
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// GET /sceniter/admin/watching/{userId}?days=90  (admin)
//
// Historie sledování jednoho uživatele po titulech a dnech. days je 1–365
// (výchozí 90); větší hodnota se ořízne na maximum.
func (h *Handler) WatchingUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "userId")
	if !ok {
		return
	}
	days := service.WatchingDefaultDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "validation.invalid", "days musí být kladné celé číslo")
			return
		}
		days = min(n, service.WatchingMaxDays)
	}

	detail, err := h.Watching.UserDetail(r.Context(), id, days)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
	case err != nil:
		slog.Error("sceniter: historie sledování", "user_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání historie sledování")
	default:
		writeJSON(w, http.StatusOK, detail)
	}
}
