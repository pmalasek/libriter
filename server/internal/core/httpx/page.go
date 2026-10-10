package httpx

import (
	"net/http"
	"strconv"
)

// Výchozí a nejvyšší velikost stránky výpisu.
const (
	DefaultPageLimit = 100
	MaxPageLimit     = 500
)

// PageParams je požadovaná stránka výpisu: Limit položek od pozice Offset.
//
// Výpisy stránkují posunem, ne kurzorem: klient zná celkový počet a skáče
// rovnou na libovolné místo seznamu (posuvník, písmeno abecedy) a drží v
// paměti jen stránky kolem viditelné části.
type PageParams struct {
	Limit  int
	Offset int
}

// Page je jedna stránka výpisu. Total je počet všech položek odpovídajících
// filtru, ne jen těch na stránce.
type Page[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// NewPage složí stránku; nil položky se pošlou jako prázdné pole.
func NewPage[T any](items []T, total int, p PageParams) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Total: total, Offset: p.Offset, Limit: p.Limit}
}

// ParsePage přečte ?limit= a ?offset=. paged je false, když klient o stránku
// nežádá (chybí limit) – endpoint pak vrací celý seznam jako dřív, kvůli
// vydaným klientům. Nesmyslné hodnoty se oříznou, nechybují.
func ParsePage(r *http.Request) (p PageParams, paged bool) {
	q := r.URL.Query()
	if !q.Has("limit") {
		return PageParams{}, false
	}
	p.Limit = clampInt(atoiOr(q.Get("limit"), DefaultPageLimit), 1, MaxPageLimit)
	p.Offset = max(atoiOr(q.Get("offset"), 0), 0)
	return p, true
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func clampInt(n, lo, hi int) int {
	return min(max(n, lo), hi)
}
