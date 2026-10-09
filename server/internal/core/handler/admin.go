package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"mediater/internal/core/middleware"
	"mediater/internal/core/model"
	"mediater/internal/core/service"

	"github.com/google/uuid"
)

// AdminHandler obsluhuje administraci společnou všem produktům: zakládání
// účtů, audit a nastavení registrace. Správu knihovny má každý produkt vlastní.
type AdminHandler struct {
	users        *service.UserService
	registration *service.RegistrationService
	audit        *service.AuditService
}

func NewAdmin(users *service.UserService, registration *service.RegistrationService, audit *service.AuditService) *AdminHandler {
	return &AdminHandler{users: users, registration: registration, audit: audit}
}

// Stránkování auditu; strop musí odpovídat stropu v service.AuditService.
const (
	auditPageSize    = 50
	auditMaxPageSize = 200
)

// ActorID vrátí ID přihlášeného administrátora pro zápis do audit logu.
func ActorID(r *http.Request) uuid.UUID {
	id, _ := middleware.UserIDFromCtx(r.Context())
	return id
}

// POST /api/v1/admin/users  (admin)
//
// Založí účet s libovolnou rolí. Veřejná registrace roli neurčuje, takže
// editora i dalšího admina umí vyrobit jen tudy (nebo přes CLI).
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Login       string `json:"login"`
		Password    string `json:"password"`
		Role        string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role == "" {
		req.Role = model.RoleReader
	}

	if req.DisplayName == "" || req.Email == "" || len(req.Password) < MinPasswordLength {
		writeError(w, http.StatusBadRequest, "validation.register_fields", "display_name, email a heslo (min. 8 znaků) jsou povinné")
		return
	}
	if _, ok := model.RoleLevel[req.Role]; !ok {
		writeError(w, http.StatusBadRequest, "user.unknown_role", "neznámá role")
		return
	}

	u, err := h.users.Create(r.Context(), req.DisplayName, req.Email, req.Login, req.Password, req.Role)
	if WriteUserInputError(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při vytváření uživatele")
		return
	}

	h.audit.Record(r.Context(), ActorID(r), service.AuditEvent{
		Action:      service.AuditUserCreate,
		TargetType:  service.AuditTargetUser,
		TargetID:    u.ID.String(),
		TargetLabel: u.Email,
		Details:     map[string]string{"role": u.Role},
	})

	writeJSON(w, http.StatusCreated, u)
}

// GET /api/v1/admin/audit?limit=50&before=<id>  (admin)
//
// Stránkuje se kurzorem (id posledního záznamu), ne offsetem – log mezitím
// roste a offset by záznamy přeskakoval.
func (h *AdminHandler) Audit(w http.ResponseWriter, r *http.Request) {
	limit, err := intParam(r, "limit", auditPageSize)
	if err != nil || limit <= 0 {
		writeError(w, http.StatusBadRequest, "validation.invalid", "parametr limit musí být kladné číslo")
		return
	}
	// Strop držíme i tady, aby kurzor níž odpovídal skutečné velikosti stránky.
	if limit > auditMaxPageSize {
		limit = auditMaxPageSize
	}
	before, err := intParam(r, "before", 0)
	if err != nil || before < 0 {
		writeError(w, http.StatusBadRequest, "validation.invalid", "parametr before musí být nezáporné číslo")
		return
	}

	entries, err := h.audit.List(r.Context(), int(limit), before)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání auditu")
		return
	}

	// Kurzor posíláme jen tehdy, když stránka došla plná – jinak jsme na konci.
	var nextBefore *int64
	if len(entries) > 0 && len(entries) == int(limit) {
		last := entries[len(entries)-1].ID
		nextBefore = &last
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       entries,
		"next_before": nextBefore,
	})
}

// GET /api/v1/admin/settings/registration  (admin)
func (h *AdminHandler) RegistrationSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.registration.Registration(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání nastavení registrace")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// PUT /api/v1/admin/settings/registration  (admin)
func (h *AdminHandler) SetRegistrationSettings(w http.ResponseWriter, r *http.Request) {
	var req service.RegistrationSettings
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	saved, err := h.registration.SetRegistration(r.Context(), req)
	if errors.Is(err, service.ErrInvalidSetting) {
		writeError(w, http.StatusBadRequest, "validation.invalid", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při ukládání nastavení registrace")
		return
	}

	h.audit.Record(r.Context(), ActorID(r), service.AuditEvent{
		Action:     service.AuditSettingsRegistration,
		TargetType: service.AuditTargetSettings,
		TargetID:   service.SettingRegistration,
		Details:    saved,
	})

	writeJSON(w, http.StatusOK, saved)
}

func intParam(r *http.Request, name string, def int64) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
