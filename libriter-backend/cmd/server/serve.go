package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"libriter/internal/api/handler"
	"libriter/internal/api/middleware"
	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/metadata"
	"libriter/internal/metadata/cbdb"
	"libriter/internal/metadata/databazeknih"
	"libriter/internal/metadata/googlebooks"
	"libriter/internal/metadata/openlibrary"
	"libriter/internal/scanner"
	"libriter/internal/service"
	"libriter/internal/storage"
	"libriter/internal/web"
)

// runServe spustí HTTP server, scanner a obsluhu webového rozhraní.
func runServe() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("konfigurace: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sqlDB, err := db.Open(ctx, cfg.DB)
	if err != nil {
		return fmt.Errorf("databáze: %w", err)
	}
	defer sqlDB.Close()

	slog.Info("databáze otevřena", "path", cfg.DB.Path)

	// Kontext života aplikace - zruší se při shutdown
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// --- dependency injection ---
	store := storage.New(sqlDB)

	// --- scanner ---
	scn := scanner.New(cfg.Storage.AudioRoot, cfg.Storage.CoverRoot, store)
	scn.Start(appCtx)

	// Zdroje metadat musí vzniknout dřív než služby, které je používají –
	// stahování fotek autorů si přes ně ověřuje povolené adresy.
	metadataChain, dkClient := buildMetadata(cfg.Metadata)

	authSvc := service.NewAuth(store, cfg.JWT)
	userSvc := service.NewUser(store)
	bookSvc := service.NewBook(store)
	authorSvc := service.NewAuthor(store)
	authorImageSvc := service.NewAuthorImage(store, metadataChain, cfg.Storage.AuthorImageRoot)
	seriesSvc := service.NewSeries(store)

	authH := handler.NewAuth(authSvc)
	userH := handler.NewUser(userSvc)
	bookH := handler.NewBook(bookSvc, cfg.Storage.CoverRoot)
	authorH := handler.NewAuthor(authorSvc, cfg.Storage.AuthorImageRoot, authorImageSvc)
	seriesH := handler.NewSeries(seriesSvc)
	metadataH := handler.NewMetadata(metadataChain, dkClient)

	// --- router ---
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)

	r.Get("/health", handler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		// Neznámé API cesty musí vracet JSON, ne index.html z SPA handleru níže.
		r.NotFound(handler.NotFoundJSON)
		r.MethodNotAllowed(handler.MethodNotAllowedJSON)

		// --- auth (bez přihlášení) ---
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

		// --- obálky (bez přihlášení) ---
		// <img> neumí poslat hlavičku Authorization; ochranou je neuhodnutelné UUID knihy.
		r.Get("/books/{id}/cover", bookH.Cover)
		r.Head("/books/{id}/cover", bookH.Cover)
		r.Get("/authors/{id}/image", authorH.Image)
		r.Head("/authors/{id}/image", authorH.Image)

		// --- chráněné endpointy ---
		r.Group(func(r chi.Router) {
			r.Use(middleware.Authenticate(authSvc))

			// Uživatelé - vlastní profil
			r.Get("/users/{id}", userH.Get)
			r.Put("/users/{id}", userH.Update)
			r.Put("/users/{id}/password", userH.ChangePassword)

			// Uživatelé - pouze admin
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Get("/users", userH.List)
				r.Delete("/users/{id}", userH.Delete)
				r.Put("/users/{id}/role", userH.SetRole)
			})

			// Knihy / autoři / série - čtení (reader+)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("reader"))
				r.Get("/books", bookH.List)
				r.Get("/books/{id}", bookH.Get)
				r.Get("/authors", authorH.List)
				r.Get("/authors/{id}", authorH.Get)
				r.Get("/series", seriesH.List)
				r.Get("/series/{id}", seriesH.Get)
			})

			// Knihy / autoři / série - zápis (editor+)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("editor"))
				r.Post("/books", bookH.Create)
				r.Put("/books/{id}", bookH.Update)
				r.Patch("/books/{id}", bookH.Patch) // částečná aktualizace (webové rozhraní)
				r.Post("/authors", authorH.Create)
				r.Put("/authors/{id}", authorH.Update)
				r.Put("/authors/{id}/image", authorH.SetImage) // stáhne fotku ze zdroje
				r.Delete("/authors/{id}/image", authorH.DeleteImage)
				r.Post("/series", seriesH.Create)
				r.Put("/series/{id}", seriesH.Update)
			})

			// Mazání (admin)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("admin"))
				r.Delete("/books/{id}", bookH.Delete)
				r.Delete("/authors/{id}", authorH.Delete)
				r.Delete("/series/{id}", seriesH.Delete)
			})

			// Metadata knih - zdroje a jejich pořadí řídí METADATA_PROVIDERS (editor+)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("editor"))
				r.Get("/metadata/sources", metadataH.Sources) // pořadí zdrojů
				r.Get("/metadata/search", metadataH.Search)   // ?q=<dotaz>
				r.Get("/metadata/author/search", metadataH.SearchAuthors)
				r.Get("/metadata/author", metadataH.FetchAuthorByURL) // ?url=<url>
				r.Get("/metadata/book/{id}", metadataH.FetchByID)     // dle DK ID
				r.Get("/metadata/book", metadataH.FetchByURL)         // ?url=<url>
			})
		})
	})

	// Webové rozhraní (SPA) - vše, co není /health ani /api/v1/*.
	// Statické cesty mají v chi přednost před wildcardem, takže API zůstává nedotčené.
	r.Handle("/*", web.Handler())

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

// buildMetadata sestaví řetězec zdrojů metadat podle konfigurace. Vrací
// zároveň klienta databazeknih.cz (nebo nil), protože endpoint /metadata/book/{id}
// pracuje s číselným ID, které je pro tento zdroj specifické.
func buildMetadata(cfg config.MetadataConfig) (*metadata.Chain, *databazeknih.Client) {
	var dkClient *databazeknih.Client

	factories := map[string]metadata.Factory{
		"databazeknih": func() metadata.Provider {
			dkClient = databazeknih.NewClient()
			return dkClient
		},
		"cbdb":        func() metadata.Provider { return cbdb.NewClient() },
		"openlibrary": func() metadata.Provider { return openlibrary.NewClient() },
		"googlebooks": func() metadata.Provider { return googlebooks.NewClient(cfg.GoogleBooksAPIKey) },
	}

	chain := metadata.BuildChain(cfg.Providers, factories)
	slog.Info("zdroje metadat", "pořadí", chain.Providers())
	return chain, dkClient
}
