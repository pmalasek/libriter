package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

type BookHandler struct {
	svc       *service.BookService
	coverRoot string // adresář s obálkami (COVER_ROOT)
}

func NewBook(svc *service.BookService, coverRoot string) *BookHandler {
	return &BookHandler{svc: svc, coverRoot: coverRoot}
}

// GET /api/v1/books
func (h *BookHandler) List(w http.ResponseWriter, r *http.Request) {
	books, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání knih")
		return
	}
	writeJSON(w, http.StatusOK, books)
}

// GET /api/v1/books/{id}
func (h *BookHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	book, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "kniha nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání knihy")
		return
	}
	writeJSON(w, http.StatusOK, book)
}

// GET|HEAD /api/v1/books/{id}/cover
//
// Veřejný endpoint - <img> v prohlížeči neumí poslat hlavičku Authorization.
// Ochranou je neuhodnutelné UUID knihy, které zná jen přihlášený uživatel.
func (h *BookHandler) Cover(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	book, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "kniha nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání knihy")
		return
	}

	if book.CoverPath == nil {
		writeError(w, http.StatusNotFound, "kniha nemá obálku")
		return
	}

	// cover_path může editor nastavit přes PUT na cokoliv, proto validujeme.
	abs, ok := resolveCoverPath(h.coverRoot, *book.CoverPath)
	if !ok {
		writeError(w, http.StatusNotFound, "obálka nenalezena")
		return
	}

	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "obálka nenalezena")
		return
	}

	// Obálky se mění zřídka a frontend přidává ?v=<updated_at>, takže dlouhá cache je bezpečná.
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	http.ServeFile(w, r, abs)
}

// resolveCoverPath ověří, že coverPath je holý název souboru, a vrátí
// absolutní cestu uvnitř coverRoot. Chrání před path traversal.
func resolveCoverPath(coverRoot, coverPath string) (string, bool) {
	if coverRoot == "" || coverPath == "" || coverPath == "." || coverPath == ".." {
		return "", false
	}
	if strings.ContainsAny(coverPath, `/\`) || filepath.Base(coverPath) != coverPath {
		return "", false
	}

	// COVER_ROOT může být relativní cesta (viz config.env.path).
	root, err := filepath.Abs(coverRoot)
	if err != nil {
		return "", false
	}

	abs := filepath.Join(root, coverPath)
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", false
	}
	return abs, true
}

// POST /api/v1/books  (editor+)
func (h *BookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req bookRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	in, err := req.toInput()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	book, err := h.svc.Create(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při vytváření knihy")
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

// PUT /api/v1/books/{id}  (editor+)
func (h *BookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req bookRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	in, err := req.toInput()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	book, err := h.svc.Update(r.Context(), id, in)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "kniha nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při aktualizaci knihy")
		return
	}
	writeJSON(w, http.StatusOK, book)
}

// DELETE /api/v1/books/{id}  (admin)
func (h *BookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	if err := h.svc.Delete(r.Context(), id); errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "kniha nenalezena")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při mazání knihy")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- request / helper ---

type bookRequest struct {
	AuthorIDs       []string `json:"author_ids"`
	SeriesID        *string  `json:"series_id"`
	SeriesPosition  *int16   `json:"series_position"`
	Title           string   `json:"title"`
	Narrator        *string  `json:"narrator"`
	DurationSeconds int      `json:"duration_seconds"`
	FilePath        string   `json:"file_path"`
	CoverPath       *string  `json:"cover_path"`
	Language        string   `json:"language"`
	Description     *string  `json:"description"`
	InternalRating  *int16   `json:"internal_rating"`
}

func (req *bookRequest) toInput() (storage.BookInput, error) {
	if strings.TrimSpace(req.Title) == "" {
		return storage.BookInput{}, errors.New("title je povinný")
	}
	if req.DurationSeconds <= 0 {
		return storage.BookInput{}, errors.New("duration_seconds musí být kladné číslo")
	}
	if strings.TrimSpace(req.FilePath) == "" {
		return storage.BookInput{}, errors.New("file_path je povinný")
	}
	if req.InternalRating != nil && (*req.InternalRating < 1 || *req.InternalRating > 5) {
		return storage.BookInput{}, errors.New("internal_rating musí být 1–5")
	}

	authorIDs, err := parseUUIDs(req.AuthorIDs, "author_ids")
	if err != nil {
		return storage.BookInput{}, err
	}
	if len(authorIDs) == 0 {
		return storage.BookInput{}, errors.New("author_ids musí obsahovat alespoň jednoho autora")
	}

	in := storage.BookInput{
		AuthorIDs:       authorIDs,
		SeriesPosition:  req.SeriesPosition,
		Title:           strings.TrimSpace(req.Title),
		Narrator:        req.Narrator,
		DurationSeconds: req.DurationSeconds,
		FilePath:        strings.TrimSpace(req.FilePath),
		CoverPath:       req.CoverPath,
		Language:        req.Language,
		Description:     req.Description,
		InternalRating:  req.InternalRating,
	}

	if req.Language == "" {
		in.Language = "cs"
	}

	if req.SeriesID != nil {
		sid, err := parseUUIDStr(*req.SeriesID, "series_id")
		if err != nil {
			return storage.BookInput{}, err
		}
		in.SeriesID = &sid
	}

	return in, nil
}

// parseUUIDs převede seznam ID; zachovává pořadí (určuje pořadí autorů u knihy).
func parseUUIDs(values []string, field string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(values))
	for _, v := range values {
		id, err := parseUUIDStr(v, field)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func parseUUIDStr(s, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("neplatné %s", field)
	}
	return id, nil
}
