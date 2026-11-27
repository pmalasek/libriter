package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mediater/internal/core/config"
	"mediater/internal/core/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func newSessionTestAuth(t *testing.T) (*AuthService, *UserService, *model.User) {
	t.Helper()
	store := newDeviceTestStore(t)
	a := NewAuth(store, config.JWTConfig{Secret: "x", ExpiryHours: time.Hour})
	users := NewUser(store, a)
	u, err := users.Create(context.Background(), "Admin", "admin@example.com", "", "tajneheslo", model.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	return a, users, u
}

// authorize je ParseToken + Authorize, jak to dělá middleware.
func authorize(t *testing.T, a *AuthService, token string) (string, error) {
	t.Helper()
	claims, err := a.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	return a.Authorize(context.Background(), claims)
}

// Tokeny vydané před nasazením nemají ver ani jti: platí dál (verze 0),
// odvolat je jde jen zvýšením verze.
func TestAuthorizeLegacyTokens(t *testing.T) {
	ctx := context.Background()
	a, users, u := newSessionTestAuth(t)
	sign := func(scope string) string {
		// Mapa claims bez „ver“ – přesně jako starý token.
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"role": u.Role, "scope": scope, "sub": u.ID.String(),
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		s, err := tok.SignedString([]byte("x"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	web, mobile := sign(""), sign(ScopeMobile)

	if role, err := authorize(t, a, web); err != nil || role != model.RoleAdmin {
		t.Fatalf("starý webový token: %q %v", role, err)
	}
	if role, err := authorize(t, a, mobile); err != nil || role != model.RoleReader {
		t.Fatalf("starý mobilní token: %q %v (role zařízení má být reader)", role, err)
	}

	if err := users.LogoutAll(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	for name, tok := range map[string]string{"web": web, "mobil": mobile} {
		if _, err := authorize(t, a, tok); !errors.Is(err, ErrSessionRevoked) {
			t.Errorf("%s po odhlášení všude: %v", name, err)
		}
	}
}

func TestEffectiveRole(t *testing.T) {
	for _, tc := range []struct{ role, scope, want string }{
		{model.RoleAdmin, "", model.RoleAdmin},
		{model.RoleEditor, "", model.RoleEditor},
		{model.RoleAdmin, ScopeMobile, model.RoleReader},
		{model.RoleEditor, ScopeTV, model.RoleReader},
		{model.RoleReader, ScopeTV, model.RoleReader},
		{"nesmysl", ScopeMobile, model.RoleReader},
	} {
		if got := EffectiveRole(tc.role, tc.scope); got != tc.want {
			t.Errorf("EffectiveRole(%q, %q) = %q, chci %q", tc.role, tc.scope, got, tc.want)
		}
	}
}

// Odvolání zařízení platí okamžitě i přes cache; ostatní relace jedou dál.
func TestRevokeDeviceInvalidatesCache(t *testing.T) {
	ctx := context.Background()
	a, users, u := newSessionTestAuth(t)
	m1, _, err := a.GenerateMobileToken(ctx, u.ID, "Jedna")
	if err != nil {
		t.Fatal(err)
	}
	m2, _, err := a.GenerateMobileToken(ctx, u.ID, "Dva")
	if err != nil {
		t.Fatal(err)
	}
	// Naplní cache.
	for _, tok := range []string{m1, m2} {
		if _, err := authorize(t, a, tok); err != nil {
			t.Fatal(err)
		}
	}
	c1, _ := a.ParseToken(m1)
	sid, _ := uuid.Parse(c1.ID)
	if _, err := users.RevokeDevice(ctx, u.ID, sid); err != nil {
		t.Fatal(err)
	}
	if _, err := authorize(t, a, m1); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("odvolaný token: %v", err)
	}
	if _, err := authorize(t, a, m2); err != nil {
		t.Fatalf("druhé zařízení: %v", err)
	}

	// Streamovací token odvolaného zařízení neprojde, webového ano.
	s1, _, _ := a.GenerateStreamToken(c1)
	sc, err := a.ParseStreamToken(s1)
	if err != nil || sc.SID != c1.ID {
		t.Fatalf("stream token: %+v %v", sc, err)
	}
	if _, err := a.Authorize(ctx, sc); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("stream token odvolaného zařízení: %v", err)
	}
	webTok, err := a.IssueWebToken(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	wc, _ := a.ParseToken(webTok)
	sw, _, _ := a.GenerateStreamToken(wc)
	swc, _ := a.ParseStreamToken(sw)
	if role, err := a.Authorize(ctx, swc); err != nil || swc.SID != "" || role != model.RoleAdmin {
		t.Fatalf("webový stream token: %q %+v %v", role, swc, err)
	}

	// Relace cizího uživatele se nedá použít ani odvolat.
	other, err := users.Create(ctx, "Jiný", "jiny@example.com", "", "tajneheslo", model.RoleReader)
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := a.ParseToken(m2)
	sid2, _ := uuid.Parse(c2.ID)
	if _, err := users.RevokeDevice(ctx, other.ID, sid2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("odvolání cizí relace: %v", err)
	}
	forged, _ := a.signToken(Claims{Scope: ScopeMobile, RegisteredClaims: jwt.RegisteredClaims{
		ID: c2.ID, Subject: other.ID.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}})
	if _, err := authorize(t, a, forged); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("cizí relace v tokenu: %v", err)
	}
}

// last_seen_at se zapisuje nejvýš jednou za hodinu.
func TestDeviceSessionLastSeen(t *testing.T) {
	ctx := context.Background()
	a, users, u := newSessionTestAuth(t)
	now := time.Now()
	a.now = func() time.Time { return now }

	tok, _, err := a.GenerateMobileToken(ctx, u.ID, "  Telefon\x07 ")
	if err != nil {
		t.Fatal(err)
	}
	seen := func() time.Time {
		t.Helper()
		list, err := users.ListDevices(ctx, u.ID)
		if err != nil || len(list) != 1 {
			t.Fatalf("zařízení: %+v %v", list, err)
		}
		if list[0].DeviceName != "Telefon" || list[0].Scope != ScopeMobile {
			t.Fatalf("relace: %+v", list[0])
		}
		return list[0].LastSeenAt
	}
	created := seen()

	now = now.Add(30 * time.Minute)
	a.InvalidateUser(u.ID)
	if _, err := authorize(t, a, tok); err != nil {
		t.Fatal(err)
	}
	if !seen().Equal(created) {
		t.Fatal("last_seen_at se zapsal dřív než po hodině")
	}

	now = now.Add(31 * time.Minute)
	if _, err := authorize(t, a, tok); err != nil {
		t.Fatal(err)
	}
	if got := seen(); got.Unix() != now.Unix() {
		t.Fatalf("last_seen_at = %v, chci %v", got, now)
	}
}

// Změna hesla zvýší verzi tokenů; nový webový token platí, starý ne.
func TestChangePasswordBumpsVersion(t *testing.T) {
	ctx := context.Background()
	a, users, u := newSessionTestAuth(t)
	_, old, err := a.Login(ctx, "admin@example.com", "tajneheslo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authorize(t, a, old); err != nil {
		t.Fatal(err)
	}
	if err := users.ChangePassword(ctx, u.ID, "noveheslo"); err != nil {
		t.Fatal(err)
	}
	if _, err := authorize(t, a, old); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("starý token: %v", err)
	}
	fresh, err := users.IssueWebToken(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := a.ParseToken(fresh); c.Ver != 1 {
		t.Fatalf("verze nového tokenu: %d", c.Ver)
	}
	if _, err := authorize(t, a, fresh); err != nil {
		t.Fatalf("nový token: %v", err)
	}
	if err := users.CheckPassword(ctx, u.ID, "tajneheslo"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("staré heslo: %v", err)
	}
	if err := users.CheckPassword(ctx, u.ID, "noveheslo"); err != nil {
		t.Fatalf("nové heslo: %v", err)
	}
}

func TestPasswordLengthInBytes(t *testing.T) {
	ctx := context.Background()
	_, users, u := newSessionTestAuth(t)
	if err := users.ChangePassword(ctx, u.ID, strings.Repeat("ž", 37)); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("74 bajtů: %v", err)
	}
	if _, err := users.Create(ctx, "X", "x@example.com", "", strings.Repeat("a", 73), model.RoleReader); !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("73 bajtů: %v", err)
	}
	if err := users.ChangePassword(ctx, u.ID, strings.Repeat("ž", 36)); err != nil {
		t.Fatalf("72 bajtů: %v", err)
	}
}

// Neznámý účet porovnává heslo s hashem stejné ceny jako skutečná hesla.
func TestLoginUnknownUserUsesDummyHash(t *testing.T) {
	a, _, _ := newSessionTestAuth(t)
	if cost, err := bcrypt.Cost(dummyPasswordHash()); err != nil || cost != bcryptCost {
		t.Fatalf("cena dummy hashe: %d %v", cost, err)
	}
	if _, _, err := a.Login(context.Background(), "neexistuje", "heslo"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("neznámý účet: %v", err)
	}
}

// Do limitu rozpracovaných párování se počítají jen neschválené kódy.
func TestDeviceStartCountsOnlyPending(t *testing.T) {
	ctx := context.Background()
	a, _, u := newSessionTestAuth(t)
	var codes []string
	for range 3 {
		s, err := a.StartDeviceAuth(ctx, "TV")
		if err != nil {
			t.Fatal(err)
		}
		codes = append(codes, s.UserCode)
	}
	if _, err := a.ApproveDevice(ctx, u.ID, codes[0]); err != nil {
		t.Fatal(err)
	}
	pending, err := a.store.PurgeDeviceCodes(ctx, a.now())
	if err != nil || pending != 2 {
		t.Fatalf("neschválených %d, chci 2 (%v)", pending, err)
	}
}
