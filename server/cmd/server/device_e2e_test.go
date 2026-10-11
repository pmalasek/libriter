package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// TestDevicePairing projde párování televize: TV si vyžádá kód, uživatel ho
// schválí přihlášený na webu a TV dostane token, se kterým chodí do API.
func TestDevicePairing(t *testing.T) {
	r, deps := testRouterDeps(t)
	e := &sceniterEnv{t: t, router: r}

	if _, err := deps.Users.Create(context.Background(), "Divák", "divak@example.com", "", "tajneheslo", "reader"); err != nil {
		t.Fatal(err)
	}
	var login struct{ Token string }
	rec := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "divak@example.com", "password": "tajneheslo"})
	if json.Unmarshal(rec.Body.Bytes(), &login) != nil || login.Token == "" {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}

	start := func() (deviceCode, userCode string) {
		var out struct {
			DeviceCode       string `json:"device_code"`
			UserCode         string `json:"user_code"`
			VerificationPath string `json:"verification_path"`
		}
		rec := e.do("POST", "/api/v1/auth/device/start", "", map[string]string{"device_name": "Apple TV"})
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.VerificationPath != "/link" {
			t.Fatalf("start: %d %s", rec.Code, rec.Body.String())
		}
		return out.DeviceCode, out.UserCode
	}

	// Neschválený kód čeká.
	pendingCode, _ := start()
	if rec := e.do("POST", "/api/v1/auth/device/token", "", map[string]string{"device_code": pendingCode}); rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("čekající: %d %s", rec.Code, rec.Body.String())
	}

	deviceCode, userCode := start()
	if rec := e.do("GET", "/api/v1/auth/device/"+userCode, "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("info bez přihlášení: %d", rec.Code)
	}
	var info struct {
		DeviceName string `json:"device_name"`
	}
	rec = e.do("GET", "/api/v1/auth/device/"+userCode, login.Token, nil)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &info) != nil || info.DeviceName != "Apple TV" {
		t.Fatalf("info: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e.do("POST", "/api/v1/auth/device/approve", login.Token, map[string]string{"user_code": "XXXX-XXXX"}); rec.Code != http.StatusNotFound {
		t.Fatalf("neznámý kód: %d", rec.Code)
	}
	if rec := e.do("POST", "/api/v1/auth/device/approve", login.Token, map[string]string{"user_code": userCode}); rec.Code != http.StatusOK {
		t.Fatalf("schválení: %d %s", rec.Code, rec.Body.String())
	}

	var tv struct {
		Token string
		User  struct{ Email string }
	}
	rec = e.do("POST", "/api/v1/auth/device/token", "", map[string]string{"device_code": deviceCode})
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &tv) != nil || tv.Token == "" || tv.User.Email != "divak@example.com" {
		t.Fatalf("token: %d %s", rec.Code, rec.Body.String())
	}
	// Kód jde vyměnit jen jednou.
	if rec := e.do("POST", "/api/v1/auth/device/token", "", map[string]string{"device_code": deviceCode}); rec.Code != http.StatusGone {
		t.Fatalf("druhá výměna: %d", rec.Code)
	}

	// TV token pouští do API a k streamovacímu tokenu, ne k mobilnímu tokenu.
	if rec := e.do("GET", "/api/v1/auth/stream-token", tv.Token, nil); rec.Code != http.StatusOK {
		t.Fatalf("stream-token s TV tokenem: %d", rec.Code)
	}
	if rec := e.do("POST", "/api/v1/auth/mobile-token", tv.Token, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("mobile-token s TV tokenem: %d", rec.Code)
	}
}
