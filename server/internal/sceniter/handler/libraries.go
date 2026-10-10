package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	coreservice "mediater/internal/core/service"
	"mediater/internal/sceniter/service"
)

// Audit akce knihoven.
const (
	AuditLibraryCreate  = "sceniter.library_create"
	AuditLibraryUpdate  = "sceniter.library_update"
	AuditLibraryDelete  = "sceniter.library_delete"
	AuditLibraryReorder = "sceniter.library_reorder"
	auditTargetLibrary  = "sceniter_library"
)

// GET /sceniter/libraries – zapnuté knihovny pro menu (reader+).
func (h *Handler) Libraries(w http.ResponseWriter, r *http.Request) {
	libs, err := h.Libs.List(r.Context(), true)
	if writeStoreError(w, err, "library.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, libs)
}

// GET /sceniter/admin/libraries – všechny knihovny včetně vypnutých.
func (h *Handler) AdminLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := h.Libs.List(r.Context(), false)
	if writeStoreError(w, err, "library.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, libs)
}

// GET /sceniter/admin/libraries/candidates – složky VIDEO_ROOT bez knihovny.
func (h *Handler) LibraryCandidates(w http.ResponseWriter, r *http.Request) {
	dirs, err := h.Libs.Candidates(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "library.root_unavailable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dirs": dirs})
}

func (h *Handler) writeLibraryError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, service.ErrConflict) {
		writeError(w, http.StatusConflict, "library.conflict", err.Error())
		return true
	}
	return writeStoreError(w, err, "library.not_found")
}

// POST /sceniter/admin/libraries
func (h *Handler) CreateLibrary(w http.ResponseWriter, r *http.Request) {
	var req service.LibraryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	lib, err := h.Libs.Create(r.Context(), req)
	if h.writeLibraryError(w, err) {
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: AuditLibraryCreate,
		TargetType: auditTargetLibrary, TargetID: lib.ID.String(), TargetLabel: lib.Name, Details: lib})
	writeJSON(w, http.StatusCreated, lib)
}

// PATCH /sceniter/admin/libraries/{id}
func (h *Handler) UpdateLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	var req service.LibraryRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	lib, err := h.Libs.Update(r.Context(), id, req)
	if h.writeLibraryError(w, err) {
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: AuditLibraryUpdate,
		TargetType: auditTargetLibrary, TargetID: lib.ID.String(), TargetLabel: lib.Name, Details: lib})
	writeJSON(w, http.StatusOK, lib)
}

// DELETE /sceniter/admin/libraries/{id}
func (h *Handler) DeleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}
	lib, err := h.Store.GetLibrary(r.Context(), id)
	if writeStoreError(w, err, "library.not_found") {
		return
	}
	if writeStoreError(w, h.Libs.Delete(r.Context(), id), "library.not_found") {
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: AuditLibraryDelete,
		TargetType: auditTargetLibrary, TargetID: id.String(), TargetLabel: lib.Name,
		Details: map[string]any{"dir": lib.Dir, "items": lib.ItemCount}})
	w.WriteHeader(http.StatusNoContent)
}

// PUT /sceniter/admin/libraries/order  {"ids": [...]}
func (h *Handler) ReorderLibraries(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uuid.UUID `json:"ids"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	if writeStoreError(w, h.Libs.Reorder(r.Context(), req.IDs), "library.not_found") {
		return
	}
	h.Audit.Record(r.Context(), userID(r), coreservice.AuditEvent{Action: AuditLibraryReorder,
		TargetType: auditTargetLibrary, Details: req.IDs})
	libs, err := h.Libs.List(r.Context(), false)
	if writeStoreError(w, err, "library.not_found") {
		return
	}
	writeJSON(w, http.StatusOK, libs)
}

// metadataProvider je zdroj video metadat pro výběr v administraci.
type metadataProvider struct {
	Name      string   `json:"name"`
	Title     string   `json:"title"`
	Kinds     []string `json:"kinds"`
	Languages []string `json:"languages"`
}

func (h *Handler) metadataProviders() []metadataProvider {
	out := []metadataProvider{}
	for _, p := range h.Matcher.Providers() {
		var kinds []string
		for _, k := range p.Manifest.Kinds {
			if k == "movie" || k == "show" {
				kinds = append(kinds, k)
			}
		}
		langs := p.Manifest.Languages
		if langs == nil {
			langs = []string{}
		}
		out = append(out, metadataProvider{Name: p.Name(), Title: p.Manifest.Title, Kinds: kinds, Languages: langs})
	}
	return out
}

// GET /sceniter/metadata/providers (editor+)
func (h *Handler) MetadataProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.metadataProviders())
}
