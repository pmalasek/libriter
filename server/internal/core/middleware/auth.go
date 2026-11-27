package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"mediater/internal/core/model"
	"mediater/internal/core/service"

	"github.com/google/uuid"
)

type contextKey string

const (
	ctxKeyUserID contextKey = "user_id"
	ctxKeyRole   contextKey = "role"
	ctxKeyScope  contextKey = "scope"
	ctxKeyClaims contextKey = "claims"
)

// Authenticate ověří JWT token z hlavičky Authorization: Bearer <token>.
// Pokud token chybí nebo je neplatný, vrátí 401.
//
// Platný podpis sám o sobě nestačí: token žije 72 hodin (token zařízení rok)
// a přežil by smazání účtu, snížení role, změnu hesla i odvolání zařízení.
// Proto ho ověří AuthService.Authorize proti databázi a do kontextu jde role
// odtud, ne ta z tokenu. Tokeny zařízení (mobil, televize) mají nejvýš roli
// čtenáře.
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

			role, err := authSvc.Authorize(r.Context(), claims)
			if err != nil {
				WriteAuthorizeError(w, userID, err)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			ctx = context.WithValue(ctx, ctxKeyRole, role)
			ctx = context.WithValue(ctx, ctxKeyScope, claims.Scope)
			ctx = context.WithValue(ctx, ctxKeyClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WriteAuthorizeError odpoví na chybu z AuthService.Authorize: smazaný účet
// a odvolané přihlášení jsou 401 (klient se odhlásí), ostatní 500. Používají
// ji i handlery streamování, které token čtou z adresy.
func WriteAuthorizeError(w http.ResponseWriter, userID uuid.UUID, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusUnauthorized, "auth.account_gone", "účet již neexistuje")
	case errors.Is(err, service.ErrSessionRevoked):
		writeError(w, http.StatusUnauthorized, "auth.session_revoked", "přihlášení bylo odvoláno – přihlaste se znovu")
	case errors.Is(err, service.ErrInvalidToken):
		writeError(w, http.StatusUnauthorized, "auth.invalid_token", "neplatný token")
	default:
		slog.Error("ověření uživatele", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, "common.internal", "chyba při ověření uživatele")
	}
}

// RequireWebSession pustí dál jen běžné webové přihlášení (prázdný scope).
// Tokeny zařízení (mobil, televize) dostanou 403: správa účtu, schvalování
// párování a vydávání dalších tokenů patří jen do webové relace – jinak by
// uniklý token zařízení šel sám prodlužovat nebo převzít účet.
func RequireWebSession(next http.Handler) http.Handler {
	return RequireWebSessionCode("auth.web_session_required", "tuto akci lze provést jen z webového přihlášení")(next)
}

// RequireWebSessionCode je RequireWebSession s vlastním kódem a textem chyby
// (POST /auth/mobile-token si drží kód, který znají vydaní klienti).
func RequireWebSessionCode(code, msg string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ScopeFromCtx(r.Context()) != "" {
				writeError(w, http.StatusForbidden, code, msg)
				return
			}
			next.ServeHTTP(w, r)
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
// řetězec je běžné přihlášení, service.ScopeMobile mobilní aplikace,
// service.ScopeTV televize.
func ScopeFromCtx(ctx context.Context) string {
	scope, _ := ctx.Value(ctxKeyScope).(string)
	return scope
}

// ClaimsFromCtx vrátí ověřené claims tokenu volajícího (nil mimo Authenticate).
// Potřebuje je vydání streamovacího tokenu, který dědí verzi a relaci zařízení.
func ClaimsFromCtx(ctx context.Context) *service.Claims {
	claims, _ := ctx.Value(ctxKeyClaims).(*service.Claims)
	return claims
}

// writeError odpoví JSON chybou se stabilním kódem, stejně jako handlery.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "code": code})
}
