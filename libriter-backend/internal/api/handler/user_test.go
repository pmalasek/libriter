package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"libriter/internal/model"
)

func TestUserAppearancePersistsAcrossLoginAndProfileEdits(t *testing.T) {
	env := newAdminTestEnv(t)
	token, id := env.login(t, "appearance@example.com", model.RoleReader)
	path := "/users/" + id

	readUser := func() model.User {
		t.Helper()
		rec := env.do(t, http.MethodGet, path, token, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
		}
		var user model.User
		if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
			t.Fatal(err)
		}
		return user
	}
	before := readUser()
	if before.ColorScheme != "teal" || before.ThemeMode != "system" {
		t.Fatalf("default appearance: %+v", before)
	}
	for _, scheme := range []string{"blue", "violet", "green", "teal"} {
		for _, mode := range []string{"light", "dark", "system"} {
			rec := env.do(t, http.MethodPut, path+"/appearance", token, map[string]string{"color_scheme": scheme, "theme_mode": mode})
			if rec.Code != http.StatusOK {
				t.Fatalf("save %s/%s: %d %s", scheme, mode, rec.Code, rec.Body.String())
			}
			saved := readUser()
			if saved.ColorScheme != scheme || saved.ThemeMode != mode {
				t.Fatalf("saved appearance: %+v", saved)
			}
			if saved.Email != before.Email || saved.DisplayName != before.DisplayName || saved.Role != before.Role {
				t.Fatalf("appearance changed personal data: %+v", saved)
			}
		}
	}
	rec := env.do(t, http.MethodPut, path+"/appearance", token, map[string]string{"color_scheme": "violet", "theme_mode": "dark"})
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	rec = env.do(t, http.MethodPut, path, token, map[string]string{"display_name": "Nové jméno", "email": "updated@example.com"})
	if rec.Code != http.StatusOK {
		t.Fatalf("profile: %d %s", rec.Code, rec.Body.String())
	}

	// Nové přihlášení (např. na druhém zařízení) vrací uložený vzhled.
	user, _, err := env.auth.Login(context.Background(), "updated@example.com", "tajneheslo")
	if err != nil {
		t.Fatal(err)
	}
	if user.ColorScheme != "violet" || user.ThemeMode != "dark" {
		t.Fatalf("login appearance: %+v", user)
	}
	users, err := env.users.List(context.Background())
	if err != nil || len(users) != 1 || users[0].ColorScheme != "violet" {
		t.Fatalf("list: %+v, %v", users, err)
	}
}

func TestUserAppearanceRejectsUnauthorizedAndInvalidChanges(t *testing.T) {
	env := newAdminTestEnv(t)
	token, id := env.login(t, "reader@example.com", model.RoleReader)
	otherToken, _ := env.login(t, "other@example.com", model.RoleReader)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)
	path := "/users/" + id + "/appearance"
	valid := map[string]string{"color_scheme": "green", "theme_mode": "dark"}
	for _, tc := range []struct {
		token  string
		status int
	}{
		{"", http.StatusUnauthorized}, {otherToken, http.StatusForbidden}, {adminToken, http.StatusForbidden},
	} {
		if rec := env.do(t, http.MethodPut, path, tc.token, valid); rec.Code != tc.status {
			t.Errorf("authorization: got %d, want %d", rec.Code, tc.status)
		}
	}
	for _, body := range []any{
		map[string]string{"color_scheme": "unknown", "theme_mode": "dark"},
		map[string]string{"color_scheme": "blue", "theme_mode": "unknown"},
		map[string]string{"color_scheme": "blue"},
		map[string]string{"theme_mode": "dark"},
		map[string]string{"color_scheme": "blue", "theme_mode": "dark", "role": "admin"},
		map[string]any{"color_scheme": nil, "theme_mode": "dark"},
		map[string]any{"color_scheme": 42, "theme_mode": "dark"},
	} {
		if rec := env.do(t, http.MethodPut, path, token, body); rec.Code != http.StatusBadRequest {
			t.Errorf("invalid input %+v: got %d", body, rec.Code)
		}
	}
	user, err := env.users.GetByEmail(context.Background(), "reader@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if user.ColorScheme != "teal" || user.ThemeMode != "system" || user.Role != model.RoleReader {
		t.Fatalf("rejected request changed profile: %+v", user)
	}
}
