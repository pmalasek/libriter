package handler

import (
	"errors"
	"net/http"

	"libriter/internal/metadata"
	"libriter/internal/scanner"
	"libriter/internal/service"

	"github.com/google/uuid"
)

// metadataSettingsResponse je nastavení zdrojů obohacené o schopnosti, které
// zdroj má – rozhraní podle nich ukazuje, že Google Books autory neumí.
type metadataSettingsResponse struct {
	Providers         []metadataProviderResponse `json:"providers"`
	GoogleBooksAPIKey string                     `json:"google_books_api_key"`
}

type metadataProviderResponse struct {
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	SupportsAuthors bool   `json:"supports_authors"`
	SupportsImages  bool   `json:"supports_images"`
}

// GET /api/v1/admin/settings/metadata  (admin)
func (h *AdminHandler) MetadataSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Metadata(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání nastavení zdrojů")
		return
	}
	writeJSON(w, http.StatusOK, h.describeMetadata(settings))
}

// PUT /api/v1/admin/settings/metadata  (admin)
//
// Tělo nahrazuje celé nastavení: pořadí v seznamu je pořadí, ve kterém se
// zdroje zkoušejí. Po uložení se řetězec zdrojů rovnou vymění, takže změna
// platí bez restartu serveru.
func (h *AdminHandler) SetMetadataSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Providers []struct {
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		} `json:"providers"`
		GoogleBooksAPIKey string `json:"google_books_api_key"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	in := service.MetadataSettings{
		Providers:         make([]service.ProviderSetting, 0, len(req.Providers)),
		GoogleBooksAPIKey: req.GoogleBooksAPIKey,
	}
	for _, p := range req.Providers {
		in.Providers = append(in.Providers, service.ProviderSetting{Name: p.Name, Enabled: p.Enabled})
	}

	saved, err := h.settings.SetMetadata(r.Context(), in)
	if errors.Is(err, service.ErrInvalidSetting) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při ukládání nastavení zdrojů")
		return
	}

	enabled := make([]string, 0, len(saved.Providers))
	for _, p := range saved.Providers {
		if p.Enabled {
			enabled = append(enabled, p.Name)
		}
	}
	h.registry.Rebuild(enabled, metadata.ProviderConfig{GoogleBooksAPIKey: saved.GoogleBooksAPIKey})

	// Klíč do auditu nepatří – stačí, že se změnil.
	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:     service.AuditSettingsMetadata,
		TargetType: service.AuditTargetSettings,
		TargetID:   service.SettingMetadataProviders,
		Details: map[string]any{
			"providers":                  enabled,
			"google_books_api_key_saved": saved.GoogleBooksAPIKey != "",
		},
	})

	writeJSON(w, http.StatusOK, h.describeMetadata(saved))
}

func (h *AdminHandler) describeMetadata(settings service.MetadataSettings) metadataSettingsResponse {
	caps := make(map[string]metadata.ProviderInfo, len(settings.Providers))
	for _, info := range h.registry.Known() {
		caps[info.Name] = info
	}

	out := metadataSettingsResponse{
		Providers:         make([]metadataProviderResponse, 0, len(settings.Providers)),
		GoogleBooksAPIKey: settings.GoogleBooksAPIKey,
	}
	for _, p := range settings.Providers {
		info := caps[p.Name]
		out.Providers = append(out.Providers, metadataProviderResponse{
			Name:            p.Name,
			Enabled:         p.Enabled,
			SupportsAuthors: info.SupportsAuthors,
			SupportsImages:  info.SupportsImages,
		})
	}
	return out
}

// GET /api/v1/admin/settings/registration  (admin)
func (h *AdminHandler) RegistrationSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settings.Registration(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání nastavení registrace")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// PUT /api/v1/admin/settings/registration  (admin)
func (h *AdminHandler) SetRegistrationSettings(w http.ResponseWriter, r *http.Request) {
	var req service.RegistrationSettings
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	saved, err := h.settings.SetRegistration(r.Context(), req)
	if errors.Is(err, service.ErrInvalidSetting) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při ukládání nastavení registrace")
		return
	}

	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:     service.AuditSettingsRegistration,
		TargetType: service.AuditTargetSettings,
		TargetID:   service.SettingRegistration,
		Details:    saved,
	})

	writeJSON(w, http.StatusOK, saved)
}

// GET /api/v1/admin/scanner  (admin)
func (h *AdminHandler) ScannerStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.scanner.Status())
}

// POST /api/v1/admin/scanner/rescan  (admin)
//
// Průchod běží na pozadí; odpověď je jen potvrzení, že se rozjel.
func (h *AdminHandler) Rescan(w http.ResponseWriter, r *http.Request) {
	if err := h.scanner.Rescan(); errors.Is(err, scanner.ErrScanRunning) {
		writeError(w, http.StatusConflict, "kontrola knihovny už běží")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "kontrolu knihovny se nepodařilo spustit")
		return
	}

	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:     service.AuditScannerRescan,
		TargetType: service.AuditTargetLibrary,
	})

	writeJSON(w, http.StatusAccepted, h.scanner.Status())
}

// GET /api/v1/admin/library/repair  (admin)
//
// Náhled opravy: co by se smazalo a načetlo znovu. Nic nemění.
func (h *AdminHandler) RepairPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := h.scanner.PlanRepair(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "kontrolu kapitol se nepodařilo provést – "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// POST /api/v1/admin/library/repair  (admin)
//
// Plán se sestaví znovu na serveru, takže se opravuje vždy aktuální stav –
// klient neposílá seznam knih ke smazání.
func (h *AdminHandler) Repair(w http.ResponseWriter, r *http.Request) {
	result, err := h.scanner.Repair(r.Context())
	if errors.Is(err, scanner.ErrScanRunning) {
		writeError(w, http.StatusConflict, "kontrola knihovny právě běží, zkuste to za chvíli")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oprava kapitol selhala – "+err.Error())
		return
	}

	if !result.Plan.IsEmpty() {
		h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
			Action:     service.AuditLibraryRepair,
			TargetType: service.AuditTargetLibrary,
			Details: map[string]int{
				"rescan":           len(result.Plan.Rescan),
				"duplicates":       len(result.Plan.Duplicates),
				"deleted_chapters": result.DeletedChapters,
				"deleted_books":    result.DeletedBooks,
			},
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// GET /api/v1/admin/library/merge  (admin)
//
// Náhled sloučení: které knihy scanner založil vícekrát. Nic nemění.
func (h *AdminHandler) MergePlan(w http.ResponseWriter, r *http.Request) {
	plan, err := h.scanner.PlanMerge(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError,
			"kontrolu rozdělených knih se nepodařilo provést – "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// POST /api/v1/admin/library/merge  (admin)
//
// Tělo je nepovinné: {"targets": ["<id cíle>", …]} omezí sloučení na vybrané
// skupiny. Plán se stejně jako u opravy kapitol sestaví znovu na serveru –
// klient tedy vybírá jen z toho, co server sám našel.
func (h *AdminHandler) Merge(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Targets []uuid.UUID `json:"targets"`
	}
	if r.ContentLength > 0 {
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "neplatný vstup – "+err.Error())
			return
		}
	}

	result, err := h.scanner.Merge(r.Context(), body.Targets)
	if errors.Is(err, scanner.ErrScanRunning) {
		writeError(w, http.StatusConflict, "kontrola knihovny právě běží, zkuste to za chvíli")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sloučení knih selhalo – "+err.Error())
		return
	}

	if !result.Plan.IsEmpty() {
		titles := make([]string, 0, len(result.Plan.Groups))
		for _, group := range result.Plan.Groups {
			titles = append(titles, group.Target.Title)
		}
		h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
			Action:     service.AuditLibraryMerge,
			TargetType: service.AuditTargetLibrary,
			Details: map[string]any{
				"groups":         len(result.Plan.Groups),
				"merged_books":   result.MergedBooks,
				"moved_chapters": result.MovedChapters,
				"titles":         titles,
			},
		})
	}

	writeJSON(w, http.StatusOK, result)
}
