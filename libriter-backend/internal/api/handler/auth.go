package handler

import (
	"errors"
	"net/http"
	"strings"

	"libriter/internal/service"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuth(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// POST /api/v1/auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
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

	if req.DisplayName == "" || req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "display_name, email a heslo (min. 8 znaků) jsou povinné")
		return
	}

	user, token, err := h.svc.Register(r.Context(), req.DisplayName, req.Email, req.Password)
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
