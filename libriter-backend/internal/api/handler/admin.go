package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"libriter/internal/api/middleware"
	"libriter/internal/metadata"
	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/service"

	"github.com/google/uuid"
)

// libraryScanner je to, co administrace potřebuje od scanneru. Rozhraní
// místo konkrétního typu drží handler testovatelný bez souborového systému.
type libraryScanner interface {
	Status() scanner.Status
	Rescan() error
	PlanRepair(ctx context.Context) (scanner.RepairPlan, error)
	Repair(ctx context.Context) (scanner.RepairResult, error)
	PlanMerge(ctx context.Context) (scanner.MergePlan, error)
	Merge(ctx context.Context, targets []uuid.UUID) (scanner.MergeResult, error)
}

// AdminHandler obsluhuje endpointy pod /api/v1/admin. Skupinu chrání
// RequireRole("admin"), handlery tedy roli znovu neověřují.
type AdminHandler struct {
	users    *service.UserService
	settings *service.SettingsService
	registry *metadata.Registry
	scanner  libraryScanner
	system   *service.SystemService
	audit    *service.AuditService
}

func NewAdmin(
	users *service.UserService,
	settings *service.SettingsService,
	registry *metadata.Registry,
	scn libraryScanner,
	system *service.SystemService,
	audit *service.AuditService,
) *AdminHandler {
	return &AdminHandler{
		users:    users,
		settings: settings,
		registry: registry,
		scanner:  scn,
		system:   system,
		audit:    audit,
	}
}

// Stránkování auditu; strop musí odpovídat stropu v service.AuditService.
const (
	auditPageSize    = 50
	auditMaxPageSize = 200
)

// actorID vrátí ID přihlášeného administrátora pro zápis do audit logu.
func actorID(r *http.Request) uuid.UUID {
	id, _ := middleware.UserIDFromCtx(r.Context())
	return id
}

// POST /api/v1/admin/users  (admin)
//
// Založí účet s libovolnou rolí. Veřejná registrace roli neurčuje, takže
// editora i dalšího admina umí vyrobit jen tudy (nebo přes CLI).
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
		Role        string `json:"role"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "neplatný formát požadavku")
		return
	}

	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Email = strings.TrimSpace(req.Email)
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role == "" {
		req.Role = model.RoleReader
	}

	if req.DisplayName == "" || req.Email == "" || len(req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest, "display_name, email a heslo (min. 8 znaků) jsou povinné")
		return
	}
	if _, ok := model.RoleLevel[req.Role]; !ok {
		writeError(w, http.StatusBadRequest, "neznámá role")
		return
	}

	u, err := h.users.Create(r.Context(), req.DisplayName, req.Email, req.Password, req.Role)
	if errors.Is(err, service.ErrEmailTaken) {
		writeError(w, http.StatusConflict, "email je již použit")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při vytváření uživatele")
		return
	}

	h.audit.Record(r.Context(), actorID(r), service.AuditEvent{
		Action:      service.AuditUserCreate,
		TargetType:  service.AuditTargetUser,
		TargetID:    u.ID.String(),
		TargetLabel: u.Email,
		Details:     map[string]string{"role": u.Role},
	})

	writeJSON(w, http.StatusCreated, u)
}

// GET /api/v1/admin/stats  (admin)
func (h *AdminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.system.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání statistik")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// GET /api/v1/admin/system  (admin)
func (h *AdminHandler) System(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.system.Info())
}

// GET /api/v1/admin/audit?limit=50&before=<id>  (admin)
//
// Stránkuje se kurzorem (id posledního záznamu), ne offsetem – log mezitím
// roste a offset by záznamy přeskakoval.
func (h *AdminHandler) Audit(w http.ResponseWriter, r *http.Request) {
	limit, err := intParam(r, "limit", auditPageSize)
	if err != nil || limit <= 0 {
		writeError(w, http.StatusBadRequest, "parametr limit musí být kladné číslo")
		return
	}
	// Strop držíme i tady, aby kurzor níž odpovídal skutečné velikosti stránky.
	if limit > auditMaxPageSize {
		limit = auditMaxPageSize
	}
	before, err := intParam(r, "before", 0)
	if err != nil || before < 0 {
		writeError(w, http.StatusBadRequest, "parametr before musí být nezáporné číslo")
		return
	}

	entries, err := h.audit.List(r.Context(), int(limit), before)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání auditu")
		return
	}

	// Kurzor posíláme jen tehdy, když stránka došla plná – jinak jsme na konci.
	var nextBefore *int64
	if len(entries) > 0 && len(entries) == int(limit) {
		last := entries[len(entries)-1].ID
		nextBefore = &last
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       entries,
		"next_before": nextBefore,
	})
}

func intParam(r *http.Request, name string, def int64) (int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
