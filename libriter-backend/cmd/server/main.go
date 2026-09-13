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
	"libriter/internal/metadata/databazeknih"
	"libriter/internal/scanner"
	"libriter/internal/service"
	"libriter/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("konfigurace", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sqlDB, err := db.Open(ctx, cfg.DB)
	if err != nil {
		slog.Error("databáze", "err", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	slog.Info("databáze otevřena", "path", cfg.DB.Path)

	// Kontext života aplikace - zruší se při shutdown
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	// --- dependency injection ---
	store := storage.New(sqlDB)

	// --- scanner ---
	scn := scanner.New(cfg.Storage.AudioRoot, store)
	scn.Start(appCtx)

	authSvc := service.NewAuth(store, cfg.JWT)
	userSvc := service.NewUser(store)
	bookSvc := service.NewBook(store)
	authorSvc := service.NewAuthor(store)
	seriesSvc := service.NewSeries(store)

	authH := handler.NewAuth(authSvc)
	userH := handler.NewUser(userSvc)
	bookH := handler.NewBook(bookSvc)
	authorH := handler.NewAuthor(authorSvc)
	seriesH := handler.NewSeries(seriesSvc)
	metadataH := handler.NewMetadata(databazeknih.NewClient())

	// --- router ---
	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)

	r.Get("/health", handler.Health)

	r.Route("/api/v1", func(r chi.Router) {
		// --- auth (bez přihlášení) ---
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)

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
				r.Post("/authors", authorH.Create)
				r.Put("/authors/{id}", authorH.Update)
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

			// Metadata scraper - databazeknih.cz (editor+)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireRole("editor"))
				r.Get("/metadata/search", metadataH.Search)       // ?q=<dotaz>
				r.Get("/metadata/book/{id}", metadataH.FetchByID) // dle DK ID
				r.Get("/metadata/book", metadataH.FetchByURL)     // ?url=<url>
			})
		})
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("server spuštěn", "addr", addr, "env", cfg.Server.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("ukončování serveru...")
	appCancel() // zastaví scanner a ostatní goroutiny
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}
