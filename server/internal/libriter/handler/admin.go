package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/scanner"
	"mediater/internal/libriter/service"

	"github.com/google/uuid"
)

// libraryScanner je to, co administrace potřebuje od scanneru. Rozhraní
// místo konkrétního typu drží handler testovatelný bez souborového systému.
type libraryScanner interface {
	Status() scanner.Status
	Rescan() error
	PlanRepair(ctx context.Context) (scanner.RepairPlan, error)
	Repair(ctx context.Context, force bool) (scanner.RepairResult, error)
	PlanDuplicates(ctx context.Context) (scanner.DuplicateReport, error)
	DismissDuplicate(ctx context.Context, groupKey string, actorID *uuid.UUID) (scanner.DuplicateGroup, error)
	RestoreDuplicate(ctx context.Context, groupKey string) error
	PlanMerge(ctx context.Context) (scanner.MergePlan, error)
	Merge(ctx context.Context, targets []uuid.UUID) (scanner.MergeResult, error)
}

// AdminHandler obsluhuje endpointy pod /api/v1/admin. Skupinu chrání
// RequireRole("admin"), handlery tedy roli znovu neověřují.
type AdminHandler struct {
	users     *service.UserService
	settings  *service.SettingsService
	registry  *metadata.Registry
	scanner   libraryScanner
	system    *service.SystemService
	audit     *service.AuditService
	listening *service.ListeningService
}

func NewAdmin(
	users *service.UserService,
	settings *service.SettingsService,
	registry *metadata.Registry,
	scn libraryScanner,
	system *service.SystemService,
	audit *service.AuditService,
	listening *service.ListeningService,
) *AdminHandler {
	return &AdminHandler{
		users:     users,
		settings:  settings,
		registry:  registry,
		scanner:   scn,
		system:    system,
		audit:     audit,
		listening: listening,
	}
}

// GET /api/v1/admin/stats  (admin)
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.system.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání statistik")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// GET /api/v1/admin/system  (admin)
func (h *AdminHandler) System(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.system.Info())
}

// GET /api/v1/admin/listening  (admin)
//
// Přehled poslechu všech uživatelů. Čtení se neaudituje – zapisují se jen
// zásahy, které něco mění.
func (h *AdminHandler) Listening(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.listening.Overview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání poslechů")
		return
	}
	writeJSON(w, http.StatusOK, summaries)
}

// GET /api/v1/admin/listening/{id}  (admin)
//
// Poslechy, stav knih a deník jednoho uživatele.
func (h *AdminHandler) ListeningUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	detail, err := h.listening.UserDetail(r.Context(), id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeError(w, http.StatusNotFound, "user.not_found", "uživatel nenalezen")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání poslechů")
	default:
		writeJSON(w, http.StatusOK, detail)
	}
}

func intParam(r *http.Request, name string, def int64) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
