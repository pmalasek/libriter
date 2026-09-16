package handler

import (
	"errors"
	"net/http"
	"path/filepath"

	"libriter/internal/model"
	"libriter/internal/service"

	"github.com/google/uuid"
)

// chapterResponse je kapitola pro klienta. Celá cesta k souboru se
// nevystavuje (model.Chapter má json:"-"); k rozpoznání pořadí stačí název
// souboru a adresář knihy je veřejný už v Book.file_path.
type chapterResponse struct {
	ID                 uuid.UUID `json:"id"`
	Position           int       `json:"position"`
	Title              string    `json:"title"`
	FileName           string    `json:"file_name"`
	StartOffsetSeconds int       `json:"start_offset_seconds"`
	DurationSeconds    int       `json:"duration_seconds"`
}

// chapterResponses převede kapitoly pro odpověď; kniha bez kapitol dostane
// prázdné pole, ne null.
func chapterResponses(chapters []model.Chapter) []chapterResponse {
	out := make([]chapterResponse, 0, len(chapters))
	for _, c := range chapters {
		out = append(out, chapterResponse{
			ID:                 c.ID,
			Position:           c.Position,
			Title:              c.Title,
			FileName:           filepath.Base(c.FilePath),
			StartOffsetSeconds: c.StartOffsetSeconds,
			DurationSeconds:    c.DurationSeconds,
		})
	}
	return out
}

// GET /api/v1/books/{id}/chapters  (reader+)
func (h *BookHandler) ListChapters(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	chapters, err := h.svc.Chapters(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "kniha nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání kapitol")
		return
	}
	writeJSON(w, http.StatusOK, chapterResponses(chapters))
}

// reorderChaptersRequest nese všechny kapitoly knihy v novém pořadí.
type reorderChaptersRequest struct {
	ChapterIDs []string `json:"chapter_ids"`
}

// PUT /api/v1/books/{id}/chapters/order  (editor+)
//
// Klient posílá kompletní seznam ID kapitol; server je přečísluje 1..N.
// Částečný seznam se odmítne – pořadí musí být jednoznačné pro celou knihu.
func (h *BookHandler) ReorderChapters(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	var req reorderChaptersRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}
	chapterIDs, err := parseUUIDs(req.ChapterIDs, "chapter_ids")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	chapters, err := h.svc.ReorderChapters(r.Context(), id, chapterIDs)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "kniha nenalezena")
		return
	case errors.Is(err, service.ErrChapterSetMismatch):
		writeError(w, http.StatusBadRequest,
			"chapter_ids musí obsahovat všechny kapitoly knihy, každou právě jednou")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "chyba při ukládání pořadí kapitol")
		return
	}
	writeJSON(w, http.StatusOK, chapterResponses(chapters))
}
