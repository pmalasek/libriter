package handler

import (
	"net/http"
	"testing"

	"libriter/internal/model"
)

// Přihlásit se jde loginem i e-mailem, obojí bez ohledu na velikost písmen.
func TestLoginByLoginOrEmail(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	rec := env.do(t, http.MethodPost, "/admin/users", adminToken, map[string]string{
		"display_name": "Petr",
		"email":        "petr@example.com",
		"login":        "petr.m",
		"password":     "tajneheslo",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created model.User
	decodeJSON(t, rec.Body.Bytes(), &created)
	if created.Login != "petr.m" {
		t.Fatalf("login = %q, chtěno petr.m", created.Login)
	}

	for _, tc := range []struct {
		login, password string
		status          int
	}{
		{"petr.m", "tajneheslo", http.StatusOK},
		{"PETR.M", "tajneheslo", http.StatusOK},
		{"petr@example.com", "tajneheslo", http.StatusOK},
		{"Petr@Example.com", "tajneheslo", http.StatusOK},
		{"  petr.m  ", "tajneheslo", http.StatusOK},
		{"petr.m", "spatneheslo", http.StatusUnauthorized},
		{"neexistuje", "tajneheslo", http.StatusUnauthorized},
	} {
		rec := env.do(t, http.MethodPost, "/auth/login", "", map[string]string{"login": tc.login, "password": tc.password})
		if rec.Code != tc.status {
			t.Errorf("login %q: status = %d (%s), chtěno %d", tc.login, rec.Code, rec.Body.String(), tc.status)
			continue
		}
		if tc.status == http.StatusOK {
			var resp struct {
				User model.User `json:"user"`
			}
			decodeJSON(t, rec.Body.Bytes(), &resp)
			if resp.User.ID != created.ID {
				t.Errorf("login %q: přihlášen %s, chtěno %s", tc.login, resp.User.ID, created.ID)
			}
		}
	}

	// Staré pole email už požadavek na přihlášení nezná.
	rec = env.do(t, http.MethodPost, "/auth/login", "", map[string]string{"email": "petr@example.com", "password": "tajneheslo"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("pole email: status = %d, chtěno 400", rec.Code)
	}
}

// Login jde nastavit, změnit i odebrat úpravou profilu; obsazený nebo
// neplatný login se odmítne.
func TestUpdateUserLogin(t *testing.T) {
	env := newAdminTestEnv(t)
	token, id := env.login(t, "reader@example.com", model.RoleReader)
	otherToken, otherID := env.login(t, "other@example.com", model.RoleReader)
	path := "/users/" + id

	update := func(tok, p, login string) int {
		t.Helper()
		email := "reader@example.com"
		if p != path {
			email = "other@example.com"
		}
		rec := env.do(t, http.MethodPut, p, tok, map[string]string{
			"display_name": "Čtenář", "email": email, "login": login,
		})
		return rec.Code
	}

	if code := update(token, path, "ctenar"); code != http.StatusOK {
		t.Fatalf("nastavení loginu: %d", code)
	}
	if code := update(otherToken, "/users/"+otherID, "CTENAR"); code != http.StatusConflict {
		t.Errorf("obsazený login (jiná velikost písmen): %d, chtěno 409", code)
	}
	for _, bad := range []string{"ab", "má-diakritiku", "s mezerou", "s@zavinacem", "x123456789012345678901234567890123"} {
		if code := update(token, path, bad); code != http.StatusBadRequest {
			t.Errorf("neplatný login %q: %d, chtěno 400", bad, code)
		}
	}

	rec := env.do(t, http.MethodPost, "/auth/login", "", map[string]string{"login": "ctenar", "password": "tajneheslo"})
	if rec.Code != http.StatusOK {
		t.Fatalf("přihlášení loginem: %d", rec.Code)
	}

	// Prázdný login ho odebere a uvolní pro ostatní.
	if code := update(token, path, ""); code != http.StatusOK {
		t.Fatalf("odebrání loginu: %d", code)
	}
	rec = env.do(t, http.MethodPost, "/auth/login", "", map[string]string{"login": "ctenar", "password": "tajneheslo"})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("přihlášení odebraným loginem: %d, chtěno 401", rec.Code)
	}
	if code := update(otherToken, "/users/"+otherID, "ctenar"); code != http.StatusOK {
		t.Errorf("uvolněný login: %d, chtěno 200", code)
	}
}
