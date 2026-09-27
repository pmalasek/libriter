package handler

import (
	"errors"
	"net/http"
	"time"

	"libriter/internal/importer"
	"libriter/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// uploadDeadline je čas na nahrání jednoho souboru. Globální ReadTimeout
// serveru (15 s) by velký soubor přes pomalou linku utnul.
const uploadDeadline = 2 * time.Hour

// ImportHandler obsluhuje import knih pod /api/v1/admin/import (admin).
type ImportHandler struct {
	svc   *importer.Service
	audit *service.AuditService
}

func NewImport(svc *importer.Service, audit *service.AuditService) *ImportHandler {
	return &ImportHandler{svc: svc, audit: audit}
}

// GET /api/v1/admin/import  (admin) – rozpracované a nedávné importy a limit
// velikosti, aby ho rozhraní ukázalo ještě před prvním nahráním.
func (h *ImportHandler) List(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"max_bytes": h.svc.MaxBytes(),
		"sessions":  h.svc.List(),
	})
}

// POST /api/v1/admin/import  (admin) – založí prázdný import
func (h *ImportHandler) Create(w http.ResponseWriter, r *http.Request) {
	sess, err := h.svc.Create()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "import.create_failed", "import se nepodařilo založit")
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

// GET /api/v1/admin/import/{id}  (admin)
func (h *ImportHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessionID(w, r)
	if !ok {
		return
	}
	sess, err := h.svc.Get(id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

// PUT /api/v1/admin/import/{id}/files?path=<relativní cesta>  (admin)
//
// Tělo je obsah souboru. Soubory se posílají po jednom, aby šel ukázat
// průběh a přerušený upload zopakovat jen pro jeden soubor.
func (h *ImportHandler) Upload(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessionID(w, r)
	if !ok {
		return
	}
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(uploadDeadline))
	_ = rc.SetWriteDeadline(time.Now().Add(uploadDeadline))

	stored, err := h.svc.AddFile(id, r.URL.Query().Get("path"), r.Body)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"stored": stored})
}

// GET /api/v1/admin/import/{id}/files  (admin) – už nahrané soubory
func (h *ImportHandler) Files(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessionID(w, r)
	if !ok {
		return
	}
	files, err := h.svc.Files(id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, files)
}

// POST /api/v1/admin/import/{id}/analyze  (admin) – rozbalí a rozpozná knihy
func (h *ImportHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessionID(w, r)
	if !ok {
		return
	}
	sess, err := h.svc.Analyze(id)
	if err != nil {
		h.writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, sess)
}

// POST /api/v1/admin/import/{id}/commit  (admin) – importuje vybrané knihy
func (h *ImportHandler) Commit(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessionID(w, r)
	if !ok {
		return
	}
	var req struct {
		Books []importer.BookEdit `json:"books"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	sess, err := h.svc.Commit(id, req.Books)
	if err != nil {
		h.writeErr(w, err)
		return
	}

	titles := make([]string, 0, len(sess.Books))
	for _, b := range sess.Books {
		if b.Include {
			titles = append(titles, b.Title)
		}
	}
	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:     service.AuditLibraryImport,
		TargetType: service.AuditTargetLibrary,
		TargetID:   id.String(),
		Details:    map[string]any{"books": titles},
	})

	writeJSON(w, http.StatusAccepted, sess)
}

// GET /api/v1/admin/import/{id}/cover?key=<adresář knihy>  (admin)
func (h *ImportHandler) Cover(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessionID(w, r)
	if !ok {
		return
	}
	filePath, data, mime, err := h.svc.CoverFile(id, r.URL.Query().Get("key"))
	if err != nil {
		h.writeErr(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if filePath != "" {
		http.ServeFile(w, r, filePath)
		return
	}
	if mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	_, _ = w.Write(data)
}

// DELETE /api/v1/admin/import/{id}  (admin) – zruší import a smaže soubory
func (h *ImportHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.sessionID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		h.writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ImportHandler) sessionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_id", "neplatné ID")
		return uuid.Nil, false
	}
	return id, true
}

// writeErr převede chybu importu na odpověď se stabilním kódem.
func (h *ImportHandler) writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, importer.ErrNotFound):
		writeError(w, http.StatusNotFound, "import.not_found", err.Error())
	case errors.Is(err, importer.ErrWrongState):
		writeError(w, http.StatusConflict, "import.wrong_state", err.Error())
	case errors.Is(err, importer.ErrInvalidPath):
		writeError(w, http.StatusBadRequest, "import.invalid_path", err.Error())
	case errors.Is(err, importer.ErrFileType):
		writeError(w, http.StatusUnsupportedMediaType, "import.file_type", err.Error())
	case errors.Is(err, importer.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "import.too_large", err.Error())
	case errors.Is(err, importer.ErrNothingChose):
		writeError(w, http.StatusBadRequest, "import.nothing_selected", err.Error())
	case errors.Is(err, importer.ErrInvalidBook):
		writeError(w, http.StatusBadRequest, "import.invalid_book", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "import.failed", "import se nezdařil – "+err.Error())
	}
}
