package handler

import (
	"errors"
	"net/http"
	"strings"

	"libriter/internal/api/middleware"
	"libriter/internal/model"
	"libriter/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type UserHandler struct {
	svc *service.UserService
}

func NewUser(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// GET /api/v1/users  (admin)
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání uživatelů")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

// GET /api/v1/users/{id}  (admin nebo vlastní profil)
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	callerRole := middleware.RoleFromCtx(r.Context())

	if callerRole != model.RoleAdmin && callerID != id {
		writeError(w, http.StatusForbidden, "nedostatečná oprávnění")
		return
	}

	u, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "uživatel nenalezen")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání uživatele")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// PUT /api/v1/users/{id}  (admin nebo vlastní profil)
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	callerRole := middleware.RoleFromCtx(r.Context())

	if callerRole != model.RoleAdmin && callerID != id {
		writeError(w, http.StatusForbidden, "nedostatečná oprávnění")
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)

	if req.DisplayName == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "display_name a email jsou povinné")
		return
	}

	u, err := h.svc.Update(r.Context(), id, req.DisplayName, req.Email)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "uživatel nenalezen")
		return
	}
	if errors.Is(err, service.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email je již použit")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při aktualizaci")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// PUT /api/v1/users/{id}/password  (admin nebo vlastní profil)
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	callerRole := middleware.RoleFromCtx(r.Context())

	if callerRole != model.RoleAdmin && callerID != id {
		writeError(w, http.StatusForbidden, "nedostatečná oprávnění")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "heslo musí mít alespoň 8 znaků")
		return
	}

	if err := h.svc.ChangePassword(r.Context(), id, req.Password); errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "uživatel nenalezen")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při změně hesla")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/v1/users/{id}  (admin)
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "uživatel nenalezen")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při mazání")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PUT /api/v1/users/{id}/role  (admin)
func (h *UserHandler) SetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	if err := h.svc.SetRole(r.Context(), id, req.Role); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "uživatel nenalezen")
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "role": req.Role})
}

// --- helper ---

func parseUUID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		writeError(w, http.StatusBadRequest, "neplatné ID")
		return uuid.UUID{}, false
	}
	return id, true
}
