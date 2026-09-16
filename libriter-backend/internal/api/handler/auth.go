package handler

import (
	"errors"
	"net/http"
	"strings"

	"libriter/internal/api/middleware"
	"libriter/internal/service"
)

// minPasswordLength je minimální délka hesla; platí pro registraci, změnu
// hesla i zakládání účtu adminem.
const minPasswordLength = 8

type AuthHandler struct {
	svc *service.AuthService
	// settings rozhoduje, jestli je veřejná registrace zapnutá a jakou roli
	// nový účet dostane. Nastavuje se v administraci.
	settings *service.SettingsService
}

func NewAuth(svc *service.AuthService, settings *service.SettingsService) *AuthHandler {
	return &AuthHandler{svc: svc, settings: settings}
}

// GET /api/v1/auth/config  (veřejné)
//
// Přihlašovací stránka podle toho ukazuje či skrývá odkaz na registraci.
// Nic citlivého to neprozrazuje – zákaz stejně vynucuje Register níže.
func (h *AuthHandler) Config(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Registration(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání nastavení")
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
		writeError(w, http.StatusInternalServerError, "chyba při registraci")
		return
	}
	if !settings.Enabled {
		writeError(w, http.StatusForbidden, "registrace nových uživatelů je vypnutá")
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)

	if req.DisplayName == "" || req.Email == "" || len(req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "display_name, email a heslo (min. 8 znaků) jsou povinné")
		return
	}

	user, token, err := h.svc.Register(r.Context(), req.DisplayName, req.Email, req.Password, settings.DefaultRole)
	if errors.Is(err, service.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email je již registrován")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při registraci")
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
		writeError(w, http.StatusUnauthorized, "chybí autorizační token")
		return
	}

	token, expiresAt, err := h.svc.GenerateStreamToken(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token pro přehrávání se nepodařilo vydat")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":      token,
		"expires_at": expiresAt,
	})
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	user, token, err := h.svc.Login(r.Context(), strings.TrimSpace(req.Email), req.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "neplatný email nebo heslo")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při přihlášení")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":  user,
		"token": token,
	})
}
