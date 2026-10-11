package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/middleware"
	"mediater/internal/core/service"
)

// MinPasswordLength je minimální délka hesla; platí pro registraci, změnu
// hesla i zakládání účtu adminem.
const MinPasswordLength = 8

type AuthHandler struct {
	svc *service.AuthService
	// settings rozhoduje, jestli je veřejná registrace zapnutá a jakou roli
	// nový účet dostane. Nastavuje se v administraci.
	settings service.RegistrationSource
}

func NewAuth(svc *service.AuthService, settings service.RegistrationSource) *AuthHandler {
	return &AuthHandler{svc: svc, settings: settings}
}

// GET /api/v1/auth/config  (veřejné)
//
// Přihlašovací stránka podle toho ukazuje či skrývá odkaz na registraci.
// Nic citlivého to neprozrazuje – zákaz stejně vynucuje Register níže.
func (h *AuthHandler) Config(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Registration(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání nastavení")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registration_enabled": settings.Enabled,
		"default_role":         settings.DefaultRole,
	})
}

// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Registration(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth.register_failed", "chyba při registraci")
		return
	}
	if !settings.Enabled {
		writeError(w, http.StatusForbidden, "auth.registration_disabled", "registrace nových uživatelů je vypnutá")
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Login       string `json:"login"`
		Password    string `json:"password"`
		UILanguage  string `json:"ui_language"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)

	if req.DisplayName == "" || req.Email == "" || len(req.Password) < MinPasswordLength {
		writeError(w, http.StatusBadRequest, "validation.register_fields", "display_name, email a heslo (min. 8 znaků) jsou povinné")
		return
	}

	user, token, err := h.svc.Register(r.Context(), req.DisplayName, req.Email, req.Login, req.Password, settings.DefaultRole, req.UILanguage)
	if errors.Is(err, service.ErrInvalidLanguage) {
		writeError(w, http.StatusBadRequest, "user.invalid_language", "nepodporovaný jazyk rozhraní")
		return
	}
	if errors.Is(err, service.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "user.email_taken", "email je již registrován")
		return
	}
	if WriteUserInputError(w, err) {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth.register_failed", "chyba při registraci")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user":  user,
		"token": token,
	})
}

// GET /api/v1/auth/stream-token  (přihlášený uživatel)
//
// Vydá krátkodobý token, kterým přehrávač otevírá audio soubory. Prvek <audio>
// neumí poslat hlavičku Authorization, takže token putuje v adrese – a do
// adresy přihlašovací token nepatří.
func (h *AuthHandler) StreamToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "auth.missing_token", "chybí autorizační token")
		return
	}

	token, expiresAt, err := h.svc.GenerateStreamToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth.token_failed", "token pro přehrávání se nepodařilo vydat")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": expiresAt,
	})
}

// POST /api/v1/auth/mobile-token  (přihlášený uživatel)
//
// Vymění běžné přihlášení za dlouhodobý token mobilní aplikace. Ta se
// přihlašuje jednou a pak jede i offline – s 72hodinovým přihlašovacím
// tokenem by po víkendu bez signálu žádala heslo u stažené knihy.
//
// Mobilní (ani televizní) token další mobilní token nevyrazí: uniklý token by
// se sám prodlužoval donekonečna a odhlášení by ho nikdy nedohnalo.
func (h *AuthHandler) MobileToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "auth.missing_token", "chybí autorizační token")
		return
	}
	if middleware.ScopeFromCtx(r.Context()) != "" {
		writeError(w, http.StatusForbidden, "auth.mobile_token_refresh", "mobilní token nelze obnovit mobilním tokenem – přihlaste se znovu")
		return
	}

	var req struct {
		// DeviceName jde jen do logu, ať je v serverovém záznamu poznat,
		// komu se token vydal. Nikde se neukládá.
		DeviceName string `json:"device_name"`
	}
	// Prázdné tělo je v pořádku – jméno zařízení je nepovinné.
	if r.ContentLength > 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
			return
		}
	}

	token, expiresAt, err := h.svc.GenerateMobileToken(userID, middleware.RoleFromCtx(r.Context()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth.token_failed", "token pro mobilní aplikaci se nepodařilo vydat")
		return
	}

	slog.Info("vydán mobilní token", "user_id", userID,
		"device", strings.TrimSpace(req.DeviceName), "expires_at", expiresAt)

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": expiresAt,
	})
}

// POST /api/v1/auth/login
//
// Pole login se porovnává s přihlašovacím jménem i s e-mailem uživatele.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	user, token, err := h.svc.Login(r.Context(), strings.TrimSpace(req.Login), req.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "auth.invalid_credentials", "neplatné přihlašovací údaje")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth.login_failed", "chyba při přihlášení")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":  user,
		"token": token,
	})
}

// POST /api/v1/auth/device/start  (veřejné)
//
// Televize zahájí párování: dostane device_code (nechá si ho) a user_code,
// který ukáže na obrazovce. Uživatel ho pak schválí na webu na stránce /link.
func (h *AuthHandler) DeviceStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceName string `json:"device_name"`
	}
	if r.ContentLength > 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
			return
		}
	}

	start, err := h.svc.StartDeviceAuth(r.Context(), req.DeviceName)
	if errors.Is(err, service.ErrDeviceBusy) {
		writeError(w, http.StatusServiceUnavailable, "auth.device_busy", "příliš mnoho rozpracovaných párování, zkuste to za chvíli")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth.token_failed", "párování se nepodařilo zahájit")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":       start.DeviceCode,
		"user_code":         start.UserCode,
		"verification_path": "/link",
		"expires_in":        start.ExpiresIn,
		"interval":          start.Interval,
	})
}

// POST /api/v1/auth/device/token  (veřejné)
//
// Televize se opakovaně ptá, jestli už uživatel kód schválil. Po schválení
// dostane dlouhodobý token se scope „tv“ a profil uživatele – stejný tvar
// jako odpověď na přihlášení.
func (h *AuthHandler) DeviceToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := readJSON(r, &req); err != nil || req.DeviceCode == "" {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	user, token, expiresAt, err := h.svc.PollDeviceToken(r.Context(), req.DeviceCode)
	switch {
	case errors.Is(err, service.ErrDevicePending):
		writeError(w, http.StatusPreconditionRequired, "auth.device_pending", "čeká se na schválení")
		return
	case errors.Is(err, service.ErrDeviceSlowDown):
		writeError(w, http.StatusTooManyRequests, "auth.device_slow_down", "zkoušejte to méně často")
		return
	case errors.Is(err, service.ErrDeviceExpired):
		writeError(w, http.StatusGone, "auth.device_expired", "párovací kód vypršel")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "auth.token_failed", "token pro televizi se nepodařilo vydat")
		return
	}

	slog.Info("vydán televizní token", "user_id", user.ID, "expires_at", expiresAt)

	writeJSON(w, http.StatusOK, map[string]any{
		"user":       user,
		"token":      token,
		"expires_at": expiresAt,
	})
}

// GET /api/v1/auth/device/{code}  (přihlášený uživatel)
//
// Stránka schválení podle toho ukáže, které zařízení se přihlašuje.
func (h *AuthHandler) DeviceInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.svc.DeviceByUserCode(r.Context(), chi.URLParam(r, "code"))
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "auth.device_code_invalid", "kód neexistuje nebo vypršel")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// POST /api/v1/auth/device/approve  (přihlášený uživatel)
//
// Schválí párování: televize se přihlásí účtem toho, kdo kód potvrdil.
func (h *AuthHandler) DeviceApprove(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "auth.missing_token", "chybí autorizační token")
		return
	}
	var req struct {
		UserCode string `json:"user_code"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	info, err := h.svc.ApproveDevice(r.Context(), userID, req.UserCode)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "auth.device_code_invalid", "kód neexistuje nebo vypršel")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.internal", "párování se nepodařilo schválit")
		return
	}

	slog.Info("schváleno párování televize", "user_id", userID, "device", info.DeviceName)
	writeJSON(w, http.StatusOK, info)
}
