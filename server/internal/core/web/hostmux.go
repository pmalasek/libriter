package web

import (
	"net"
	"net/http"
	"strings"
)

// HostMux vybírá webové rozhraní produktu podle hlavičky Host. API na hostu
// nezávisí (cesty mají prefix produktu); podle hostu se rozhoduje jen to,
// které SPA dostane prohlížeč na „/“.
type HostMux struct {
	byHost   map[string]http.Handler
	fallback http.Handler
}

// NewHostMux vytvoří mux s výchozím handlerem pro neznámé hosty (localhost,
// IP adresa, host bez nastavení).
func NewHostMux(fallback http.Handler) *HostMux {
	return &HostMux{byHost: map[string]http.Handler{}, fallback: fallback}
}

// Handle přiřadí handler hostům. Porovnává se bez portu a bez ohledu na velikost písmen.
func (m *HostMux) Handle(hosts []string, h http.Handler) {
	for _, host := range hosts {
		if host = normalizeHost(host); host != "" {
			m.byHost[host] = h
		}
	}
}

func (m *HostMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h, ok := m.byHost[normalizeHost(r.Host)]; ok {
		h.ServeHTTP(w, r)
		return
	}
	m.fallback.ServeHTTP(w, r)
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}
