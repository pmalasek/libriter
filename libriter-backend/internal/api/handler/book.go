package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"libriter/internal/imagestore"
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
	abs, ok := imagestore.Resolve(h.coverRoot, *book.CoverPath)
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

// PATCH /api/v1/books/{id}  (editor+)
//
// Mění jen pole, která klient skutečně poslal. Existuje vedle PUT proto, že
// file_path se přes API nevystavuje (model.Book má json:"-"), takže úplná
// náhrada by cestu k audiu přepsala prázdnou hodnotou.
func (h *BookHandler) Patch(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req bookPatchRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	apply, err := req.toPatch()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	book, err := h.svc.Patch(r.Context(), id, apply)
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
	PublishedYear   *int     `json:"published_year"`
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
	if err := checkPublishedYear(req.PublishedYear); err != nil {
		return storage.BookInput{}, err
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
		PublishedYear:   req.PublishedYear,
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

// bookPatchRequest je bookRequest pro částečnou aktualizaci – každé pole nese
// navíc informaci, jestli ho klient poslal. file_path chybí schválně: cestu
// k audiu spravuje výhradně scanner a klient ji nikdy nevidí.
type bookPatchRequest struct {
	AuthorIDs      Optional[[]string] `json:"author_ids"`
	SeriesID       Optional[*string]  `json:"series_id"`
	SeriesPosition Optional[*int16]   `json:"series_position"`
	Title          Optional[string]   `json:"title"`
	Narrator       Optional[*string]  `json:"narrator"`
	// duration_seconds zůstává i tady povinně kladné – nulová délka by rozbila
	// zobrazení i budoucí přehrávání.
	DurationSeconds Optional[int]     `json:"duration_seconds"`
	CoverPath       Optional[*string] `json:"cover_path"`
	Language        Optional[string]  `json:"language"`
	Description     Optional[*string] `json:"description"`
	InternalRating  Optional[*int16]  `json:"internal_rating"`
	PublishedYear   Optional[*int]    `json:"published_year"`
}

// toPatch ověří poslaná pole a vrátí funkci, která je zanese do vstupu knihy.
// Validuje se jen to, co klient poslal – zbytek vstupu pochází z uloženého
// řádku, který je už platný.
func (req *bookPatchRequest) toPatch() (func(*storage.BookInput), error) {
	if req.Title.Set && strings.TrimSpace(req.Title.Value) == "" {
		return nil, errors.New("title nesmí být prázdný")
	}
	if req.DurationSeconds.Set && req.DurationSeconds.Value <= 0 {
		return nil, errors.New("duration_seconds musí být kladné číslo")
	}
	if req.InternalRating.Set && req.InternalRating.Value != nil &&
		(*req.InternalRating.Value < 1 || *req.InternalRating.Value > 5) {
		return nil, errors.New("internal_rating musí být 1–5")
	}
	if req.PublishedYear.Set {
		if err := checkPublishedYear(req.PublishedYear.Value); err != nil {
			return nil, err
		}
	}

	var authorIDs []uuid.UUID
	if req.AuthorIDs.Set {
		ids, err := parseUUIDs(req.AuthorIDs.Value, "author_ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, errors.New("author_ids musí obsahovat alespoň jednoho autora")
		}
		authorIDs = ids
	}

	var seriesID *uuid.UUID
	if req.SeriesID.Set && req.SeriesID.Value != nil {
		sid, err := parseUUIDStr(*req.SeriesID.Value, "series_id")
		if err != nil {
			return nil, err
		}
		seriesID = &sid
	}

	return func(in *storage.BookInput) {
		if req.AuthorIDs.Set {
			in.AuthorIDs = authorIDs
		}
		if req.SeriesID.Set {
			in.SeriesID = seriesID // null v těle sérii odpojí
		}
		if req.SeriesPosition.Set {
			in.SeriesPosition = req.SeriesPosition.Value
		}
		if req.Title.Set {
			in.Title = strings.TrimSpace(req.Title.Value)
		}
		if req.Narrator.Set {
			in.Narrator = req.Narrator.Value
		}
		if req.DurationSeconds.Set {
			in.DurationSeconds = req.DurationSeconds.Value
		}
		if req.CoverPath.Set {
			in.CoverPath = req.CoverPath.Value
		}
		if req.Language.Set && req.Language.Value != "" {
			in.Language = req.Language.Value
		}
		if req.Description.Set {
			in.Description = req.Description.Value
		}
		if req.InternalRating.Set {
			in.InternalRating = req.InternalRating.Value
		}
		if req.PublishedYear.Set {
			in.PublishedYear = req.PublishedYear.Value
		}
	}, nil
}

// checkPublishedYear odmítne nesmyslný rok vydání. Zdroje metadat rok občas
// přečtou špatně (např. z ISBN), takže hlídat horní i dolní mez se vyplatí.
func checkPublishedYear(year *int) error {
	if year == nil {
		return nil
	}
	if *year < 1000 || *year > time.Now().Year()+1 {
		return errors.New("published_year musí být mezi 1000 a příštím rokem")
	}
	return nil
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
