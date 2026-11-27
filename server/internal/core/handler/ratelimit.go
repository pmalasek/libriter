package handler

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"mediater/internal/core/middleware"
)

// RateLimits jsou limity veřejných přihlašovacích endpointů. Nil položka
// znamená bez limitu. Sestavují se pro každý router zvlášť (DefaultRateLimits),
// ať testy se svými routery nesdílejí počítadla.
type RateLimits struct {
	Login       func(http.Handler) http.Handler
	Register    func(http.Handler) http.Handler
	DeviceStart func(http.Handler) http.Handler
	DeviceToken func(http.Handler) http.Handler
	// LoginFailures je nejvyšší počet neúspěšných přihlášení k jednomu
	// loginu za LoginFailureWindow; 0 = bez limitu.
	LoginFailures      int
	LoginFailureWindow time.Duration
}

// DefaultRateLimits vrátí produkční limity s novými počítadly.
func DefaultRateLimits() RateLimits {
	return RateLimits{
		Login:    middleware.RateLimit(10, time.Minute),
		Register: middleware.RateLimit(5, time.Hour),
		// Párování: kód platí 10 minut, víc než pár televizí naráz se nepáruje.
		DeviceStart: middleware.RateLimit(10, 10*time.Minute),
		// TV se ptá každých 5 s (12/min); rezerva na víc televizí za jednou NAT.
		DeviceToken:        middleware.RateLimit(60, time.Minute),
		LoginFailures:      10,
		LoginFailureWindow: 10 * time.Minute,
	}
}

// limited obalí handler limitem, pokud je nastavený.
func limited(limit func(http.Handler) http.Handler, h http.HandlerFunc) http.Handler {
	if limit == nil {
		return h
	}
	return limit(h)
}

// loginFailureCap je strop počtu sledovaných loginů. Při jeho dosažení se
// zahodí prošlé záznamy; když ani to nestačí, nové loginy se nesledují
// (zbývá limit na IP adresu) – paměť serveru nesmí jít zaplnit.
const loginFailureCap = 10_000

// loginFailures počítá neúspěšná přihlášení k jednotlivým loginům. Limit na
// IP adresu nestačí: útočník s mnoha adresami by jinak zkoušel hesla jednoho
// účtu bez omezení. Počítadlo je jen v paměti – restart ho vynuluje.
type loginFailures struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]loginFailure
}

type loginFailure struct {
	count int
	start time.Time
}

func newLoginFailures(limit int, window time.Duration) *loginFailures {
	return &loginFailures{limit: limit, window: window, now: time.Now, entries: map[string]loginFailure{}}
}

// loginKey normalizuje zadaný login (login i e-mail se porovnávají bez
// ohledu na velikost písmen).
func loginKey(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}

// blocked vrátí true a zbývající dobu, pokud login vyčerpal pokusy.
func (f *loginFailures) blocked(login string) (time.Duration, bool) {
	if f == nil || f.limit <= 0 {
		return 0, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[loginKey(login)]
	if !ok {
		return 0, false
	}
	left := e.start.Add(f.window).Sub(f.now())
	if left <= 0 {
		delete(f.entries, loginKey(login))
		return 0, false
	}
	return left, e.count >= f.limit
}

// fail započítá neúspěšný pokus.
func (f *loginFailures) fail(login string) {
	if f == nil || f.limit <= 0 {
		return
	}
	key := loginKey(login)
	now := f.now()
	f.mu.Lock()
	defer f.mu.Unlock()

	e, ok := f.entries[key]
	if ok && now.Sub(e.start) >= f.window {
		ok = false
	}
	if !ok {
		if len(f.entries) >= loginFailureCap {
			for k, v := range f.entries {
				if now.Sub(v.start) >= f.window {
					delete(f.entries, k)
				}
			}
			if len(f.entries) >= loginFailureCap {
				return
			}
		}
		e = loginFailure{start: now}
	}
	e.count++
	f.entries[key] = e
}

// reset smaže počítadlo po úspěšném přihlášení.
func (f *loginFailures) reset(login string) {
	if f == nil || f.limit <= 0 {
		return
	}
	f.mu.Lock()
	delete(f.entries, loginKey(login))
	f.mu.Unlock()
}

// writeRetryAfter odpoví 429 s Retry-After zaokrouhleným nahoru na sekundy.
func writeRetryAfter(w http.ResponseWriter, r *http.Request, wait time.Duration) {
	secs := int((wait + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	middleware.WriteRateLimited(w, r)
}
