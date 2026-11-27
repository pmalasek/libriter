package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

const (
	ctxKeyClientIP     contextKey = "client_ip"
	ctxKeyTrustedProxy contextKey = "trusted_proxy"
)

// ClientIP zjistí skutečnou adresu klienta a uloží ji do kontextu (viz
// ClientIPFromCtx). Podle ní se počítají limity přihlášení.
//
// Hlavičkám X-Real-IP a X-Forwarded-For se věří jen tehdy, když spojení
// přišlo z adresy v trusted (TRUSTED_PROXIES) – jinak by si útočník každý
// pokus o heslo poslal s jinou smyšlenou adresou. Pořadí: X-Real-IP, potom
// nejpravější adresa z X-Forwarded-For, která sama není důvěryhodná proxy.
// True-Client-IP se ignoruje vždy.
//
// r.RemoteAddr zůstává beze změny (skutečný protějšek spojení).
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, viaProxy := resolveClientIP(r, trusted)
			ctx := context.WithValue(r.Context(), ctxKeyClientIP, ip)
			ctx = context.WithValue(ctx, ctxKeyTrustedProxy, viaProxy)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClientIPFromCtx vrátí adresu klienta zjištěnou middlewarem ClientIP.
// Mimo něj (testy handlerů) vrátí neplatnou adresu.
func ClientIPFromCtx(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(ctxKeyClientIP).(netip.Addr)
	return ip
}

// ViaTrustedProxy říká, jestli požadavek přišel přes důvěryhodnou proxy
// (TRUSTED_PROXIES). Jen tehdy se dá věřit jejím hlavičkám, např.
// X-Forwarded-Proto.
func ViaTrustedProxy(ctx context.Context) bool {
	v, _ := ctx.Value(ctxKeyTrustedProxy).(bool)
	return v
}

func resolveClientIP(r *http.Request, trusted []netip.Prefix) (netip.Addr, bool) {
	peer := parseIP(r.RemoteAddr)
	if !peer.IsValid() || !isTrusted(peer, trusted) {
		return peer, false
	}

	if ip := parseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip.IsValid() {
		return ip, true
	}

	// X-Forwarded-For: klient, proxy1, proxy2 – každá proxy připisuje
	// doprava. Věrohodná je jen pravá část, kterou připsaly naše proxy;
	// první nedůvěryhodná adresa zprava je klient.
	var hops []string
	for _, h := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(h, ",")...)
	}
	var leftmost netip.Addr
	for i := len(hops) - 1; i >= 0; i-- {
		ip := parseIP(strings.TrimSpace(hops[i]))
		if !ip.IsValid() {
			// Nesmysl v řetězci – dál doleva se věřit nedá.
			break
		}
		leftmost = ip
		if !isTrusted(ip, trusted) {
			return ip, true
		}
	}
	if leftmost.IsValid() {
		return leftmost, true
	}
	return peer, true
}

// parseIP přečte adresu ve tvaru „ip“ nebo „ip:port“ (i „[ipv6]:port“).
// IPv4 zabalená v IPv6 se rozbalí, ať se shoduje s IPv4 rozsahy.
func parseIP(s string) netip.Addr {
	if s == "" {
		return netip.Addr{}
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	ip, err := netip.ParseAddr(strings.Trim(s, "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap().WithZone("")
}

func isTrusted(ip netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}
