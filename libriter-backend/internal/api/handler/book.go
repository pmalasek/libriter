package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

type BookHandler struct {
	svc *service.BookService
}

func NewBook(svc *service.BookService) *BookHandler {
	return &BookHandler{svc: svc}
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
