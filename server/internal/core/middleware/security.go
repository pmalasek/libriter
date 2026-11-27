package middleware

import (
	"net/http"
	"strings"
)

// APIContentSecurityPolicy platí pro odpovědi API. JSON, audio ani obrázky
// z API se nikdy nezobrazují jako stránka, takže nesmí načítat nic.
const APIContentSecurityPolicy = "default-src 'none'; frame-ancestors 'none'"

// WebContentSecurityPolicy platí pro webová rozhraní (SPA) a /health.
//   - style-src 'unsafe-inline': sonner, react-remove-scroll (Radix) a
//     next-themes vkládají značky <style>; stránka „frontend není sestaven“
//     má inline atribut style.
//   - blob: u médií a workerů: hls.js skládá video přes MediaSource a
//     spouští worker z blob: adresy.
//   - blob: u obrázků: náhled obálky při importu (URL.createObjectURL).
//   - https: u obrázků: výsledky hledání metadat ukazují vzdálené obálky přímo.
const WebContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: https:; media-src 'self' blob:; worker-src 'self' blob:; font-src 'self' data:; " +
	"connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// SecurityHeaders nastaví bezpečnostní hlavičky na každou odpověď: zákaz
// odhadování typu obsahu, zákaz vložení do rámce, omezení Refereru a CSP
// podle toho, jestli jde o API, nebo o webové rozhraní.
//
// HSTS se tu záměrně nenastavuje – server sám HTTPS neobsluhuje a hlavičku
// přidává reverzní proxy (nginx), která ví, že spojení je šifrované.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Content-Security-Policy", APIContentSecurityPolicy)
		} else {
			h.Set("Content-Security-Policy", WebContentSecurityPolicy)
		}
		next.ServeHTTP(w, r)
	})
}
