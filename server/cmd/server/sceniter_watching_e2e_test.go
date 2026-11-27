package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// Statistika sledování je jen pro admina; čtenář i editor dostanou 403.
func TestSceniterWatchingAdminOnly(t *testing.T) {
	// Trasy Sceniteru odpovídají jen se zapnutou videoknihovnou; prázdná
	// složka stačí, ffmpeg se nevolá.
	t.Setenv("VIDEO_ROOT", t.TempDir())
	t.Setenv("TRANSCODE_ROOT", t.TempDir())
	t.Setenv("SCENITER_IMAGE_ROOT", t.TempDir())
	t.Setenv("TMDB_API_KEY", "")

	r, deps := testRouterDeps(t)
	e := &sceniterEnv{t: t, router: r}
	ctx := context.Background()

	tokens := map[string]string{}
	ids := map[string]uuid.UUID{}
	for _, role := range []string{"admin", "editor", "reader"} {
		email := role + "@example.com"
		u, err := deps.Users.Create(ctx, role, email, "", "tajneheslo", role)
		if err != nil {
			t.Fatal(err)
		}
		ids[role] = u.ID
		var login struct{ Token string }
		rec := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": email, "password": "tajneheslo"})
		if err := json.Unmarshal(rec.Body.Bytes(), &login); err != nil || login.Token == "" {
			t.Fatalf("login %s: %d %s", role, rec.Code, rec.Body.String())
		}
		tokens[role] = login.Token
	}

	detailPath := "/api/v1/sceniter/admin/watching/" + ids["reader"].String()
	for _, path := range []string{"/api/v1/sceniter/admin/watching", detailPath} {
		for _, role := range []string{"reader", "editor"} {
			if rec := e.do("GET", path, tokens[role], nil); rec.Code != http.StatusForbidden {
				t.Errorf("%s jako %s: %d, chci 403", path, role, rec.Code)
			}
		}
		if rec := e.do("GET", path, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s bez přihlášení: %d, chci 401", path, rec.Code)
		}
	}

	var overview []struct {
		UserID        uuid.UUID `json:"user_id"`
		DisplayName   string    `json:"display_name"`
		Seconds       int64     `json:"seconds_watched"`
		LastWatchedAt *string   `json:"last_watched_at"`
	}
	e.token = tokens["admin"]
	if code := e.json("GET", "/api/v1/sceniter/admin/watching", nil, &overview); code != http.StatusOK || len(overview) != 3 {
		t.Fatalf("přehled: %d %+v", code, overview)
	}
	for _, row := range overview {
		if row.Seconds != 0 || row.LastWatchedAt != nil {
			t.Errorf("bez historie: %+v", row)
		}
	}

	var detail struct {
		User struct {
			ID    uuid.UUID `json:"id"`
			Email string    `json:"email"`
		} `json:"user"`
		Days    int               `json:"days"`
		Entries []json.RawMessage `json:"entries"`
	}
	if code := e.json("GET", detailPath+"?days=1000", nil, &detail); code != http.StatusOK ||
		detail.User.ID != ids["reader"] || detail.Days != 365 || detail.Entries == nil {
		t.Fatalf("detail: %d %+v", code, detail)
	}

	rec := e.do("GET", "/api/v1/sceniter/admin/watching/"+uuid.NewString(), tokens["admin"], nil)
	var apiErr struct{ Code string }
	_ = json.Unmarshal(rec.Body.Bytes(), &apiErr)
	if rec.Code != http.StatusNotFound || apiErr.Code != "user.not_found" {
		t.Errorf("neznámý účet: %d %s", rec.Code, rec.Body.String())
	}
	for _, q := range []string{"?days=0", "?days=abc"} {
		if rec := e.do("GET", detailPath+q, tokens["admin"], nil); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, chci 400", q, rec.Code)
		}
	}
	if rec := e.do("GET", "/api/v1/sceniter/admin/watching/nic", tokens["admin"], nil); rec.Code != http.StatusBadRequest {
		t.Errorf("neplatné ID: %d, chci 400", rec.Code)
	}
}
