package handler

import (
	"errors"
	"net/http"
	"strconv"

	"libriter/internal/metadata"
	"libriter/internal/metadata/databazeknih"
	"libriter/internal/service"

	"github.com/go-chi/chi/v5"
)

// LanguageChainSource dodává řetězce zdrojů: všech zapnutých (detail podle
// URL, autoři) a pro konkrétní jazyk knihy (hledání). Server předává
// metadata.Registry, testům stačí *metadata.Chain.
type LanguageChainSource interface {
	service.ChainSource
	ChainFor(language string) *metadata.Chain
	AuthorChainFor(uiLanguage string) *metadata.Chain
}

// MetadataHandler obsluhuje vyhledávání metadat. Které zdroje a v jakém
// pořadí se zkoušejí, určuje nastavení v administraci – proto se řetězec
// bere až při požadavku, ne jednou při startu.
type MetadataHandler struct {
	source LanguageChainSource
}

func NewMetadata(source LanguageChainSource) *MetadataHandler {
	return &MetadataHandler{source: source}
}

// authorChain vrátí řetězec pro autory podle jazyka rozhraní z parametru
// ui_language.
func (h *MetadataHandler) authorChain(r *http.Request) *metadata.Chain {
	return h.source.AuthorChainFor(normalizeLanguage(r.URL.Query().Get("ui_language")))
}

// chain vrátí aktuální řetězec všech zapnutých zdrojů.
func (h *MetadataHandler) chain() *metadata.Chain {
	return h.source.Chain()
}

// chainFor vrátí řetězec pro jazyk z parametru language; bez něj výchozí.
func (h *MetadataHandler) chainFor(r *http.Request) *metadata.Chain {
	return h.source.ChainFor(normalizeLanguage(r.URL.Query().Get("language")))
}

// dkClient vrátí klienta databazeknih.cz, pokud je tento zdroj zapnutý.
// Ostatní zdroje číselné ID knihy nesdílejí, takže endpoint /metadata/book/{id}
// bez něj obsloužit nejde.
func (h *MetadataHandler) dkClient() (*databazeknih.Client, bool) {
	p, ok := h.chain().Provider(databazeknih.ProviderName)
	if !ok {
		return nil, false
	}
	dk, ok := p.(*databazeknih.Client)
	return dk, ok
}

// GET /api/v1/metadata/search?q=<název>&author=<autor>&language=<kód>
//
// Zkouší zdroje v pořadí nastaveném pro jazyk knihy (bez jazyka nebo pro
// jazyk bez vlastního profilu ve výchozím pořadí), vrátí výsledky prvního,
// který něco najde. Autor je nepovinný, ale výrazně zpřesňuje hledání – viz
// metadata.SearchQuery.
// Přístup: editor+
func (h *MetadataHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := metadata.SearchQuery{
		Title:    r.URL.Query().Get("q"),
		Author:   r.URL.Query().Get("author"),
		Language: normalizeLanguage(r.URL.Query().Get("language")),
	}
	if query.Title == "" {
		writeError(w, http.StatusBadRequest, "validation.query_required", "parametr q je povinný")
		return
	}

	results, err := h.chainFor(r).Search(r.Context(), query)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "request.timeout", "vypršel čas požadavku")
			return
		}
		// Důvod se propouští k editorovi schválně: typicky jde o vyčerpanou
		// kvótu nebo rozbitý scraper a z obecné hlášky by nešlo poznat, co dělat.
		writeError(w, http.StatusBadGateway, "metadata.fetch_failed", "zdroje metadat selhaly – "+err.Error())
		return
	}

	if results == nil {
		results = []metadata.SearchResult{}
	}
	writeJSON(w, http.StatusOK, results)
}

// GET /api/v1/metadata/sources?language=<kód>&ui_language=<kód>
//
// Vrátí zdroje v pořadí, ve kterém se zkoušejí – knihy pro jazyk knihy,
// autory pro jazyk rozhraní. Rozhraní podle toho popisuje, odkud data přijdou.
// Přístup: editor+
func (h *MetadataHandler) Sources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{
		"books":   h.chainFor(r).Providers(),
		"authors": h.authorChain(r).AuthorProviders(),
	})
}

// GET /api/v1/metadata/author/search?q=<dotaz>&ui_language=<kód>  (editor+)
//
// Zdroje se vybírají podle jazyka rozhraní (viz authorChain) – životopis
// má být v jazyce, kterému hledající rozumí.
func (h *MetadataHandler) SearchAuthors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "validation.query_required", "parametr q je povinný")
		return
	}

	results, err := h.authorChain(r).SearchAuthors(r.Context(), q)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "request.timeout", "vypršel čas požadavku")
			return
		}
		writeError(w, http.StatusBadGateway, "metadata.fetch_failed", "zdroje metadat selhaly – "+err.Error())
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
		writeError(w, http.StatusBadRequest, "validation.url_required", "parametr url je povinný")
		return
	}

	meta, err := h.chain().FetchAuthorByURL(r.Context(), authorURL)
	switch {
	case errors.Is(err, metadata.ErrNoProvider):
		writeError(w, http.StatusBadRequest, "metadata.url_unsupported", "url nepatří žádnému zapnutému zdroji metadat")
		return
	case errors.Is(err, r.Context().Err()) && err != nil:
		writeError(w, http.StatusGatewayTimeout, "request.timeout", "vypršel čas požadavku")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "metadata.fetch_failed", "chyba při stahování metadat autora")
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
	dk, ok := h.dkClient()
	if !ok {
		writeError(w, http.StatusNotFound, "metadata.no_providers", "zdroj databazeknih.cz není zapnutý")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "request.invalid_id", "neplatné ID knihy")
		return
	}

	meta, err := dk.FetchBook(r.Context(), id)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeError(w, http.StatusGatewayTimeout, "request.timeout", "vypršel čas požadavku")
			return
		}
		writeError(w, http.StatusBadGateway, "metadata.fetch_failed", "chyba při stahování metadat z databazeknih.cz")
		return
	}

	meta.Source = dk.Name()
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
		writeError(w, http.StatusBadRequest, "validation.url_required", "parametr url je povinný")
		return
	}

	meta, err := h.chain().FetchByURL(r.Context(), bookURL)
	switch {
	case errors.Is(err, metadata.ErrNoProvider):
		writeError(w, http.StatusBadRequest, "metadata.url_unsupported", "url nepatří žádnému zapnutému zdroji metadat")
		return
	case errors.Is(err, r.Context().Err()) && err != nil:
		writeError(w, http.StatusGatewayTimeout, "request.timeout", "vypršel čas požadavku")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "metadata.fetch_failed", "chyba při stahování metadat")
		return
	}

	writeJSON(w, http.StatusOK, meta)
}
