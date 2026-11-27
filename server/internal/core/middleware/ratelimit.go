package middleware

import (
	"net"
	"net/http"
	"time"

	"github.com/go-chi/httprate"

	"mediater/internal/core/httpx"
)

// RateLimit omezí počet požadavků z jedné adresy klienta (ClientIP) na limit
// za okno. Nad limitem odpoví 429 auth.rate_limited s hlavičkou Retry-After.
//
// Každé volání vytvoří nový limiter s vlastním počítadlem – router si je
// skládá při sestavení, takže testy (každý se svým routerem) stav nesdílejí.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.NewRateLimiter(limit, window,
		httprate.WithKeyFuncs(clientIPKey),
		httprate.WithLimitHandler(WriteRateLimited),
	).Handler
}

// WriteRateLimited odpoví 429 auth.rate_limited. Hlavičku Retry-After
// nastaví volající (httprate ji doplní sám).
func WriteRateLimited(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteError(w, http.StatusTooManyRequests, "auth.rate_limited", "příliš mnoho pokusů, zkuste to později")
}

// clientIPKey je klíč limitu: adresa klienta, u IPv6 celá síť /64 – tu má
// obvykle k dispozici jeden připojený uživatel a mohl by si adresy střídat.
func clientIPKey(r *http.Request) (string, error) {
	ip := ClientIPFromCtx(r.Context())
	if !ip.IsValid() {
		ip = parseIP(r.RemoteAddr)
	}
	if !ip.IsValid() {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr, nil
		}
		return host, nil
	}
	if ip.Is6() {
		if p, err := ip.Prefix(64); err == nil {
			return p.String(), nil
		}
	}
	return ip.String(), nil
}
