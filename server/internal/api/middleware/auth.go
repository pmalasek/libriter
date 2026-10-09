package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"libriter/internal/model"
	"libriter/internal/service"

	"github.com/google/uuid"
)

type contextKey string

const (
	ctxKeyUserID contextKey = "user_id"
	ctxKeyRole   contextKey = "role"
	ctxKeyScope  contextKey = "scope"
)

// Authenticate ověří JWT token z hlavičky Authorization: Bearer <token>.
// Pokud token chybí nebo je neplatný, vrátí 401.
//
// Platný podpis sám o sobě nestačí: token žije 72 hodin a přežil by i smazání
// účtu nebo snížení role. Uživatel se proto dohledává v databázi a do kontextu
// jde role odtud, ne ta z tokenu.
func Authenticate(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "auth.missing_token", "chybí autorizační token")
				return
			}

			claims, err := authSvc.ParseToken(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				writeError(w, http.StatusUnauthorized, "auth.invalid_token", "neplatný token")
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "auth.invalid_token", "neplatný token")
				return
			}

			role, err := authSvc.ResolveRole(r.Context(), userID)
			if errors.Is(err, service.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "auth.account_gone", "účet již neexistuje")
				return
			}
			if err != nil {
				slog.Error("ověření uživatele", "user_id", userID, "err", err)
				writeError(w, http.StatusInternalServerError, "common.internal", "chyba při ověření uživatele")
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			ctx = context.WithValue(ctx, ctxKeyRole, role)
			ctx = context.WithValue(ctx, ctxKeyScope, claims.Scope)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole povolí přístup pouze uživatelům s dostatečnou rolí.
// Hierarchie: admin (1) > editor (2) > reader (3).
// Příklad: RequireRole(model.RoleEditor) = editor nebo admin.
func RequireRole(minRole string) func(http.Handler) http.Handler {
	minLevel := model.RoleLevel[minRole]
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(ctxKeyRole).(string)
			level, ok := model.RoleLevel[role]
			if !ok || level > minLevel {
				writeError(w, http.StatusForbidden, "auth.forbidden", "nedostatečná oprávnění")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserIDFromCtx vrátí ID přihlášeného uživatele z kontextu.
func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return v, ok
}

// RoleFromCtx vrátí roli přihlášeného uživatele z kontextu.
func RoleFromCtx(ctx context.Context) string {
	role, _ := ctx.Value(ctxKeyRole).(string)
	return role
}

// ScopeFromCtx vrátí scope tokenu, kterým se volající prokázal. Prázdný
// řetězec je běžné přihlášení, service.ScopeMobile mobilní aplikace.
func ScopeFromCtx(ctx context.Context) string {
	scope, _ := ctx.Value(ctxKeyScope).(string)
	return scope
}

// writeError odpoví JSON chybou se stabilním kódem, stejně jako handlery.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}
