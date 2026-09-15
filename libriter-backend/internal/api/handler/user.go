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
	// audit je volitelný (nil = akce se nezaznamenávají).
	audit *service.AuditService
}

func NewUser(svc *service.UserService, audit *service.AuditService) *UserHandler {
	return &UserHandler{svc: svc, audit: audit}
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

	if len(req.Password) < minPasswordLength {
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

	// Vlastní změna hesla je běžný úkon; do auditu patří jen reset cizího účtu.
	if callerID != id {
		h.audit.Record(r.Context(), callerID, service.AuditEvent{
			Action:      service.AuditUserPasswordReset,
			TargetType:  service.AuditTargetUser,
			TargetID:    id.String(),
			TargetLabel: userLabel(r, h.svc, id),
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/v1/users/{id}  (admin)
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	// Vlastní účet si admin smazat nemůže: buď by se odstřihl od administrace,
	// nebo (u posledního admina) nechal knihovnu bez správce.
	callerID, _ := middleware.UserIDFromCtx(r.Context())
	if callerID == id {
		writeError(w, http.StatusBadRequest, "nemůžete smazat vlastní účet")
		return
	}

	// Popisek načteme dřív, než záznam zmizí.
	label := userLabel(r, h.svc, id)

	err := h.svc.Delete(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "uživatel nenalezen")
		return
	case errors.Is(err, service.ErrLastAdmin):
		writeError(w, http.StatusConflict, "nelze smazat posledního administrátora")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "chyba při mazání")
		return
	}

	h.audit.Record(r.Context(), callerID, service.AuditEvent{
		Action:      service.AuditUserDelete,
		TargetType:  service.AuditTargetUser,
		TargetID:    id.String(),
		TargetLabel: label,
	})

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

	var previousRole, label string
	if u, err := h.svc.GetByID(r.Context(), id); err == nil {
		previousRole, label = u.Role, u.Email
	}

	if err := h.svc.SetRole(r.Context(), id, req.Role); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			writeError(w, http.StatusNotFound, "uživatel nenalezen")
		case errors.Is(err, service.ErrLastAdmin):
			writeError(w, http.StatusConflict, "nelze odebrat roli poslednímu administrátorovi")
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	h.audit.Record(r.Context(), callerID, service.AuditEvent{
		Action:      service.AuditUserRoleChange,
		TargetType:  service.AuditTargetUser,
		TargetID:    id.String(),
		TargetLabel: label,
		Details:     map[string]string{"from": previousRole, "to": req.Role},
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "role": req.Role})
}

// userLabel vrátí e-mail uživatele pro čitelný záznam v auditu.
func userLabel(r *http.Request, svc *service.UserService, id uuid.UUID) string {
	u, err := svc.GetByID(r.Context(), id)
	if err != nil {
		return ""
	}
	return u.Email
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
