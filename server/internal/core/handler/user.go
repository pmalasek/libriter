package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"mediater/internal/core/middleware"
	"mediater/internal/core/model"
	"mediater/internal/core/service"

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
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání uživatelů")
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
		writeError(w, http.StatusForbidden, "auth.forbidden", "nedostatečná oprávnění")
		return
	}

	u, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání uživatele")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// PUT /api/v1/users/{id}  (admin nebo vlastní profil, jen webové přihlášení)
//
// Kdo mění vlastní e-mail nebo login, musí zadat současné heslo
// (current_password) – e-mail a login jsou přihlašovací údaje a ukradená
// relace by si je jinak přepsala. Admin u cizího účtu heslo nezadává.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	callerRole := middleware.RoleFromCtx(r.Context())

	if callerRole != model.RoleAdmin && callerID != id {
		writeError(w, http.StatusForbidden, "auth.forbidden", "nedostatečná oprávnění")
		return
	}

	var req struct {
		DisplayName     string `json:"display_name"`
		Email           string `json:"email"`
		Login           string `json:"login"`
		CurrentPassword string `json:"current_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)

	if req.DisplayName == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "validation.profile_fields", "display_name a email jsou povinné")
		return
	}

	if callerID == id {
		current, err := h.svc.GetByID(r.Context(), id)
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při aktualizaci")
			return
		}
		login, err := service.NormalizeLogin(req.Login)
		if WriteUserInputError(w, err) {
			return
		}
		if (req.Email != current.Email || login != current.Login) &&
			!h.checkCurrentPassword(w, r, id, req.CurrentPassword) {
			return
		}
	}

	u, err := h.svc.Update(r.Context(), id, req.DisplayName, req.Email, req.Login)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
		return
	}
	if WriteUserInputError(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při aktualizaci")
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// checkCurrentPassword ověří current_password volajícího. Při chybě rovnou
// odpoví (400 chybí, 403 nesouhlasí) a vrátí false.
func (h *UserHandler) checkCurrentPassword(w http.ResponseWriter, r *http.Request, id uuid.UUID, password string) bool {
	if password == "" {
		writeError(w, http.StatusBadRequest, "auth.current_password_required", "zadejte současné heslo")
		return false
	}
	err := h.svc.CheckPassword(r.Context(), id, password)
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		writeError(w, http.StatusForbidden, "auth.current_password_invalid", "současné heslo nesouhlasí")
		return false
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
		return false
	case err != nil:
		writeError(w, http.StatusInternalServerError, "common.internal", "heslo se nepodařilo ověřit")
		return false
	}
	return true
}

// PUT /api/v1/users/{id}/appearance (pouze vlastní profil)
func (h *UserHandler) UpdateAppearance(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	callerID, authenticated := middleware.UserIDFromCtx(r.Context())
	if !authenticated || callerID != id {
		writeError(w, http.StatusForbidden, "user.own_profile_only", "vzhled lze měnit pouze ve vlastním profilu")
		return
	}
	var req struct {
		ColorScheme string `json:"color_scheme"`
		ThemeMode   string `json:"theme_mode"`
		// Product: "sceniter" = vlastní téma Sceniteru; jinak téma Libriteru.
		Product string `json:"product"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	u, err := h.svc.UpdateAppearance(r.Context(), id, req.Product, req.ColorScheme, req.ThemeMode)
	switch {
	case errors.Is(err, service.ErrInvalidSetting):
		writeError(w, http.StatusBadRequest, "validation.invalid", err.Error())
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "common.save_failed", "vzhled se nepodařilo uložit do profilu")
	default:
		writeJSON(w, http.StatusOK, u)
	}
}

// PUT /api/v1/users/{id}/language (pouze vlastní profil)
func (h *UserHandler) UpdateLanguage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	callerID, authenticated := middleware.UserIDFromCtx(r.Context())
	if !authenticated || callerID != id {
		writeError(w, http.StatusForbidden, "user.own_profile_only", "jazyk lze měnit pouze ve vlastním profilu")
		return
	}
	var req struct {
		UILanguage string `json:"ui_language"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	u, err := h.svc.UpdateLanguage(r.Context(), id, req.UILanguage)
	switch {
	case errors.Is(err, service.ErrInvalidLanguage):
		writeError(w, http.StatusBadRequest, "user.invalid_language", "nepodporovaný jazyk rozhraní")
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "common.save_failed", "jazyk se nepodařilo uložit do profilu")
	default:
		writeJSON(w, http.StatusOK, u)
	}
}

// PUT /api/v1/users/{id}/password  (admin nebo vlastní profil, jen webové přihlášení)
//
// Vlastní změna vyžaduje current_password; admin resetuje cizí heslo bez
// něj. Změna i reset odhlásí účet všude (zvýší verzi tokenů a odvolá
// zařízení). Při vlastní změně dostane volající v odpovědi nový token, aby
// web zůstal přihlášený.
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	callerRole := middleware.RoleFromCtx(r.Context())

	if callerRole != model.RoleAdmin && callerID != id {
		writeError(w, http.StatusForbidden, "auth.forbidden", "nedostatečná oprávnění")
		return
	}

	var req struct {
		Password        string `json:"password"`
		CurrentPassword string `json:"current_password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	if len(req.Password) < MinPasswordLength {
		writeError(w, http.StatusBadRequest, "validation.password_too_short", "heslo musí mít alespoň 8 znaků")
		return
	}
	if len(req.Password) > service.MaxPasswordBytes {
		writePasswordTooLong(w)
		return
	}

	self := callerID == id
	if self && !h.checkCurrentPassword(w, r, id, req.CurrentPassword) {
		return
	}

	if err := h.svc.ChangePassword(r.Context(), id, req.Password); errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při změně hesla")
		return
	}

	action := service.AuditUserPasswordReset
	if self {
		action = service.AuditUserPasswordChange
	}
	h.audit.Record(r.Context(), callerID, service.AuditEvent{
		Action:      action,
		TargetType:  service.AuditTargetUser,
		TargetID:    id.String(),
		TargetLabel: UserLabel(r, h.svc, id),
	})

	resp := map[string]string{"status": "ok"}
	if self {
		token, err := h.svc.IssueWebToken(r.Context(), id)
		if err != nil {
			// Heslo je změněné; web se jen bude muset znovu přihlásit.
			slog.Error("nový token po změně hesla", "user_id", id, "err", err)
		} else {
			resp["token"] = token
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/v1/users/{id}/devices  (admin nebo vlastní profil, jen webové přihlášení)
//
// Přihlášená zařízení (mobilní aplikace, televize). Webová přihlášení se tu
// neukazují – odhlásí je jen „Odhlásit všude“ nebo změna hesla.
func (h *UserHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	id, ok := h.selfOrAdmin(w, r)
	if !ok {
		return
	}
	devices, err := h.svc.ListDevices(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání zařízení")
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

// DELETE /api/v1/users/{id}/devices/{sid}  (admin nebo vlastní profil, jen webové přihlášení)
//
// Odvolá jedno zařízení: jeho token i streamovací tokeny, které si vydalo,
// okamžitě přestanou platit. Ostatní zařízení zůstanou přihlášená.
func (h *UserHandler) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := h.selfOrAdmin(w, r)
	if !ok {
		return
	}
	sid, ok := parseUUID(w, r, "sid")
	if !ok {
		return
	}
	d, err := h.svc.RevokeDevice(r.Context(), id, sid)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user.device_not_found", "zařízení nenalezeno")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.delete_failed", "zařízení se nepodařilo odhlásit")
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	h.audit.Record(r.Context(), callerID, service.AuditEvent{
		Action:      service.AuditUserDeviceRevoke,
		TargetType:  service.AuditTargetUser,
		TargetID:    id.String(),
		TargetLabel: UserLabel(r, h.svc, id),
		Details:     map[string]string{"session": d.ID.String(), "scope": d.Scope, "device": d.DeviceName},
	})
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/users/{id}/logout-all  (admin nebo vlastní profil, jen webové přihlášení)
//
// Zneplatní všechny tokeny účtu – web, mobil, televize i streamovací adresy.
// Kdo odhlašuje sám sebe, dostane v odpovědi nový token a zůstane přihlášený
// v tomto prohlížeči.
func (h *UserHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	id, ok := h.selfOrAdmin(w, r)
	if !ok {
		return
	}
	if err := h.svc.LogoutAll(r.Context(), id); errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "common.save_failed", "odhlášení se nepodařilo")
		return
	}

	callerID, _ := middleware.UserIDFromCtx(r.Context())
	h.audit.Record(r.Context(), callerID, service.AuditEvent{
		Action:      service.AuditUserLogoutAll,
		TargetType:  service.AuditTargetUser,
		TargetID:    id.String(),
		TargetLabel: UserLabel(r, h.svc, id),
	})

	resp := map[string]string{"status": "ok"}
	if callerID == id {
		token, err := h.svc.IssueWebToken(r.Context(), id)
		if err != nil {
			slog.Error("nový token po odhlášení všude", "user_id", id, "err", err)
		} else {
			resp["token"] = token
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// selfOrAdmin přečte {id} a pustí dál jen jeho vlastníka nebo admina.
func (h *UserHandler) selfOrAdmin(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return uuid.Nil, false
	}
	callerID, _ := middleware.UserIDFromCtx(r.Context())
	if middleware.RoleFromCtx(r.Context()) != model.RoleAdmin && callerID != id {
		writeError(w, http.StatusForbidden, "auth.forbidden", "nedostatečná oprávnění")
		return uuid.Nil, false
	}
	return id, true
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
		writeError(w, http.StatusBadRequest, "user.cannot_delete_self", "nemůžete smazat vlastní účet")
		return
	}

	// Popisek načteme dřív, než záznam zmizí.
	label := UserLabel(r, h.svc, id)

	err := h.svc.Delete(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
		return
	case errors.Is(err, service.ErrLastAdmin):
		writeError(w, http.StatusConflict, "user.last_admin", "nelze smazat posledního administrátora")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "common.delete_failed", "chyba při mazání")
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
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	var previousRole, label string
	if u, err := h.svc.GetByID(r.Context(), id); err == nil {
		previousRole, label = u.Role, u.Email
	}

	if err := h.svc.SetRole(r.Context(), id, req.Role); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
		case errors.Is(err, service.ErrLastAdmin):
			writeError(w, http.StatusConflict, "user.last_admin", "nelze odebrat roli poslednímu administrátorovi")
		default:
			writeError(w, http.StatusBadRequest, "user.unknown_role", err.Error())
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
// UserLabel vrátí popisek uživatele pro audit (e-mail, nebo ID, když účet chybí).
func UserLabel(r *http.Request, svc *service.UserService, id uuid.UUID) string {
	u, err := svc.GetByID(r.Context(), id)
	if err != nil {
		return ""
	}
	return u.Email
}

// --- helper ---

// writePasswordTooLong odpoví 400 na heslo delší, než bcrypt zpracuje.
func writePasswordTooLong(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "auth.password_too_long",
		fmt.Sprintf("heslo může mít nejvýš %d bajtů", service.MaxPasswordBytes))
}

// WriteUserInputError odpoví na chyby údajů uživatele, které zavinil klient
// (neplatný nebo obsazený login, obsazený e-mail). Vrací true, pokud odpověděl.
func WriteUserInputError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, service.ErrInvalidLogin):
		writeError(w, http.StatusBadRequest, "user.invalid_login", err.Error())
	case errors.Is(err, service.ErrLoginTaken):
		writeError(w, http.StatusConflict, "user.login_taken", "login je již použit")
	case errors.Is(err, service.ErrEmailTaken):
		writeError(w, http.StatusConflict, "user.email_taken", "email je již použit")
	case errors.Is(err, service.ErrPasswordTooLong):
		writePasswordTooLong(w)
	default:
		return false
	}
	return true
}
