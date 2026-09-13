package handler

import (
	"errors"
	"net/http"
	"strconv"

	"libriter/internal/metadata"
	"libriter/internal/metadata/databazeknih"

	"github.com/go-chi/chi/v5"
)

// MetadataHandler obsluhuje vyhledávání metadat. Zdroje a jejich pořadí
// určuje METADATA_PROVIDERS, viz metadata.Chain.
type MetadataHandler struct {
	chain *metadata.Chain
	// dk je potřeba jen pro /metadata/book/{id}, kde je číselné ID
	// specifické pro databazeknih.cz. Může být nil, pokud zdroj není zapnutý.
	dk *databazeknih.Client
}

func NewMetadata(chain *metadata.Chain, dk *databazeknih.Client) *MetadataHandler {
	return &MetadataHandler{chain: chain, dk: dk}
}

// GET /api/v1/metadata/search?q=<název>&author=<autor>
//
// Zkouší zdroje v nakonfigurovaném pořadí, vrátí výsledky prvního, který
// něco najde. Autor je nepovinný, ale výrazně zpřesňuje hledání – viz
// metadata.SearchQuery.
// Přístup: editor+
func (h *MetadataHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := metadata.SearchQuery{
		Title:  r.URL.Query().Get("q"),
		Author: r.URL.Query().Get("author"),
	}
	if query.Title == "" {
		writeError(w, http.StatusBadRequest, "parametr q je povinný")
		return
	}

	results, err := h.chain.Search(r.Context(), query)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "vypršel čas požadavku")
			return
		}
		// Důvod se propouští k editorovi schválně: typicky jde o vyčerpanou
		// kvótu nebo rozbitý scraper a z obecné hlášky by nešlo poznat, co dělat.
		writeError(w, http.StatusBadGateway, "zdroje metadat selhaly – "+err.Error())
		return
	}

	if results == nil {
		results = []metadata.SearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// GET /api/v1/metadata/sources
//
// Vrátí zdroje v pořadí, ve kterém se zkoušejí – rozhraní podle toho
// popisuje, odkud data přijdou.
// Přístup: editor+
func (h *MetadataHandler) Sources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{
		"books":   h.chain.Providers(),
		"authors": h.chain.AuthorProviders(),
	})
}

// GET /api/v1/metadata/author/search?q=<dotaz>  (editor+)
func (h *MetadataHandler) SearchAuthors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "parametr q je povinný")
		return
	}

	results, err := h.chain.SearchAuthors(r.Context(), q)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "vypršel čas požadavku")
			return
		}
		writeError(w, http.StatusBadGateway, "zdroje metadat selhaly – "+err.Error())
		return
	}

	if results == nil {
		results = []metadata.AuthorSearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// GET /api/v1/metadata/author?url=<url>  (editor+)
//
// Zdroj se vybere podle adresy; tím zároveň vzniká allowlist.
func (h *MetadataHandler) FetchAuthorByURL(w http.ResponseWriter, r *http.Request) {
	authorURL := r.URL.Query().Get("url")
	if authorURL == "" {
		writeError(w, http.StatusBadRequest, "parametr url je povinný")
		return
	}

	meta, err := h.chain.FetchAuthorByURL(r.Context(), authorURL)
	switch {
	case errors.Is(err, metadata.ErrNoProvider):
		writeError(w, http.StatusBadRequest, "url nepatří žádnému zapnutému zdroji metadat")
		return
	case errors.Is(err, r.Context().Err()) && err != nil:
		writeError(w, http.StatusGatewayTimeout, "vypršel čas požadavku")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "chyba při stahování metadat autora")
		return
	}

	writeJSON(w, http.StatusOK, meta)
}

// GET /api/v1/metadata/book/{id}
//
// Stáhne metadata podle interního ID databazeknih.cz. Ostatní zdroje číselné
// ID nesdílejí, pro ně slouží varianta s url.
// Přístup: editor+
func (h *MetadataHandler) FetchByID(w http.ResponseWriter, r *http.Request) {
	if h.dk == nil {
		writeError(w, http.StatusNotFound, "zdroj databazeknih.cz není zapnutý")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "neplatné ID knihy")
		return
	}

	meta, err := h.dk.FetchBook(r.Context(), id)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "vypršel čas požadavku")
			return
		}
		writeError(w, http.StatusBadGateway, "chyba při stahování metadat z databazeknih.cz")
		return
	}

	meta.Source = h.dk.Name()
	if len(meta.Authors) == 0 {
		meta.Authors = metadata.SplitAuthors(meta.Author)
	}
	writeJSON(w, http.StatusOK, meta)
}

// GET /api/v1/metadata/book?url=<url>
//
// Stáhne metadata přímo z URL. Zdroj se vybere podle adresy – a zároveň tím
// vzniká allowlist, protože cizí adresu neobslouží nikdo.
// Přístup: editor+
func (h *MetadataHandler) FetchByURL(w http.ResponseWriter, r *http.Request) {
	bookURL := r.URL.Query().Get("url")
	if bookURL == "" {
		writeError(w, http.StatusBadRequest, "parametr url je povinný")
		return
	}

	meta, err := h.chain.FetchByURL(r.Context(), bookURL)
	switch {
	case errors.Is(err, metadata.ErrNoProvider):
		writeError(w, http.StatusBadRequest, "url nepatří žádnému zapnutému zdroji metadat")
		return
	case errors.Is(err, r.Context().Err()) && err != nil:
		writeError(w, http.StatusGatewayTimeout, "vypršel čas požadavku")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "chyba při stahování metadat")
		return
	}

	writeJSON(w, http.StatusOK, meta)
}
