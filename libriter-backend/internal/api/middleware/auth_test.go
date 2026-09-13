package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"
)

// newAuthTestEnv připraví store, AuthService a chráněný handler, který jen
// vrátí roli, se kterou ho middleware pustil dál.
func newAuthTestEnv(t *testing.T) (*storage.Store, *service.AuthService, http.Handler) {
	t.Helper()

	conn, err := db.Open(context.Background(), config.DBConfig{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := storage.New(conn)
	authSvc := service.NewAuth(store, config.JWTConfig{Secret: "test-secret", ExpiryHours: time.Hour})

	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(RoleFromCtx(r.Context())))
	})

	return store, authSvc, Authenticate(authSvc)(protected)
}

func do(t *testing.T, h http.Handler, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Token smazaného uživatele musí skončit 401, i když je podpis pořád platný –
// dřív takový token procházel až do své expirace.
func TestAuthenticateRejectsDeletedUser(t *testing.T) {
	ctx := context.Background()
	store, authSvc, h := newAuthTestEnv(t)

	u, token, err := authSvc.Register(ctx, "Petr", "petr@example.com", "tajneheslo")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if rec := do(t, h, token); rec.Code != http.StatusOK {
		t.Fatalf("před smazáním: status = %d, chtěno 200", rec.Code)
	}

	userSvc := service.NewUser(store, authSvc)
	if err := userSvc.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if rec := do(t, h, token); rec.Code != http.StatusUnauthorized {
		t.Errorf("po smazání: status = %d, chtěno 401", rec.Code)
	}
}

// Role se bere z databáze, ne z tokenu: po snížení role musí starý token
// dostat nová, nižší práva.
func TestAuthenticateUsesRoleFromDB(t *testing.T) {
	ctx := context.Background()
	store, authSvc, h := newAuthTestEnv(t)

	userSvc := service.NewUser(store, authSvc)
	u, err := userSvc.Create(ctx, "Admin", "admin@example.com", "tajneheslo", model.RoleAdmin)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, token, err := authSvc.Login(ctx, "admin@example.com", "tajneheslo")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	if rec := do(t, h, token); rec.Body.String() != model.RoleAdmin {
		t.Fatalf("role = %q, chtěno %q", rec.Body.String(), model.RoleAdmin)
	}

	if err := userSvc.SetRole(ctx, u.ID, model.RoleReader); err != nil {
		t.Fatalf("SetRole: %v", err)
	}

	// Stejný token, ale role už musí být ta nová – SetRole zahodil cache.
	if rec := do(t, h, token); rec.Body.String() != model.RoleReader {
		t.Errorf("role po snížení = %q, chtěno %q", rec.Body.String(), model.RoleReader)
	}
}

func TestAuthenticateRejectsMissingAndInvalidToken(t *testing.T) {
	_, _, h := newAuthTestEnv(t)

	if rec := do(t, h, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("bez tokenu: status = %d, chtěno 401", rec.Code)
	}
	if rec := do(t, h, "nesmysl"); rec.Code != http.StatusUnauthorized {
		t.Errorf("neplatný token: status = %d, chtěno 401", rec.Code)
	}
}
