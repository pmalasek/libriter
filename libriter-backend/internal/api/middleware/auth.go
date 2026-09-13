package middleware

import (
	"context"
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
				http.Error(w, `{"error":"chybí autorizační token"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authSvc.ParseToken(strings.TrimPrefix(header, "Bearer "))
			if err != nil {
				http.Error(w, `{"error":"neplatný token"}`, http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				http.Error(w, `{"error":"neplatný token"}`, http.StatusUnauthorized)
				return
			}

			role, err := authSvc.ResolveRole(r.Context(), userID)
			if errors.Is(err, service.ErrNotFound) {
				http.Error(w, `{"error":"účet již neexistuje"}`, http.StatusUnauthorized)
				return
			}
			if err != nil {
				slog.Error("ověření uživatele", "user_id", userID, "err", err)
				http.Error(w, `{"error":"chyba při ověření uživatele"}`, http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			ctx = context.WithValue(ctx, ctxKeyRole, role)
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
				http.Error(w, `{"error":"nedostatečná oprávnění"}`, http.StatusForbidden)
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
