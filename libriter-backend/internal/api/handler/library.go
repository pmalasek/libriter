package handler

import (
	"errors"
	"net/http"
	"strings"

	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"
)

type AuthorHandler struct {
	svc *service.AuthorService
}

func NewAuthor(svc *service.AuthorService) *AuthorHandler {
	return &AuthorHandler{svc: svc}
}

// GET /api/v1/authors
func (h *AuthorHandler) List(w http.ResponseWriter, r *http.Request) {
	authors, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání autorů")
		return
	}
	writeJSON(w, http.StatusOK, authors)
}

// GET /api/v1/authors/{id}
func (h *AuthorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	a, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "autor nenalezen")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání autora")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// POST /api/v1/authors  (editor+)
func (h *AuthorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req authorRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	in, err := req.toInput()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a, err := h.svc.Create(r.Context(), in)
	if errors.Is(err, service.ErrConflict) {
		writeError(w, http.StatusConflict, "autor se stejným jménem už existuje")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při vytváření autora")
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

// PUT /api/v1/authors/{id}  (editor+)
func (h *AuthorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req authorRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	in, err := req.toInput()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	a, err := h.svc.Update(r.Context(), id, in)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "autor nenalezen")
		return
	case errors.Is(err, service.ErrConflict):
		writeError(w, http.StatusConflict, "autor se stejným jménem už existuje")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "chyba při aktualizaci autora")
		return
	}
	writeJSON(w, http.StatusOK, a)
}

// DELETE /api/v1/authors/{id}  (admin)
func (h *AuthorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	err := h.svc.Delete(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "autor nenalezen")
		return
	case errors.Is(err, service.ErrConflict):
		writeError(w, http.StatusConflict, "autor má v knihovně knihy")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "chyba při mazání autora")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// authorRequest přijímá jméno po částech; pro pohodlí lze poslat i celé jméno
// v poli name, které se rozdělí samo ("Komenský, Jan Amos" i "Jan Amos Komenský").
type authorRequest struct {
	FirstName  string  `json:"first_name"`
	MiddleName string  `json:"middle_name"`
	LastName   string  `json:"last_name"`
	Name       string  `json:"name"`
	Bio        *string `json:"bio"`
	ImagePath  *string `json:"image_path"`
}

func (req *authorRequest) toInput() (storage.AuthorInput, error) {
	name := model.AuthorName{
		First:  strings.TrimSpace(req.FirstName),
		Middle: strings.TrimSpace(req.MiddleName),
		Last:   strings.TrimSpace(req.LastName),
	}
	if name.IsEmpty() {
		name = model.ParseAuthorName(req.Name)
	}
	if name.Last == "" {
		return storage.AuthorInput{}, errors.New("last_name (příjmení) je povinné")
	}

	return storage.AuthorInput{Name: name, Bio: req.Bio, ImagePath: req.ImagePath}, nil
}

// --- Series ---

type SeriesHandler struct {
	svc *service.SeriesService
}

func NewSeries(svc *service.SeriesService) *SeriesHandler {
	return &SeriesHandler{svc: svc}
}

// GET /api/v1/series
func (h *SeriesHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání sérií")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// GET /api/v1/series/{id}
func (h *SeriesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	sr, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "série nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání série")
		return
	}
	writeJSON(w, http.StatusOK, sr)
}

// POST /api/v1/series  (editor+)
func (h *SeriesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title je povinný")
		return
	}

	sr, err := h.svc.Create(r.Context(), strings.TrimSpace(req.Title), req.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při vytváření série")
		return
	}
	writeJSON(w, http.StatusCreated, sr)
}

// PUT /api/v1/series/{id}  (editor+)
func (h *SeriesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		Title       string  `json:"title"`
		Description *string `json:"description"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title je povinný")
		return
	}

	sr, err := h.svc.Update(r.Context(), id, strings.TrimSpace(req.Title), req.Description)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "série nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při aktualizaci série")
		return
	}
	writeJSON(w, http.StatusOK, sr)
}

// DELETE /api/v1/series/{id}  (admin)
func (h *SeriesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "série nenalezena")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při mazání série")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
