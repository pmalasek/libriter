package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	corehandler "mediater/internal/core/handler"
	"mediater/internal/core/httpx"
	"mediater/internal/core/middleware"
	"mediater/internal/core/plugin"
	"mediater/internal/core/product"
	"mediater/internal/core/service"
	"mediater/internal/core/storage"
	"mediater/internal/core/version"
	"mediater/internal/core/web"
	"mediater/internal/libriter"
	"mediater/internal/sceniter"
)

// products vrací produkty, které binárka obsahuje, v pořadí migrací a
// registrace rout.
func products() []product.Product {
	return []product.Product{libriter.New(), sceniter.New()}
}

// runServe spustí HTTP server: jádro (účty, přihlášení, administrace uživatelů)
// a nad ním všechny produkty.
func runServe() error {
	startedAt := time.Now()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("konfigurace: %w", err)
	}
	if err := cfg.Storage.EnsureDirs(); err != nil {
		return fmt.Errorf("datové adresáře: %w", err)
	}

	slog.Info("mediater", "verze", version.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sqlDB, err := db.Open(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("databáze: %w", err)
	}
	defer sqlDB.Close()

	slog.Info("databáze otevřena", "path", cfg.DB.Path)

	prods := products()
	for _, p := range prods {
		if err := db.MigrateModule(ctx, sqlDB, p.Name(), p.Migrations()); err != nil {
			return fmt.Errorf("databáze: %w", err)
		}
	}

	// Kontext života aplikace - zruší se při shutdown
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// --- jádro ---
	coreStore := storage.New(sqlDB)
	authSvc := service.NewAuth(coreStore, cfg.JWT)

	// Pluginy metadat musí být načtené dřív než produkty – ty si z nich
	// skládají seznamy zdrojů.
	pluginSettings := plugin.NewSettings(coreStore, storage.ErrNotFound)
	pluginHost := plugin.NewHost(cfg.PluginDir)
	pluginHost.SetConfigSource(pluginSettings)
	if err := pluginHost.Discover(ctx); err != nil {
		slog.Warn("pluginy metadat", "err", err)
	}
	slog.Info("pluginy metadat", "dir", cfg.PluginDir, "počet", len(pluginHost.Plugins()))
	deps := product.Deps{
		Ctx:            appCtx,
		Cfg:            cfg,
		DB:             sqlDB,
		Auth:           authSvc,
		Users:          service.NewUser(coreStore, authSvc),
		Registration:   service.NewRegistration(coreStore),
		Audit:          service.NewAudit(coreStore),
		Plugins:        pluginHost,
		PluginSettings: pluginSettings,
		StartedAt:      startedAt,
	}

	for _, p := range prods {
		if err := p.Init(deps); err != nil {
			return fmt.Errorf("%s: %w", p.Name(), err)
		}
	}

	r := newRouter(cfg, deps, prods)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	srvErr := make(chan error, 1)
	go func() {
		slog.Info("server spuštěn", "addr", addr, "env", cfg.Server.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			srvErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-srvErr:
		return fmt.Errorf("server: %w", err)
	case <-quit:
	}

	slog.Info("ukončování serveru...")
	appCancel() // zastaví scanner a ostatní goroutiny
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// newRouter sestaví router: /health, API jádra a produktů pod /api/v1 a
// webová rozhraní produktů vybíraná podle hostname.
func newRouter(cfg *config.Config, deps product.Deps, prods []product.Product) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)

	r.Get("/health", corehandler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		// Neznámé API cesty musí vracet JSON, ne index.html z SPA handleru níže.
		r.NotFound(httpx.NotFoundJSON)
		r.MethodNotAllowed(httpx.MethodNotAllowedJSON)

		corehandler.RegisterRoutes(r, deps.Auth, corehandler.Handlers{
			Auth:    corehandler.NewAuth(deps.Auth, deps.Registration),
			Users:   corehandler.NewUser(deps.Users, deps.Audit),
			Admin:   corehandler.NewAdmin(deps.Users, deps.Registration, deps.Audit),
			Plugins: corehandler.NewPlugins(deps.Plugins, deps.PluginSettings, deps.Audit),
		})
		for _, p := range prods {
			p.RegisterRoutes(r)
		}
	})

	// Webová rozhraní (SPA) - vše, co není /health ani /api/v1/*. Statické
	// cesty mají v chi přednost před wildcardem, takže API zůstává nedotčené.
	r.Handle("/*", webMux(cfg, prods))
	return r
}

// webMux vybere SPA podle hostu: <PRODUKT>_HOST (seznam oddělený čárkami)
// přiřadí host produktu, ostatní hosty dostanou DEFAULT_PRODUCT.
func webMux(cfg *config.Config, prods []product.Product) http.Handler {
	handlers := make(map[string]http.Handler, len(prods))
	for _, p := range prods {
		handlers[p.Name()] = web.New(p.Web(), p.Name())
	}

	fallback, ok := handlers[cfg.DefaultProduct]
	if !ok {
		slog.Warn("DEFAULT_PRODUCT neodpovídá žádnému produktu", "value", cfg.DefaultProduct)
		fallback = handlers[prods[0].Name()]
	}

	mux := web.NewHostMux(fallback)
	for _, p := range prods {
		hosts := cfg.EnvList(strings.ToUpper(p.Name())+"_HOST", nil)
		if len(hosts) > 0 {
			mux.Handle(hosts, handlers[p.Name()])
			slog.Info("webové rozhraní", "produkt", p.Name(), "hosty", hosts)
		}
	}
	return mux
}
