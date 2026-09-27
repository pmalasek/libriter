package handler

import (
	"errors"
	"net/http"

	"libriter/internal/metadata"
	"libriter/internal/scanner"
	"libriter/internal/service"
	"libriter/internal/storage"

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
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání nastavení zdrojů")
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
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
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
		writeError(w, http.StatusBadRequest, "validation.invalid", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při ukládání nastavení zdrojů")
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
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání nastavení registrace")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// PUT /api/v1/admin/settings/registration  (admin)
func (h *AdminHandler) SetRegistrationSettings(w http.ResponseWriter, r *http.Request) {
	var req service.RegistrationSettings
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}

	saved, err := h.settings.SetRegistration(r.Context(), req)
	if errors.Is(err, service.ErrInvalidSetting) {
		writeError(w, http.StatusBadRequest, "validation.invalid", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při ukládání nastavení registrace")
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
		writeError(w, http.StatusConflict, "library.scan_running", "kontrola knihovny už běží")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "library.scan_start_failed", "kontrolu knihovny se nepodařilo spustit")
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
	if errors.Is(err, scanner.ErrAudioRootUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "library.audio_root_unavailable", audioRootUnavailableMessage(err))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "library.check_failed", "kontrolu kapitol se nepodařilo provést – "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// POST /api/v1/admin/library/repair  (admin)
//
// Plán se sestaví znovu na serveru, takže se opravuje vždy aktuální stav –
// klient neposílá seznam knih ke smazání.
//
// Tělo je nepovinné: {"force_missing": true} přebije pojistku proti
// nepřipojenému disku. Bez něj se při sepnuté pojistce chybějící soubory
// přeskočí a zbytek opravy proběhne normálně.
func (h *AdminHandler) Repair(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ForceMissing bool `json:"force_missing"`
	}
	if r.ContentLength > 0 {
		if err := readJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný vstup – "+err.Error())
			return
		}
	}

	result, err := h.scanner.Repair(r.Context(), body.ForceMissing)
	if errors.Is(err, scanner.ErrScanRunning) {
		writeError(w, http.StatusConflict, "library.scan_running", "kontrola knihovny právě běží, zkuste to za chvíli")
		return
	}
	if errors.Is(err, scanner.ErrAudioRootUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "library.audio_root_unavailable", audioRootUnavailableMessage(err))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "library.repair_failed", "oprava kapitol selhala – "+err.Error())
		return
	}

	if !result.Plan.IsEmpty() {
		h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
			Action:     service.AuditLibraryRepair,
			TargetType: service.AuditTargetLibrary,
			Details: map[string]any{
				"rescan":           len(result.Plan.Rescan),
				"duplicates":       len(result.Plan.Duplicates),
				"orphans":          len(result.Plan.Orphans),
				"missing":          len(result.Plan.Missing),
				"deleted_chapters": result.DeletedChapters,
				"deleted_books":    result.DeletedBooks,
				"deleted_orphans":  result.DeletedOrphans,
				"guard_tripped":    result.Plan.Guard.Tripped,
				"forced":           body.ForceMissing,
			},
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// audioRootUnavailableMessage vysvětlí, proč se kontrola vůbec nespustila.
// Prázdný nebo nečitelný AUDIO_ROOT je skoro vždy nepřipojený disk, a oprava
// by v takovou chvíli navrhla smazat celou knihovnu.
func audioRootUnavailableMessage(err error) string {
	return "adresář s audioknihami není dostupný – zkontrolujte, že je disk připojený (" +
		err.Error() + ")"
}

// GET /api/v1/admin/library/duplicates  (admin)
//
// Hlášení knih, které jsou v knihovně nejspíš dvakrát. Jen čtení: smazat
// kopii znamená sáhnout na soubory, a to zůstává na uživateli (mazání knihy
// v jejím detailu). Falešný nález je možný – dvě vydání téhož titulu
// s jiným vypravěčem jsou legitimní.
func (h *AdminHandler) DuplicateReport(w http.ResponseWriter, r *http.Request) {
	report, err := h.scanner.PlanDuplicates(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "library.check_failed",
			"kontrolu duplicitních knih se nepodařilo provést – "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// POST /api/v1/admin/library/duplicates/dismiss  (admin)
//
// „Není to duplicita.“ Skupina zmizí z nálezů, ale zůstane v odpovědi mezi
// odmítnutými, aby šlo rozhodnutí vzít zpět. Klíč se ověřuje proti čerstvému
// hlášení – odmítnout jde jen to, co server právě sám našel.
func (h *AdminHandler) DismissDuplicate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Key string `json:"key"`
	}
	if err := readJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný vstup – "+err.Error())
		return
	}
	if body.Key == "" {
		writeError(w, http.StatusBadRequest, "validation.invalid", "chybí klíč skupiny")
		return
	}

	actor := actorID(r)
	group, err := h.scanner.DismissDuplicate(r.Context(), body.Key, &actor)
	if errors.Is(err, scanner.ErrDuplicateGroupUnknown) {
		writeError(w, http.StatusConflict, "library.changed",
			"knihovna se mezitím změnila – spusťte kontrolu znovu")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "library.dismiss_failed", "odmítnutí se nepodařilo – "+err.Error())
		return
	}

	titles := make([]string, 0, len(group.Books))
	for _, b := range group.Books {
		titles = append(titles, b.FilePath)
	}
	h.audit.Record(r.Context(), actor, service.AuditEvent{
		Action:      service.AuditDuplicateDismiss,
		TargetType:  service.AuditTargetLibrary,
		TargetLabel: group.Books[0].Title,
		Details:     map[string]any{"match": group.Match, "paths": titles},
	})

	writeJSON(w, http.StatusOK, group)
}

// DELETE /api/v1/admin/library/duplicates/dismiss?key=…  (admin)
//
// Vrátí odmítnutou skupinu zpátky mezi nálezy.
func (h *AdminHandler) RestoreDuplicate(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "validation.invalid", "chybí klíč skupiny")
		return
	}

	err := h.scanner.RestoreDuplicate(r.Context(), key)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "library.changed", "taková odmítnutá skupina není")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "library.dismiss_failed", "obnovení se nepodařilo – "+err.Error())
		return
	}

	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:     service.AuditDuplicateRestore,
		TargetType: service.AuditTargetLibrary,
		Details:    map[string]any{"key": key},
	})

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/admin/library/merge  (admin)
//
// Náhled sloučení: které knihy scanner založil vícekrát. Nic nemění.
func (h *AdminHandler) MergePlan(w http.ResponseWriter, r *http.Request) {
	plan, err := h.scanner.PlanMerge(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "library.check_failed",
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
			writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný vstup – "+err.Error())
			return
		}
	}

	result, err := h.scanner.Merge(r.Context(), body.Targets)
	if errors.Is(err, scanner.ErrScanRunning) {
		writeError(w, http.StatusConflict, "library.scan_running", "kontrola knihovny právě běží, zkuste to za chvíli")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "library.merge_failed", "sloučení knih selhalo – "+err.Error())
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
