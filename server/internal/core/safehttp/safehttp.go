// Package safehttp je HTTP klient pro požadavky na adresy, které server
// nezná předem: obrázky ze zdrojů metadat, stránky katalogů, adresy, které
// poslal uživatel. Chrání před SSRF – server se nesmí nechat přimět sáhnout
// do vnitřní sítě, na sebe sama ani na metadata cloudu.
//
// Kontrola běží při navazování spojení (net.Dialer.Control) nad už
// přeloženou IP adresou. Ověření jména předem by obešel DNS rebinding
// i přesměrování; tady se kontroluje skutečný cíl každého spojení, včetně
// spojení po přesměrování. Proto se nemusí hlídat, kam přesměrování vede –
// CDN běžně přesměrovávají na jiné domény (openlibrary.org → archive.org).
package safehttp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// MaxRedirects je nejvyšší počet přesměrování jednoho požadavku.
const MaxRedirects = 5

var (
	// ErrBlocked: cíl spojení je ve vnitřní síti nebo na nepovoleném portu.
	ErrBlocked = errors.New("adresa není veřejná nebo port není 80/443")
	// ErrTooManyRedirects: víc než MaxRedirects přesměrování.
	ErrTooManyRedirects = errors.New("příliš mnoho přesměrování")
	// ErrScheme: přesměrování na jiné schéma než http/https.
	ErrScheme = errors.New("povoleno je jen http a https")
	// ErrTooLarge: odpověď přesáhla strop velikosti.
	ErrTooLarge = errors.New("odpověď je příliš velká")
)

// Option upravuje klienta.
type Option func(*config)

type config struct {
	timeout time.Duration
	allowed map[netip.AddrPort]bool
}

// WithTimeout nastaví celkový strop požadavku (http.Client.Timeout).
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// AllowAddr povolí spojení na přesně tyto adresy „ip:port“, i když jsou
// v místní síti. Slouží testům proti httptest.Server na 127.0.0.1; jiný
// port nebo jiná adresa zůstává zakázaná, takže test ověří i blokování.
func AllowAddr(addrs ...string) Option {
	return func(c *config) {
		for _, a := range addrs {
			if ap, err := netip.ParseAddrPort(a); err == nil {
				c.allowed[ap] = true
			}
		}
	}
}

func newConfig(opts []Option) *config {
	c := &config{timeout: 30 * time.Second, allowed: map[netip.AddrPort]bool{}}
	for _, o := range opts {
		o(c)
	}
	return c
}

// NewClient vrátí HTTP klienta, který se spojí jen s veřejnými adresami na
// portu 80 nebo 443, přesměruje nejvýš MaxRedirects-krát a jen na http(s).
func NewClient(opts ...Option) *http.Client {
	c := newConfig(opts)
	return &http.Client{
		Timeout:       c.timeout,
		Transport:     newTransport(c),
		CheckRedirect: CheckRedirect,
	}
}

// NewTransport vrátí jen transport s kontrolou cíle spojení (pro klienty,
// kteří si http.Client skládají sami).
func NewTransport(opts ...Option) *http.Transport {
	return newTransport(newConfig(opts))
}

func newTransport(c *config) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, _ syscall.RawConn) error {
			return c.check(network, address)
		},
	}
	return &http.Transport{
		// Proxy z prostředí by kontrolu obešla – spojení by šlo na proxy
		// a ta by se připojila kamkoli.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	}
}

// check ověří cíl spojení. address je už přeložená adresa „ip:port“.
func (c *config) check(network, address string) error {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return fmt.Errorf("%w: síť %s", ErrBlocked, network)
	}
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlocked, address)
	}
	ap = netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
	if c.allowed[ap] {
		return nil
	}
	if ap.Port() != 80 && ap.Port() != 443 {
		return fmt.Errorf("%w: %s", ErrBlocked, address)
	}
	if !IsPublic(ap.Addr()) {
		return fmt.Errorf("%w: %s", ErrBlocked, address)
	}
	return nil
}

// Rozsahy, které nejsou veřejný internet a netip je neoznačí sám.
var nonPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // „tato síť“
	netip.MustParsePrefix("100.64.0.0/10"),   // CGNAT (RFC 6598)
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protokolové adresy
	netip.MustParsePrefix("192.0.2.0/24"),    // dokumentace
	netip.MustParsePrefix("198.18.0.0/15"),   // měření výkonu
	netip.MustParsePrefix("198.51.100.0/24"), // dokumentace
	netip.MustParsePrefix("203.0.113.0/24"),  // dokumentace
	netip.MustParsePrefix("240.0.0.0/4"),     // rezervováno, včetně broadcastu
	netip.MustParsePrefix("64:ff9b::/96"),    // NAT64 – vnořená IPv4 může být vnitřní
	netip.MustParsePrefix("64:ff9b:1::/48"),  // místní NAT64
	netip.MustParsePrefix("2001::/32"),       // Teredo
	netip.MustParsePrefix("2001:db8::/32"),   // dokumentace
	netip.MustParsePrefix("2002::/16"),       // 6to4 – vnořená IPv4
	netip.MustParsePrefix("fec0::/10"),       // zastaralé site-local
}

// IsPublic řekne, jestli adresa patří veřejnému internetu: není loopback,
// privátní (včetně IPv6 ULA fc00::/7), link-local, CGNAT, nespecifikovaná,
// multicast ani jinak vyhrazená.
func IsPublic(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsUnspecified() || a.IsLoopback() || a.IsPrivate() ||
		a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() ||
		a.IsMulticast() {
		return false
	}
	for _, p := range nonPublic {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// CheckRedirect omezí přesměrování na MaxRedirects a schémata http/https.
// Cílovou adresu ověří až Dialer – i u přesměrování.
func CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= MaxRedirects {
		return ErrTooManyRedirects
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("%w: %s", ErrScheme, req.URL.Scheme)
	}
	return nil
}

// ReadAll načte nejvýš max bajtů; delší odpověď je ErrTooLarge (cizí
// server nesmí zaplnit paměť ani disk).
func ReadAll(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%w (víc než %d B)", ErrTooLarge, max)
	}
	return data, nil
}

// LimitBody obalí tělo odpovědi tak, že čtení za hranicí max skončí chybou
// ErrTooLarge místo tichého useknutí (useknutý JSON by vypadal jako jiná
// chyba).
func LimitBody(body io.ReadCloser, max int64) io.ReadCloser {
	return &limitedBody{rc: body, left: max}
}

type limitedBody struct {
	rc   io.ReadCloser
	left int64
}

func (l *limitedBody) Read(p []byte) (int, error) {
	if l.left < 0 {
		return 0, ErrTooLarge
	}
	if int64(len(p)) > l.left+1 {
		p = p[:l.left+1]
	}
	n, err := l.rc.Read(p)
	l.left -= int64(n)
	if l.left < 0 {
		return n + int(l.left), ErrTooLarge
	}
	return n, err
}

func (l *limitedBody) Close() error { return l.rc.Close() }

// HostMatches ověří, že URL má schéma http(s), standardní port (žádný,
// nebo 80 u http a 443 u https) a hostitele host, volitelně s předponou
// "www.". Zdroje metadat jím popisují, odkud smí server stahovat; port
// navíc by z povolené domény udělal přístup k jiné službě na stejném stroji.
func HostMatches(rawURL, host string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || host == "" {
		return false
	}
	switch port := u.Port(); u.Scheme {
	case "http":
		if port != "" && port != "80" {
			return false
		}
	case "https":
		if port != "" && port != "443" {
			return false
		}
	default:
		return false
	}
	return strings.EqualFold(u.Hostname(), host) ||
		strings.EqualFold(u.Hostname(), "www."+host)
}

// IsBlocked řekne, jestli chyba vznikla zákazem cíle spojení.
func IsBlocked(err error) bool { return errors.Is(err, ErrBlocked) }
