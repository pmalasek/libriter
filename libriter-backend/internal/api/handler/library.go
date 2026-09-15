package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"libriter/internal/imagestore"
	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"
)

type AuthorHandler struct {
	svc *service.AuthorService
	// imageRoot je adresář s fotkami autorů (AUTHOR_IMAGE_ROOT).
	imageRoot string
	// images stahuje fotky z povolených zdrojů; nil = zdroje jsou vypnuté.
	images *service.AuthorImageService
	// audit je volitelný (nil = mazání se nezaznamenává).
	audit *service.AuditService
}

func NewAuthor(
	svc *service.AuthorService,
	imageRoot string,
	images *service.AuthorImageService,
	audit *service.AuditService,
) *AuthorHandler {
	return &AuthorHandler{svc: svc, imageRoot: imageRoot, images: images, audit: audit}
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
	var name string
	if a, err := h.svc.GetByID(r.Context(), id); err == nil {
		name = a.Name
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

	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:      service.AuditAuthorDelete,
		TargetType:  service.AuditTargetAuthor,
		TargetID:    id.String(),
		TargetLabel: name,
	})

	w.WriteHeader(http.StatusNoContent)
}

// GET|HEAD /api/v1/authors/{id}/image
//
// Veřejný endpoint ze stejného důvodu jako obálky knih – <img> v prohlížeči
// neumí poslat hlavičku Authorization. Ochranou je neuhodnutelné UUID autora.
func (h *AuthorHandler) Image(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	author, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "autor nenalezen")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání autora")
		return
	}

	if author.ImagePath == nil {
		writeError(w, http.StatusNotFound, "autor nemá obrázek")
		return
	}

	// image_path může editor nastavit přes PUT na cokoliv, proto validujeme.
	abs, ok := imagestore.Resolve(h.imageRoot, *author.ImagePath)
	if !ok || !imagestore.Exists(h.imageRoot, *author.ImagePath) {
		writeError(w, http.StatusNotFound, "obrázek nenalezen")
		return
	}

	// Frontend přidává ?v=<image_path>, takže dlouhá cache je bezpečná.
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	http.ServeFile(w, r, abs)
}

// PUT /api/v1/authors/{id}/image  (editor+)
//
// Stáhne obrázek z adresy u zdroje metadat a uloží ho do AUTHOR_IMAGE_ROOT.
// Adresu určuje klient, proto se pouští jen hostitelé zapnutých zdrojů.
func (h *AuthorHandler) SetImage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if h.images == nil {
		writeError(w, http.StatusNotFound, "zdroje metadat nejsou zapnuté")
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "url je povinná")
		return
	}

	author, err := h.images.SetFromURL(r.Context(), id, strings.TrimSpace(req.URL))
	switch {
	case errors.Is(err, service.ErrImageNotAllowed):
		writeError(w, http.StatusBadRequest, "adresa obrázku nepatří žádnému zapnutému zdroji metadat")
		return
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "autor nenalezen")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "obrázek se nepodařilo stáhnout – "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, author)
}

// DELETE /api/v1/authors/{id}/image  (editor+)
func (h *AuthorHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	if h.images == nil {
		writeError(w, http.StatusNotFound, "zdroje metadat nejsou zapnuté")
		return
	}

	author, err := h.images.Clear(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "autor nenalezen")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při mazání obrázku")
		return
	}
	writeJSON(w, http.StatusOK, author)
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
	BirthYear  *int    `json:"birth_year"`
	DeathYear  *int    `json:"death_year"`
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

	if err := checkLifeYears(req.BirthYear, req.DeathYear); err != nil {
		return storage.AuthorInput{}, err
	}

	return storage.AuthorInput{
		Name:      name,
		Bio:       req.Bio,
		ImagePath: req.ImagePath,
		BirthYear: req.BirthYear,
		DeathYear: req.DeathYear,
	}, nil
}

// checkLifeYears ověří roky života. Zdroje metadat je občas přečtou špatně,
// takže nesmysly je lepší odmítnout, než je uložit.
func checkLifeYears(birth, death *int) error {
	for _, year := range []*int{birth, death} {
		if year != nil && (*year < 1000 || *year > time.Now().Year()) {
			return errors.New("rok narození i úmrtí musí být mezi 1000 a letošním rokem")
		}
	}
	if birth != nil && death != nil && *death < *birth {
		return errors.New("rok úmrtí nesmí být dřív než rok narození")
	}
	return nil
}

// --- Series ---

type SeriesHandler struct {
	svc *service.SeriesService
	// audit je volitelný (nil = mazání se nezaznamenává).
	audit *service.AuditService
}

func NewSeries(svc *service.SeriesService, audit *service.AuditService) *SeriesHandler {
	return &SeriesHandler{svc: svc, audit: audit}
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
	var title string
	if sr, err := h.svc.GetByID(r.Context(), id); err == nil {
		title = sr.Title
	}

	if err := h.svc.Delete(r.Context(), id); errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "série nenalezena")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při mazání série")
		return
	}

	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:      service.AuditSeriesDelete,
		TargetType:  service.AuditTargetSeries,
		TargetID:    id.String(),
		TargetLabel: title,
	})

	w.WriteHeader(http.StatusNoContent)
}
