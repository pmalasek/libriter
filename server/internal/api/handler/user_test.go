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

func TestUserLanguageDefaultsAndPersists(t *testing.T) {
	env := newAdminTestEnv(t)
	token, id := env.login(t, "lang@example.com", model.RoleReader)
	path := "/users/" + id

	user, err := env.users.GetByEmail(context.Background(), "lang@example.com")
	if err != nil || user.UILanguage != model.DefaultUILanguage {
		t.Fatalf("default language: %+v, %v", user, err)
	}
	for _, lang := range model.UILanguages {
		rec := env.do(t, http.MethodPut, path+"/language", token, map[string]string{"ui_language": lang})
		if rec.Code != http.StatusOK {
			t.Fatalf("save %s: %d %s", lang, rec.Code, rec.Body.String())
		}
		var saved model.User
		if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
			t.Fatal(err)
		}
		if saved.UILanguage != lang || saved.Role != model.RoleReader {
			t.Fatalf("saved language: %+v", saved)
		}
	}

	// Nové přihlášení vrací uložený jazyk.
	user, _, err = env.auth.Login(context.Background(), "lang@example.com", "tajneheslo")
	if err != nil || user.UILanguage != "es" {
		t.Fatalf("login language: %+v, %v", user, err)
	}
}

func TestUserLanguageRejectsUnauthorizedAndInvalidChanges(t *testing.T) {
	env := newAdminTestEnv(t)
	token, id := env.login(t, "reader@example.com", model.RoleReader)
	otherToken, _ := env.login(t, "other@example.com", model.RoleReader)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)
	path := "/users/" + id + "/language"
	for _, tc := range []struct {
		token  string
		status int
	}{
		{"", http.StatusUnauthorized}, {otherToken, http.StatusForbidden}, {adminToken, http.StatusForbidden},
	} {
		if rec := env.do(t, http.MethodPut, path, tc.token, map[string]string{"ui_language": "cs"}); rec.Code != tc.status {
			t.Errorf("authorization: got %d, want %d", rec.Code, tc.status)
		}
	}
	for _, body := range []any{
		map[string]string{"ui_language": "pl"},
		map[string]string{"ui_language": "CS"},
		map[string]string{"ui_language": ""},
		map[string]string{},
		map[string]any{"ui_language": 42},
	} {
		if rec := env.do(t, http.MethodPut, path, token, body); rec.Code != http.StatusBadRequest {
			t.Errorf("invalid input %+v: got %d", body, rec.Code)
		}
	}
	rec := env.do(t, http.MethodPut, path, token, map[string]string{"ui_language": "pl"})
	if code := errorCode(t, rec.Body.Bytes()); code != "user.invalid_language" {
		t.Errorf("error code = %q, want user.invalid_language", code)
	}
	rec = env.do(t, http.MethodPut, path, otherToken, map[string]string{"ui_language": "cs"})
	if code := errorCode(t, rec.Body.Bytes()); code != "user.own_profile_only" {
		t.Errorf("error code = %q, want user.own_profile_only", code)
	}
	rec = env.do(t, http.MethodPut, path, "", map[string]string{"ui_language": "cs"})
	if code := errorCode(t, rec.Body.Bytes()); code != "auth.missing_token" {
		t.Errorf("error code = %q, want auth.missing_token", code)
	}
	user, err := env.users.GetByEmail(context.Background(), "reader@example.com")
	if err != nil || user.UILanguage != model.DefaultUILanguage {
		t.Fatalf("rejected request changed profile: %+v, %v", user, err)
	}
}

func TestRegisterStoresUILanguage(t *testing.T) {
	env := newAdminTestEnv(t)
	register := func(email, lang string) (int, model.User) {
		t.Helper()
		body := map[string]string{"display_name": "Nový", "email": email, "password": "tajneheslo"}
		if lang != "" {
			body["ui_language"] = lang
		}
		rec := env.do(t, http.MethodPost, "/auth/register", "", body)
		var resp struct {
			User model.User `json:"user"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		return rec.Code, resp.User
	}

	if code, u := register("de@example.com", "de"); code != http.StatusCreated || u.UILanguage != "de" || u.Role != model.RoleReader {
		t.Errorf("register de: %d %+v", code, u)
	}
	if code, u := register("default@example.com", ""); code != http.StatusCreated || u.UILanguage != model.DefaultUILanguage {
		t.Errorf("register default: %d %+v", code, u)
	}
	if code, _ := register("bad@example.com", "xx"); code != http.StatusBadRequest {
		t.Errorf("register invalid language: %d", code)
	}
	if _, err := env.users.GetByEmail(context.Background(), "bad@example.com"); err == nil {
		t.Error("invalid language still created the user")
	}
}

// errorCode vrátí stabilní kód z chybové odpovědi API.
func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var resp struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("chybová odpověď není JSON: %v (%s)", err, body)
	}
	if resp.Error == "" {
		t.Errorf("chybová odpověď bez zprávy: %s", body)
	}
	return resp.Code
}
