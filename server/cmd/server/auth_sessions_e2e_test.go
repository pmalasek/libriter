package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/google/uuid"

	"mediater/internal/core/config"
	"mediater/internal/core/product"
	"mediater/internal/core/service"
)

// authEnv je router s účty pro testy tokenů, relací zařízení a limitů.
type authEnv struct {
	t      *testing.T
	router http.Handler
	deps   product.Deps
}

func newAuthEnv(t *testing.T, configure func(*config.Config)) *authEnv {
	t.Helper()
	r, deps := testRouterWith(t, configure)
	return &authEnv{t: t, router: r, deps: deps}
}

// req pošle požadavek; headers jsou dvojice název, hodnota.
func (e *authEnv) req(method, path, token string, body any, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// expect ověří stav a případně kód chyby.
func (e *authEnv) expect(rec *httptest.ResponseRecorder, status int, code string, what string) {
	e.t.Helper()
	if rec.Code != status {
		e.t.Fatalf("%s: stav %d, chci %d (%s)", what, rec.Code, status, rec.Body.String())
	}
	if code == "" {
		return
	}
	var body struct{ Code string }
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Code != code {
		e.t.Fatalf("%s: kód %q, chci %q (%s)", what, body.Code, code, rec.Body.String())
	}
}

func (e *authEnv) user(email, role string) string {
	e.t.Helper()
	u, err := e.deps.Users.Create(context.Background(), "Test "+role, email, "", "tajneheslo", role)
	if err != nil {
		e.t.Fatal(err)
	}
	return u.ID.String()
}

func (e *authEnv) login(login, password string) string {
	e.t.Helper()
	rec := e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": login, "password": password})
	var out struct{ Token string }
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Token == "" {
		e.t.Fatalf("login %s: %d %s", login, rec.Code, rec.Body.String())
	}
	return out.Token
}

func (e *authEnv) token(rec *httptest.ResponseRecorder, what string) string {
	e.t.Helper()
	var out struct{ Token string }
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Token == "" {
		e.t.Fatalf("%s: %d %s", what, rec.Code, rec.Body.String())
	}
	return out.Token
}

func (e *authEnv) mobileToken(web, deviceName string) string {
	e.t.Helper()
	return e.token(e.req("POST", "/api/v1/auth/mobile-token", web, nil, "X-Device-Name", deviceName), "mobile-token")
}

// tvToken spáruje televizi a schválí ji webovým tokenem.
func (e *authEnv) tvToken(web, deviceName string) string {
	e.t.Helper()
	var start struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	rec := e.req("POST", "/api/v1/auth/device/start", "", map[string]string{"device_name": deviceName})
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &start) != nil {
		e.t.Fatalf("device start: %d %s", rec.Code, rec.Body.String())
	}
	e.expect(e.req("POST", "/api/v1/auth/device/approve", web, map[string]string{"user_code": start.UserCode}), http.StatusOK, "", "schválení")
	return e.token(e.req("POST", "/api/v1/auth/device/token", "", map[string]string{"device_code": start.DeviceCode}), "device token")
}

func (e *authEnv) streamToken(tok string) string {
	e.t.Helper()
	return e.token(e.req("GET", "/api/v1/auth/stream-token", tok, nil), "stream-token")
}

// audio otevře audio neexistující kapitoly. Token se ověřuje dřív než
// kapitola: platný token skončí 404, neplatný 401.
func (e *authEnv) audio(stream string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.req("GET", "/api/v1/chapters/"+uuid.NewString()+"/audio?t="+stream, "", nil)
}

type deviceRow struct {
	ID         string `json:"id"`
	Scope      string `json:"scope"`
	DeviceName string `json:"device_name"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
}

func (e *authEnv) devices(web, userID string) []deviceRow {
	e.t.Helper()
	rec := e.req("GET", "/api/v1/users/"+userID+"/devices", web, nil)
	var out []deviceRow
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		e.t.Fatalf("devices: %d %s", rec.Code, rec.Body.String())
	}
	return out
}

// Token mobilu i televize admina pouští jen ke čtení: administrace, změna
// hesla a schválení párování z něj nejdou, čtenářské trasy ano.
func TestDeviceTokensAreReaderOnly(t *testing.T) {
	e := newAuthEnv(t, nil)
	adminID := e.user("admin@example.com", "admin")
	web := e.login("admin@example.com", "tajneheslo")

	// Webová relace admina administraci smí.
	e.expect(e.req("GET", "/api/v1/users", web, nil), http.StatusOK, "", "admin web")

	devices := map[string]string{
		"mobil":    e.mobileToken(web, "Pixel"),
		"televize": e.tvToken(web, "Obývák"),
	}
	// Další kód, který by zařízení chtělo schválit.
	var start struct {
		UserCode string `json:"user_code"`
	}
	rec := e.req("POST", "/api/v1/auth/device/start", "", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &start)

	for name, tok := range devices {
		e.expect(e.req("GET", "/api/v1/users", tok, nil), http.StatusForbidden, "auth.forbidden", name+": admin trasa")
		e.expect(e.req("POST", "/api/v1/admin/users", tok, map[string]string{}), http.StatusForbidden, "auth.forbidden", name+": založení účtu")
		e.expect(e.req("PUT", "/api/v1/users/"+adminID+"/password", tok,
			map[string]string{"password": "noveheslo123", "current_password": "tajneheslo"}),
			http.StatusForbidden, "auth.web_session_required", name+": změna hesla")
		e.expect(e.req("PUT", "/api/v1/users/"+adminID, tok,
			map[string]string{"display_name": "X", "email": "admin@example.com"}),
			http.StatusForbidden, "auth.web_session_required", name+": úprava profilu")
		e.expect(e.req("GET", "/api/v1/auth/device/"+start.UserCode, tok, nil),
			http.StatusForbidden, "auth.web_session_required", name+": info o párování")
		e.expect(e.req("POST", "/api/v1/auth/device/approve", tok, map[string]string{"user_code": start.UserCode}),
			http.StatusForbidden, "auth.web_session_required", name+": schválení párování")
		e.expect(e.req("POST", "/api/v1/users/"+adminID+"/logout-all", tok, nil),
			http.StatusForbidden, "auth.web_session_required", name+": odhlásit všude")
		e.expect(e.req("GET", "/api/v1/users/"+adminID+"/devices", tok, nil),
			http.StatusForbidden, "auth.web_session_required", name+": seznam zařízení")
		e.expect(e.req("POST", "/api/v1/auth/mobile-token", tok, nil),
			http.StatusForbidden, "auth.mobile_token_refresh", name+": další mobilní token")

		// Čtení, profil a jazyk zůstávají.
		e.expect(e.req("GET", "/api/v1/sessions", tok, nil), http.StatusOK, "", name+": čtenářská trasa")
		e.expect(e.req("GET", "/api/v1/users/"+adminID, tok, nil), http.StatusOK, "", name+": profil")
		e.expect(e.req("PUT", "/api/v1/users/"+adminID+"/language", tok, map[string]string{"ui_language": "cs"}),
			http.StatusOK, "", name+": jazyk")
		e.expect(e.req("PUT", "/api/v1/users/"+adminID+"/appearance", tok,
			map[string]string{"color_scheme": "emerald", "theme_mode": "dark"}), http.StatusOK, "", name+": vzhled")
		e.streamToken(tok)
	}
}

// Změna hesla vyžaduje současné heslo a odhlásí všechny dosavadní tokeny;
// web dostane nový token. Reset adminem současné heslo nepotřebuje.
func TestPasswordChangeRevokesTokens(t *testing.T) {
	e := newAuthEnv(t, nil)
	userID := e.user("ctenar@example.com", "reader")
	e.user("admin@example.com", "admin")
	admin := e.login("admin@example.com", "tajneheslo")
	web := e.login("ctenar@example.com", "tajneheslo")
	mobile := e.mobileToken(web, "Telefon")
	stream := e.streamToken(mobile)
	path := "/api/v1/users/" + userID + "/password"

	e.expect(e.req("PUT", path, web, map[string]string{"password": "noveheslo123"}),
		http.StatusBadRequest, "auth.current_password_required", "bez současného hesla")
	e.expect(e.req("PUT", path, web, map[string]string{"password": "noveheslo123", "current_password": "spatne"}),
		http.StatusForbidden, "auth.current_password_invalid", "špatné současné heslo")
	e.expect(e.req("GET", "/api/v1/users/"+userID, web, nil), http.StatusOK, "", "token po neúspěšné změně")

	newWeb := e.token(e.req("PUT", path, web,
		map[string]string{"password": "noveheslo123", "current_password": "tajneheslo"}), "změna hesla")

	e.expect(e.req("GET", "/api/v1/users/"+userID, web, nil), http.StatusUnauthorized, "auth.session_revoked", "starý webový token")
	e.expect(e.req("GET", "/api/v1/users/"+userID, mobile, nil), http.StatusUnauthorized, "auth.session_revoked", "mobilní token")
	e.expect(e.audio(stream), http.StatusUnauthorized, "auth.session_revoked", "streamovací token")
	e.expect(e.req("GET", "/api/v1/users/"+userID, newWeb, nil), http.StatusOK, "", "nový token")
	if got := e.devices(newWeb, userID); len(got) != 0 {
		t.Fatalf("zařízení po změně hesla: %+v", got)
	}
	e.login("ctenar@example.com", "noveheslo123")

	// Admin resetuje cizí heslo bez současného hesla; odpověď token nenese.
	rec := e.req("PUT", path, admin, map[string]string{"password": "resetovane1"})
	e.expect(rec, http.StatusOK, "", "reset adminem")
	if strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("reset adminem vrátil token: %s", rec.Body.String())
	}
	e.expect(e.req("GET", "/api/v1/users/"+userID, newWeb, nil), http.StatusUnauthorized, "auth.session_revoked", "token po resetu")
	e.expect(e.req("GET", "/api/v1/users", admin, nil), http.StatusOK, "", "admin zůstává přihlášený")

	// Vlastní změna e-mailu chce heslo, změna jména ne.
	fresh := e.login("ctenar@example.com", "resetovane1")
	e.expect(e.req("PUT", "/api/v1/users/"+userID, fresh,
		map[string]string{"display_name": "Nové jméno", "email": "ctenar@example.com"}), http.StatusOK, "", "jen jméno")
	e.expect(e.req("PUT", "/api/v1/users/"+userID, fresh,
		map[string]string{"display_name": "Nové jméno", "email": "jiny@example.com"}),
		http.StatusBadRequest, "auth.current_password_required", "e-mail bez hesla")
	e.expect(e.req("PUT", "/api/v1/users/"+userID, fresh,
		map[string]string{"display_name": "Nové jméno", "email": "ctenar@example.com", "login": "ctenar"}),
		http.StatusBadRequest, "auth.current_password_required", "login bez hesla")
	e.expect(e.req("PUT", "/api/v1/users/"+userID, fresh,
		map[string]string{"display_name": "Nové jméno", "email": "jiny@example.com", "current_password": "resetovane1"}),
		http.StatusOK, "", "e-mail s heslem")
	// Admin mění cizí e-mail bez hesla.
	e.expect(e.req("PUT", "/api/v1/users/"+userID, admin,
		map[string]string{"display_name": "Nové jméno", "email": "treti@example.com"}), http.StatusOK, "", "e-mail adminem")

	rec = e.req("GET", "/api/v1/admin/audit", admin, nil)
	for _, action := range []string{service.AuditUserPasswordChange, service.AuditUserPasswordReset} {
		if !strings.Contains(rec.Body.String(), `"`+action+`"`) {
			t.Errorf("audit neobsahuje %s: %s", action, rec.Body.String())
		}
	}
}

// Odvolané zařízení ztratí přístup i se svými streamovacími tokeny, ostatní
// zařízení a web jedou dál.
func TestRevokeDevice(t *testing.T) {
	e := newAuthEnv(t, nil)
	userID := e.user("ctenar@example.com", "reader")
	otherID := e.user("jiny@example.com", "reader")
	web := e.login("ctenar@example.com", "tajneheslo")
	other := e.login("jiny@example.com", "tajneheslo")

	m1 := e.mobileToken(web, "Petr%C5%AFv%20telefon")
	m2 := e.mobileToken(web, "Tablet")
	tv := e.tvToken(web, "Obývák")
	s1 := e.streamToken(m1)
	s2 := e.streamToken(m2)
	sWeb := e.streamToken(web)

	list := e.devices(web, userID)
	if len(list) != 3 {
		t.Fatalf("zařízení: %+v", list)
	}
	byName := map[string]deviceRow{}
	for _, d := range list {
		byName[d.DeviceName] = d
	}
	if byName["Petrův telefon"].Scope != "mobile" || byName["Obývák"].Scope != "tv" || byName["Tablet"].ID == "" {
		t.Fatalf("názvy a typy zařízení: %+v", list)
	}
	if byName["Tablet"].CreatedAt == "" || byName["Tablet"].LastSeenAt == "" {
		t.Fatalf("časy zařízení: %+v", byName["Tablet"])
	}
	sid := byName["Petrův telefon"].ID

	// Cizí účet ani token zařízení odvolávat nesmí.
	e.expect(e.req("DELETE", "/api/v1/users/"+userID+"/devices/"+sid, other, nil), http.StatusForbidden, "auth.forbidden", "cizí účet")
	e.expect(e.req("DELETE", "/api/v1/users/"+otherID+"/devices/"+sid, other, nil), http.StatusNotFound, "user.device_not_found", "cizí relace")
	e.expect(e.req("DELETE", "/api/v1/users/"+userID+"/devices/"+sid, m2, nil), http.StatusForbidden, "auth.web_session_required", "tokenem zařízení")

	e.expect(e.req("DELETE", "/api/v1/users/"+userID+"/devices/"+sid, web, nil), http.StatusNoContent, "", "odvolání")
	e.expect(e.req("DELETE", "/api/v1/users/"+userID+"/devices/"+sid, web, nil), http.StatusNotFound, "user.device_not_found", "druhé odvolání")

	e.expect(e.req("GET", "/api/v1/sessions", m1, nil), http.StatusUnauthorized, "auth.session_revoked", "odvolaný mobil")
	e.expect(e.audio(s1), http.StatusUnauthorized, "auth.session_revoked", "stream odvolaného mobilu")
	e.expect(e.req("GET", "/api/v1/sessions", m2, nil), http.StatusOK, "", "druhý mobil")
	e.expect(e.req("GET", "/api/v1/sessions", tv, nil), http.StatusOK, "", "televize")
	e.expect(e.audio(s2), http.StatusNotFound, "", "stream druhého mobilu")
	e.expect(e.audio(sWeb), http.StatusNotFound, "", "stream webu")
	e.expect(e.req("GET", "/api/v1/sessions", web, nil), http.StatusOK, "", "web")

	if got := e.devices(web, userID); len(got) != 2 {
		t.Fatalf("zařízení po odvolání: %+v", got)
	}

	// Admin vidí a odvolává zařízení cizího účtu.
	e.user("admin@example.com", "admin")
	admin := e.login("admin@example.com", "tajneheslo")
	e.expect(e.req("DELETE", "/api/v1/users/"+userID+"/devices/"+byName["Obývák"].ID, admin, nil), http.StatusNoContent, "", "odvolání adminem")
	e.expect(e.req("GET", "/api/v1/sessions", tv, nil), http.StatusUnauthorized, "auth.session_revoked", "odvolaná televize")

	rec := e.req("GET", "/api/v1/admin/audit", admin, nil)
	if n := strings.Count(rec.Body.String(), `"`+service.AuditUserDeviceRevoke+`"`); n != 2 {
		t.Errorf("audit odvolání: %d záznamů, chci 2", n)
	}
}

// „Odhlásit všude“ zneplatní všechny tokeny včetně volajícího; ten dostane nový.
func TestLogoutAll(t *testing.T) {
	e := newAuthEnv(t, nil)
	userID := e.user("ctenar@example.com", "reader")
	web := e.login("ctenar@example.com", "tajneheslo")
	web2 := e.login("ctenar@example.com", "tajneheslo")
	mobile := e.mobileToken(web, "Telefon")
	tv := e.tvToken(web, "TV")
	stream := e.streamToken(web)

	fresh := e.token(e.req("POST", "/api/v1/users/"+userID+"/logout-all", web, nil), "logout-all")

	for name, tok := range map[string]string{"web": web, "druhý web": web2, "mobil": mobile, "tv": tv} {
		e.expect(e.req("GET", "/api/v1/users/"+userID, tok, nil), http.StatusUnauthorized, "auth.session_revoked", name)
	}
	e.expect(e.audio(stream), http.StatusUnauthorized, "auth.session_revoked", "stream")
	e.expect(e.req("GET", "/api/v1/users/"+userID, fresh, nil), http.StatusOK, "", "nový token")
	if got := e.devices(fresh, userID); len(got) != 0 {
		t.Fatalf("zařízení po odhlášení všude: %+v", got)
	}

	// Admin odhlásí cizí účet; sám token nedostane a zůstane přihlášený.
	e.user("admin@example.com", "admin")
	admin := e.login("admin@example.com", "tajneheslo")
	rec := e.req("POST", "/api/v1/users/"+userID+"/logout-all", admin, nil)
	e.expect(rec, http.StatusOK, "", "logout-all adminem")
	if strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("logout-all cizího účtu vrátil token: %s", rec.Body.String())
	}
	e.expect(e.req("GET", "/api/v1/users/"+userID, fresh, nil), http.StatusUnauthorized, "auth.session_revoked", "po odhlášení adminem")
	e.expect(e.req("GET", "/api/v1/users", admin, nil), http.StatusOK, "", "admin")

	rec = e.req("GET", "/api/v1/admin/audit", admin, nil)
	if n := strings.Count(rec.Body.String(), `"`+service.AuditUserLogoutAll+`"`); n != 2 {
		t.Errorf("audit odhlášení všude: %d záznamů, chci 2", n)
	}
}

// Jedenáctý pokus o přihlášení z jedné adresy za minutu dostane 429.
func TestLoginRateLimitPerIP(t *testing.T) {
	e := newAuthEnv(t, nil)
	for i := range 10 {
		e.expect(e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": fmt.Sprintf("nikdo%d", i), "password": "x"}),
			http.StatusUnauthorized, "auth.invalid_credentials", fmt.Sprintf("pokus %d", i+1))
	}
	rec := e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "nikdo", "password": "x"})
	e.expect(rec, http.StatusTooManyRequests, "auth.rate_limited", "11. pokus")
	if rec.Header().Get("Retry-After") == "" {
		t.Error("chybí Retry-After")
	}
}

// Podvržené X-Real-IP / X-Forwarded-For od nedůvěryhodné adresy limit neobejde.
func TestSpoofedClientIPIgnored(t *testing.T) {
	e := newAuthEnv(t, nil)
	for i := range 10 {
		e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "nikdo", "password": "x"},
			"X-Real-IP", fmt.Sprintf("203.0.113.%d", i+1), "X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1),
			"True-Client-IP", fmt.Sprintf("192.0.2.%d", i+100))
	}
	e.expect(e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "nikdo", "password": "x"},
		"X-Real-IP", "203.0.113.200"), http.StatusTooManyRequests, "auth.rate_limited", "podvržená adresa")
}

// Za důvěryhodnou proxy se adresy rozliší; neúspěchy k jednomu loginu se ale
// sčítají napříč adresami a jedenáctý pokus (i se správným heslem) neprojde.
func TestLoginFailureLimitPerLogin(t *testing.T) {
	e := newAuthEnv(t, func(c *config.Config) {
		// httptest posílá z 192.0.2.1.
		c.Server.TrustedProxies = []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	})
	e.user("cil@example.com", "reader")
	for i := range 10 {
		e.expect(e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "CIL@example.com ", "password": "spatne"},
			"X-Real-IP", fmt.Sprintf("203.0.113.%d", i+1)), http.StatusUnauthorized, "auth.invalid_credentials", fmt.Sprintf("pokus %d", i+1))
	}
	rec := e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "cil@example.com", "password": "tajneheslo"},
		"X-Real-IP", "203.0.113.99")
	e.expect(rec, http.StatusTooManyRequests, "auth.rate_limited", "11. pokus o login")
	if rec.Header().Get("Retry-After") == "" {
		t.Error("chybí Retry-After")
	}
	// Jiný login ze stejné (nové) adresy projde – adresy se díky proxy liší.
	e.expect(e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "jiny@example.com", "password": "x"},
		"X-Real-IP", "203.0.113.100"), http.StatusUnauthorized, "auth.invalid_credentials", "jiný login")
}

// Heslo delší než 72 bajtů (bcrypt) je 400, ne 500 – měří se bajty, ne znaky.
func TestPasswordTooLong(t *testing.T) {
	e := newAuthEnv(t, nil)
	if _, err := e.deps.Registration.SetRegistration(context.Background(),
		service.RegistrationSettings{Enabled: true, DefaultRole: "reader"}); err != nil {
		t.Fatal(err)
	}
	userID := e.user("admin@example.com", "admin")
	admin := e.login("admin@example.com", "tajneheslo")
	long := strings.Repeat("a", 73)
	// 37 znaků, 74 bajtů.
	wide := strings.Repeat("ž", 37)

	for _, pw := range []string{long, wide} {
		e.expect(e.req("POST", "/api/v1/auth/register", "", map[string]string{
			"display_name": "Nový", "email": "novy@example.com", "password": pw,
		}), http.StatusBadRequest, "auth.password_too_long", "registrace")
		e.expect(e.req("POST", "/api/v1/admin/users", admin, map[string]string{
			"display_name": "Nový", "email": "novy@example.com", "password": pw, "role": "reader",
		}), http.StatusBadRequest, "auth.password_too_long", "založení adminem")
		e.expect(e.req("PUT", "/api/v1/users/"+userID+"/password", admin, map[string]string{
			"password": pw, "current_password": "tajneheslo",
		}), http.StatusBadRequest, "auth.password_too_long", "změna hesla")
	}
	// Přesně 72 bajtů projde.
	e.expect(e.req("PUT", "/api/v1/users/"+userID+"/password", admin, map[string]string{
		"password": strings.Repeat("a", 72), "current_password": "tajneheslo",
	}), http.StatusOK, "", "72 bajtů")
}

// Neznámý účet i špatné heslo vrací totéž (a neznámý účet také počítá bcrypt).
func TestLoginUnknownUser(t *testing.T) {
	e := newAuthEnv(t, nil)
	e.user("ctenar@example.com", "reader")
	e.expect(e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "neexistuje", "password": "tajneheslo"}),
		http.StatusUnauthorized, "auth.invalid_credentials", "neznámý účet")
	e.expect(e.req("POST", "/api/v1/auth/login", "", map[string]string{"login": "ctenar@example.com", "password": "spatne"}),
		http.StatusUnauthorized, "auth.invalid_credentials", "špatné heslo")
}
