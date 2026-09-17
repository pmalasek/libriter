package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"libriter/internal/api/middleware"
	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/metadata"
	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// fakeScanner nahrazuje skutečný scanner – testy admin rozhraní nepotřebují
// souborový systém ani běžící goroutiny.
type fakeScanner struct {
	status       scanner.Status
	rescanErr    error
	rescans      int
	plan         scanner.RepairPlan
	repairErr    error
	repairCall   int
	mergePlan    scanner.MergePlan
	mergeErr     error
	mergeCall    int
	mergeTargets []uuid.UUID
}

func (f *fakeScanner) Status() scanner.Status { return f.status }

func (f *fakeScanner) Rescan() error {
	if f.rescanErr != nil {
		return f.rescanErr
	}
	f.rescans++
	f.status.Running = true
	return nil
}

func (f *fakeScanner) PlanRepair(context.Context) (scanner.RepairPlan, error) {
	return f.plan, nil
}

func (f *fakeScanner) Repair(context.Context) (scanner.RepairResult, error) {
	if f.repairErr != nil {
		return scanner.RepairResult{}, f.repairErr
	}
	f.repairCall++
	return scanner.RepairResult{Plan: f.plan, DeletedChapters: 3, DeletedBooks: 1}, nil
}

func (f *fakeScanner) PlanMerge(context.Context) (scanner.MergePlan, error) {
	return f.mergePlan, nil
}

func (f *fakeScanner) Merge(_ context.Context, targets []uuid.UUID) (scanner.MergeResult, error) {
	if f.mergeErr != nil {
		return scanner.MergeResult{}, f.mergeErr
	}
	f.mergeCall++
	f.mergeTargets = targets
	plan := scanner.FilterPlan(f.mergePlan, targets)
	return scanner.MergeResult{Plan: plan, MergedBooks: 1, MovedChapters: 23}, nil
}

type adminTestEnv struct {
	router  http.Handler
	store   *storage.Store
	users   *service.UserService
	auth    *service.AuthService
	audit   *service.AuditService
	scanner *fakeScanner
	// audioRoot je kořen, pod který testy zapisují audio soubory kapitol.
	audioRoot string
}

// newAdminTestEnv postaví router se stejnými skupinami jako server: veřejná
// registrace, vlastní profil, admin skupina a /admin.
func newAdminTestEnv(t *testing.T) *adminTestEnv {
	t.Helper()

	conn, err := db.Open(context.Background(), config.DBConfig{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	store := storage.New(conn)
	authSvc := service.NewAuth(store, config.JWTConfig{Secret: "test-secret", ExpiryHours: time.Hour})
	userSvc := service.NewUser(store, authSvc)
	auditSvc := service.NewAudit(store)

	registry := metadata.NewRegistry(map[string]metadata.Factory{})
	cfg := &config.Config{Server: config.ServerConfig{Env: "test"}}
	settingsSvc := service.NewSettings(store, config.MetadataConfig{}, registry.KnownNames())
	systemSvc := service.NewSystem(store, cfg, time.Now())

	scn := &fakeScanner{
		plan:      scanner.RepairPlan{Rescan: []scanner.RepairBook{}, Duplicates: []scanner.RepairBook{}},
		mergePlan: scanner.MergePlan{Groups: []scanner.MergeGroup{}},
	}
	bookSvc := service.NewBook(store)
	audioRoot := t.TempDir()

	adminH := NewAdmin(userSvc, settingsSvc, registry, scn, systemSvc, auditSvc, service.NewListening(store))
	authH := NewAuth(authSvc, settingsSvc)
	userH := NewUser(userSvc, auditSvc)
	bookH := NewBook(bookSvc, t.TempDir(), auditSvc)
	sessionH := NewPlaySession(service.NewPlaySession(store))
	audioH := NewAudio(bookSvc, authSvc, audioRoot)

	r := chi.NewRouter()
	r.Get("/auth/config", authH.Config)
	r.Post("/auth/register", authH.Register)
	// Audio se streamuje mimo Authenticate – token je v adrese.
	r.Get("/chapters/{id}/audio", audioH.Stream)
	r.Head("/chapters/{id}/audio", audioH.Stream)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(authSvc))
		r.Get("/auth/stream-token", authH.StreamToken)
		r.Post("/auth/mobile-token", authH.MobileToken)
		r.Put("/users/{id}/password", userH.ChangePassword)
		r.Get("/users/{id}", userH.Get)
		r.Put("/users/{id}", userH.Update)
		r.Put("/users/{id}/appearance", userH.UpdateAppearance)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleReader))
			r.Get("/books/progress", bookH.ListProgress)
			r.Get("/books/{id}/chapters", bookH.ListChapters)
			r.Put("/books/{id}/progress", bookH.SetProgress)
			r.Delete("/books/{id}/progress", bookH.ResetProgress)
			r.Get("/sessions", sessionH.List)
			r.Post("/sessions", sessionH.Create)
			// Dávka pozic z offline zařízení; musí být nad /sessions/{id},
			// ať se "sync" nečte jako UUID session.
			r.Post("/sessions/sync", sessionH.Sync)
			r.Get("/sessions/{id}", sessionH.Get)
			r.Put("/sessions/{id}/position", sessionH.SavePosition)
			r.Post("/sessions/{id}/items", sessionH.AddItems)
			r.Delete("/sessions/{id}", sessionH.Delete)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleEditor))
			r.Put("/books/{id}/chapters/order", bookH.ReorderChapters)
		})
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleAdmin))
			r.Delete("/users/{id}", userH.Delete)
			r.Put("/users/{id}/role", userH.SetRole)
		})
		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleAdmin))
			r.Post("/users", adminH.CreateUser)
			r.Get("/stats", adminH.Stats)
			r.Get("/system", adminH.System)
			r.Get("/audit", adminH.Audit)
			r.Get("/settings/registration", adminH.RegistrationSettings)
			r.Put("/settings/registration", adminH.SetRegistrationSettings)
			r.Post("/scanner/rescan", adminH.Rescan)
			r.Get("/library/repair", adminH.RepairPlan)
			r.Post("/library/repair", adminH.Repair)
			r.Get("/library/merge", adminH.MergePlan)
			r.Post("/library/merge", adminH.Merge)
			r.Get("/listening", adminH.Listening)
			r.Get("/listening/{id}", adminH.ListeningUser)
		})
	})

	return &adminTestEnv{
		router: r, store: store, users: userSvc, auth: authSvc,
		audit: auditSvc, scanner: scn, audioRoot: audioRoot,
	}
}

// login vytvoří uživatele dané role a vrátí jeho token a ID.
func (e *adminTestEnv) login(t *testing.T, email, role string) (string, string) {
	t.Helper()
	ctx := context.Background()

	u, err := e.users.Create(ctx, "Test "+role, email, "tajneheslo", role)
	if err != nil {
		t.Fatalf("Create %s: %v", role, err)
	}
	_, token, err := e.auth.Login(ctx, email, "tajneheslo")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return token, u.ID.String()
}

func (e *adminTestEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return e.serve(e.request(t, method, path, token, body))
}

// request sestaví požadavek, aniž by ho rovnou odeslal – testy, které potřebují
// vlastní hlavičku (například Range), si ji doplní a pošlou přes serve.
func (e *adminTestEnv) request(t *testing.T, method, path, token string, body ...any) *http.Request {
	t.Helper()

	var reader *bytes.Reader
	if len(body) > 0 && body[0] != nil {
		raw, err := json.Marshal(body[0])
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func (e *adminTestEnv) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("rozbalení odpovědi: %v (%s)", err, raw)
	}
}

// Administrace patří jen adminovi; editor na ni nesmí.
func TestAdminRoutesRequireAdminRole(t *testing.T) {
	env := newAdminTestEnv(t)
	env.login(t, "admin@example.com", model.RoleAdmin) // aby existoval aspoň jeden
	editorToken, _ := env.login(t, "editor@example.com", model.RoleEditor)

	for _, path := range []string{"/admin/stats", "/admin/system", "/admin/audit"} {
		if rec := env.do(t, http.MethodGet, path, editorToken, nil); rec.Code != http.StatusForbidden {
			t.Errorf("editor na %s: status = %d, chtěno 403", path, rec.Code)
		}
	}
}

func TestAdminCreateUser(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	body := map[string]string{
		"display_name": "Nový editor",
		"email":        "editor@example.com",
		"password":     "tajneheslo",
		"role":         model.RoleEditor,
	}
	rec := env.do(t, http.MethodPost, "/admin/users", adminToken, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), chtěno 201", rec.Code, rec.Body.String())
	}

	var created model.User
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("odpověď: %v", err)
	}
	if created.Role != model.RoleEditor {
		t.Errorf("role = %q, chtěno editor", created.Role)
	}

	// Stejný e-mail podruhé neprojde.
	if rec := env.do(t, http.MethodPost, "/admin/users", adminToken, body); rec.Code != http.StatusConflict {
		t.Errorf("duplicitní e-mail: status = %d, chtěno 409", rec.Code)
	}

	// Krátké heslo neprojde.
	short := map[string]string{
		"display_name": "Krátké heslo", "email": "kratke@example.com", "password": "krat", "role": model.RoleReader,
	}
	if rec := env.do(t, http.MethodPost, "/admin/users", adminToken, short); rec.Code != http.StatusBadRequest {
		t.Errorf("krátké heslo: status = %d, chtěno 400", rec.Code)
	}

	// Akce se zapsala do auditu.
	entries, err := env.audit.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(entries) == 0 || entries[0].Action != service.AuditUserCreate {
		t.Errorf("audit = %+v, chtěn záznam user.create", entries)
	}
}

func TestRegistrationCanBeDisabled(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	// Výchozí stav: registrace zapnutá.
	rec := env.do(t, http.MethodGet, "/auth/config", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("auth/config: status = %d", rec.Code)
	}
	var cfg struct {
		RegistrationEnabled bool   `json:"registration_enabled"`
		DefaultRole         string `json:"default_role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("odpověď: %v", err)
	}
	if !cfg.RegistrationEnabled || cfg.DefaultRole != model.RoleReader {
		t.Errorf("výchozí konfigurace = %+v, chtěna zapnutá registrace s rolí reader", cfg)
	}

	rec = env.do(t, http.MethodPut, "/admin/settings/registration", adminToken,
		map[string]any{"enabled": false, "default_role": model.RoleEditor})
	if rec.Code != http.StatusOK {
		t.Fatalf("uložení nastavení: status = %d (%s)", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodPost, "/auth/register", "", map[string]string{
		"display_name": "Zvědavec", "email": "novy@example.com", "password": "tajneheslo",
	})
	if rec.Code != http.StatusForbidden {
		t.Errorf("registrace při vypnutí: status = %d, chtěno 403", rec.Code)
	}

	rec = env.do(t, http.MethodGet, "/auth/config", "", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("odpověď: %v", err)
	}
	if cfg.RegistrationEnabled {
		t.Error("auth/config hlásí zapnutou registraci, ačkoliv je vypnutá")
	}

	// Role admin jako výchozí pro registraci neprojde.
	rec = env.do(t, http.MethodPut, "/admin/settings/registration", adminToken,
		map[string]any{"enabled": true, "default_role": model.RoleAdmin})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("role admin: status = %d, chtěno 400", rec.Code)
	}
}

// Registrovaný účet dostane roli z nastavení, ne pevně reader.
func TestRegisterUsesDefaultRoleFromSettings(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	if rec := env.do(t, http.MethodPut, "/admin/settings/registration", adminToken,
		map[string]any{"enabled": true, "default_role": model.RoleEditor}); rec.Code != http.StatusOK {
		t.Fatalf("uložení nastavení: status = %d", rec.Code)
	}

	rec := env.do(t, http.MethodPost, "/auth/register", "", map[string]string{
		"display_name": "Nováček", "email": "novacek@example.com", "password": "tajneheslo",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("registrace: status = %d (%s)", rec.Code, rec.Body.String())
	}

	var resp struct {
		User model.User `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("odpověď: %v", err)
	}
	if resp.User.Role != model.RoleEditor {
		t.Errorf("role = %q, chtěno editor", resp.User.Role)
	}
}

func TestAdminCannotDeleteSelfOrLastAdmin(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, adminID := env.login(t, "admin@example.com", model.RoleAdmin)

	if rec := env.do(t, http.MethodDelete, "/users/"+adminID, adminToken, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("smazání vlastního účtu: status = %d, chtěno 400", rec.Code)
	}

	// Snížení role poslednímu adminovi (i sobě) musí skončit 409.
	rec := env.do(t, http.MethodPut, "/users/"+adminID+"/role", adminToken,
		map[string]string{"role": model.RoleReader})
	if rec.Code != http.StatusConflict {
		t.Errorf("snížení role poslednímu adminovi: status = %d (%s), chtěno 409", rec.Code, rec.Body.String())
	}
}

func TestAdminDeletesOtherUserAndRecordsAudit(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)
	_, readerID := env.login(t, "reader@example.com", model.RoleReader)

	if rec := env.do(t, http.MethodDelete, "/users/"+readerID, adminToken, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("smazání čtenáře: status = %d (%s)", rec.Code, rec.Body.String())
	}

	entries, err := env.audit.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(entries) == 0 || entries[0].Action != service.AuditUserDelete {
		t.Fatalf("audit = %+v, chtěn záznam user.delete", entries)
	}
	if entries[0].TargetLabel != "reader@example.com" {
		t.Errorf("popisek cíle = %q, chtěno reader@example.com", entries[0].TargetLabel)
	}
}

func TestAdminScannerAndRepair(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	if rec := env.do(t, http.MethodPost, "/admin/scanner/rescan", adminToken, nil); rec.Code != http.StatusAccepted {
		t.Errorf("rescan: status = %d (%s), chtěno 202", rec.Code, rec.Body.String())
	}
	if env.scanner.rescans != 1 {
		t.Errorf("scanner spuštěn %d×, chtěno 1×", env.scanner.rescans)
	}

	// Běžící scan vrací 409, ne 500.
	env.scanner.rescanErr = scanner.ErrScanRunning
	if rec := env.do(t, http.MethodPost, "/admin/scanner/rescan", adminToken, nil); rec.Code != http.StatusConflict {
		t.Errorf("rescan během běhu: status = %d, chtěno 409", rec.Code)
	}

	if rec := env.do(t, http.MethodGet, "/admin/library/repair", adminToken, nil); rec.Code != http.StatusOK {
		t.Errorf("náhled opravy: status = %d", rec.Code)
	}

	env.scanner.plan = scanner.RepairPlan{
		Rescan:     []scanner.RepairBook{{Title: "Ze života hmyzu", FilePath: "capek/hmyz"}},
		Duplicates: []scanner.RepairBook{},
	}
	rec := env.do(t, http.MethodPost, "/admin/library/repair", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("oprava: status = %d (%s)", rec.Code, rec.Body.String())
	}
	if env.scanner.repairCall != 1 {
		t.Errorf("oprava volána %d×, chtěno 1×", env.scanner.repairCall)
	}

	entries, err := env.audit.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(entries) == 0 || entries[0].Action != service.AuditLibraryRepair {
		t.Errorf("audit = %+v, chtěn záznam library.repair_apply", entries)
	}
}

func TestAdminMergeBooks(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	if rec := env.do(t, http.MethodGet, "/admin/library/merge", adminToken, nil); rec.Code != http.StatusOK {
		t.Errorf("náhled sloučení: status = %d (%s)", rec.Code, rec.Body.String())
	}

	// Prázdný plán se do auditu nezapisuje – nic se nestalo.
	if rec := env.do(t, http.MethodPost, "/admin/library/merge", adminToken, nil); rec.Code != http.StatusOK {
		t.Fatalf("sloučení prázdného plánu: status = %d (%s)", rec.Code, rec.Body.String())
	}
	entries, err := env.audit.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for _, e := range entries {
		if e.Action == service.AuditLibraryMerge {
			t.Error("prázdné sloučení se zapsalo do auditu")
		}
	}

	target := uuid.New()
	env.scanner.mergePlan = scanner.MergePlan{Groups: []scanner.MergeGroup{{
		Target:  scanner.MergeBook{ID: target, Title: "Písečná bouře", FilePath: "rollins/pisecna-boure"},
		Sources: []scanner.MergeBook{{Title: "Písečná bouře", FilePath: "rollins/pisecna-boure"}},
	}}}

	// Výběr skupin: klient posílá jen ID cílů, plán si server počítá sám.
	body := map[string][]string{"targets": {target.String()}}
	if rec := env.do(t, http.MethodPost, "/admin/library/merge", adminToken, body); rec.Code != http.StatusOK {
		t.Fatalf("sloučení: status = %d (%s)", rec.Code, rec.Body.String())
	}
	if env.scanner.mergeCall != 2 {
		t.Errorf("sloučení voláno %d×, chtěno 2×", env.scanner.mergeCall)
	}
	if len(env.scanner.mergeTargets) != 1 || env.scanner.mergeTargets[0] != target {
		t.Errorf("vybrané cíle = %v, chtěno [%s]", env.scanner.mergeTargets, target)
	}

	entries, err = env.audit.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(entries) == 0 || entries[0].Action != service.AuditLibraryMerge {
		t.Errorf("audit = %+v, chtěn záznam library.merge_books", entries)
	}

	// Běžící scan vrací 409, ne 500.
	env.scanner.mergeErr = scanner.ErrScanRunning
	if rec := env.do(t, http.MethodPost, "/admin/library/merge", adminToken, nil); rec.Code != http.StatusConflict {
		t.Errorf("sloučení během scanu: status = %d, chtěno 409", rec.Code)
	}
}

func TestAdminAuditPagination(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	for i := 0; i < 3; i++ {
		env.do(t, http.MethodPost, "/admin/scanner/rescan", adminToken, nil)
		env.scanner.rescanErr = nil
	}

	rec := env.do(t, http.MethodGet, "/admin/audit?limit=2", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit: status = %d", rec.Code)
	}

	var page struct {
		Items      []model.AuditEntry `json:"items"`
		NextBefore *int64             `json:"next_before"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("odpověď: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("položek = %d, chtěny 2", len(page.Items))
	}
	if page.NextBefore == nil || *page.NextBefore != page.Items[1].ID {
		t.Errorf("next_before = %v, chtěno id posledního záznamu", page.NextBefore)
	}

	if rec := env.do(t, http.MethodGet, "/admin/audit?limit=0", adminToken, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("limit=0: status = %d, chtěno 400", rec.Code)
	}
}

func TestAdminStatsAndSystem(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	rec := env.do(t, http.MethodGet, "/admin/stats", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats: status = %d", rec.Code)
	}
	var stats storage.LibraryStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("odpověď: %v", err)
	}
	if stats.Users != 1 {
		t.Errorf("uživatelů = %d, chtěn 1", stats.Users)
	}

	rec = env.do(t, http.MethodGet, "/admin/system", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("system: status = %d", rec.Code)
	}
	var info service.SystemInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("odpověď: %v", err)
	}
	if info.Version == "" || info.GoVersion == "" {
		t.Errorf("systémové info = %+v, chtěny vyplněné verze", info)
	}
}
