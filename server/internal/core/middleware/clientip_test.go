package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"mediater/internal/core/service"
)

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("10.0.0.0/8"),
	}
	cases := []struct {
		name    string
		remote  string
		headers map[string]string
		want    string
		proxied bool
	}{
		{"přímé spojení", "203.0.113.5:4000", nil, "203.0.113.5", false},
		{"podvržené hlavičky od nedůvěryhodné adresy", "203.0.113.5:4000",
			map[string]string{"X-Real-IP": "1.2.3.4", "X-Forwarded-For": "5.6.7.8", "True-Client-IP": "9.9.9.9"}, "203.0.113.5", false},
		{"X-Real-IP od proxy", "127.0.0.1:5000", map[string]string{"X-Real-IP": "198.51.100.7"}, "198.51.100.7", true},
		{"X-Real-IP má přednost", "127.0.0.1:5000",
			map[string]string{"X-Real-IP": "198.51.100.7", "X-Forwarded-For": "198.51.100.8"}, "198.51.100.7", true},
		{"nejpravější nedůvěryhodná z XFF", "127.0.0.1:5000",
			map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.9, 10.1.2.3"}, "198.51.100.9", true},
		{"neplatný XFF", "127.0.0.1:5000", map[string]string{"X-Forwarded-For": "nesmysl"}, "127.0.0.1", true},
		{"True-Client-IP se ignoruje i od proxy", "127.0.0.1:5000", map[string]string{"True-Client-IP": "9.9.9.9"}, "127.0.0.1", true},
		{"IPv6 proxy", "[::1]:5000", map[string]string{"X-Real-IP": "2001:db8::1"}, "2001:db8::1", true},
		{"IPv4 v IPv6", "[::ffff:127.0.0.1]:5000", map[string]string{"X-Real-IP": "198.51.100.7"}, "198.51.100.7", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got netip.Addr
			var proxied bool
			h := ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = ClientIPFromCtx(r.Context())
				proxied = ViaTrustedProxy(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got.String() != tc.want || proxied != tc.proxied {
				t.Errorf("adresa %s (proxy %v), chci %s (proxy %v)", got, proxied, tc.want, tc.proxied)
			}
		})
	}
}

func TestClientIPKeyGroupsIPv6Network(t *testing.T) {
	key := func(remote string) string {
		var k string
		h := ClientIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			k, _ = clientIPKey(r)
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = remote
		h.ServeHTTP(httptest.NewRecorder(), req)
		return k
	}
	if a, b := key("[2001:db8:1:2::1]:1"), key("[2001:db8:1:2:ffff::9]:1"); a != b {
		t.Errorf("adresy z jedné /64 mají různé klíče: %s, %s", a, b)
	}
	if a, b := key("203.0.113.1:1"), key("203.0.113.2:1"); a == b {
		t.Errorf("různé IPv4 mají stejný klíč %s", a)
	}
}

func TestRequireWebSession(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	for scope, want := range map[string]int{
		"":                  http.StatusOK,
		service.ScopeMobile: http.StatusForbidden,
		service.ScopeTV:     http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(contextWithScope(req.Context(), scope))
		rec := httptest.NewRecorder()
		RequireWebSession(ok).ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("scope %q: %d, chci %d", scope, rec.Code, want)
		}
	}
}

func contextWithScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, ctxKeyScope, scope)
}
