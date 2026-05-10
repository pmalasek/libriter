package handler

import (
	"errors"
	"net/http"
	"strconv"

	"libriter/internal/metadata/databazeknih"

	"github.com/go-chi/chi/v5"
)

type MetadataHandler struct {
	client *databazeknih.Client
}

func NewMetadata(client *databazeknih.Client) *MetadataHandler {
	return &MetadataHandler{client: client}
}

// GET /api/v1/metadata/search?q=<dotaz>
//
// Vyhledá knihy na databazeknih.cz a vrátí seznam výsledků.
// Přístup: editor+
func (h *MetadataHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "parametr q je povinný")
		return
	}

	results, err := h.client.Search(r.Context(), q)
	if err != nil {
		writeError(w, http.StatusBadGateway, "chyba při vyhledávání na databazeknih.cz")
		return
	}

	if results == nil {
		results = []databazeknih.SearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// GET /api/v1/metadata/book/{id}
//
// Stáhne metadata knihy z databazeknih.cz podle jejího interního ID.
// Přístup: editor+
func (h *MetadataHandler) FetchByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "neplatné ID knihy")
		return
	}

	meta, err := h.client.FetchBook(r.Context(), id)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "vypršel čas požadavku")
			return
		}
		writeError(w, http.StatusBadGateway, "chyba při stahování metadat z databazeknih.cz")
		return
	}

	writeJSON(w, http.StatusOK, meta)
}

// GET /api/v1/metadata/book?url=<url>
//
// Stáhne metadata knihy přímo z URL stránky na databazeknih.cz.
// Preferovaná metoda po vyhledávání – URL pochází ze SearchResult.
// Přístup: editor+
func (h *MetadataHandler) FetchByURL(w http.ResponseWriter, r *http.Request) {
	bookURL := r.URL.Query().Get("url")
	if bookURL == "" {
		writeError(w, http.StatusBadRequest, "parametr url je povinný")
		return
	}

	// Základní ochrana – akceptujeme pouze URLs z databazeknih.cz
	const allowedHost = "databazeknih.cz"
	if !isAllowedHost(bookURL, allowedHost) {
		writeError(w, http.StatusBadRequest, "url musí být z databazeknih.cz")
		return
	}

	meta, err := h.client.FetchBookByURL(r.Context(), bookURL)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "vypršel čas požadavku")
			return
		}
		writeError(w, http.StatusBadGateway, "chyba při stahování metadat z databazeknih.cz")
		return
	}

	writeJSON(w, http.StatusOK, meta)
}

// isAllowedHost ověří, zda URL patří k povolenému hostu (bez importu net/url jako závislosti).
func isAllowedHost(rawURL, host string) bool {
	// Musí začínat http:// nebo https:// + host
	for _, scheme := range []string{"https://", "http://"} {
		after, ok := cutPrefix(rawURL, scheme)
		if !ok {
			continue
		}
		// after = "www.databazeknih.cz/..." nebo "databazeknih.cz/..."
		if hasHostPrefix(after, host) {
			return true
		}
	}
	return false
}

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

func hasHostPrefix(s, host string) bool {
	// s = "www.databazeknih.cz/path" nebo "databazeknih.cz/path"
	if len(s) >= len(host) && s[:len(host)] == host {
		// Musí následovat '/', '?' nebo konec
		if len(s) == len(host) || s[len(host)] == '/' || s[len(host)] == '?' {
			return true
		}
	}
	// www. prefix
	const www = "www."
	if len(s) >= len(www)+len(host) && s[:len(www)] == www {
		return hasHostPrefix(s[len(www):], host)
	}
	return false
}
