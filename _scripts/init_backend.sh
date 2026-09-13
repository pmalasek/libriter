#!/usr/bin/env bash
# =============================================================
#  Libriter - inicializace backend projektu
#  Spuštění: bash libriter_backend_init.sh
# =============================================================

set -e

PROJECT="libriter-backend"
MODULE="libriter"

echo "→ Vytvářím strukturu projektu: $PROJECT"
mkdir -p "$PROJECT"
cd "$PROJECT"

# -------------------------------------------------------------
#  Adresářová struktura
# -------------------------------------------------------------

mkdir -p \
  cmd/server \
  internal/api/handler \
  internal/api/middleware \
  internal/config \
  internal/db \
  internal/model \
  internal/service \
  internal/storage \
  internal/db/migrations \
  scripts

echo "→ Adresáře vytvořeny"

# -------------------------------------------------------------
#  Go modul + závislosti
# -------------------------------------------------------------

go mod init "$MODULE"

go get \
  github.com/go-chi/chi/v5 \
  github.com/go-chi/chi/v5/middleware \
  modernc.org/sqlite \
  golang.org/x/text \
  github.com/golang-jwt/jwt/v5 \
  github.com/joho/godotenv \
  golang.org/x/crypto

go mod tidy

echo "→ Go závislosti staženy"

# -------------------------------------------------------------
#  .env
# -------------------------------------------------------------

cat > .env << 'ENV'
# Server
SERVER_PORT=8080
SERVER_ENV=development

# Databáze (SQLite – soubor i schéma se vytvoří automaticky při startu)
DB_PATH=./data/libriter.db

# JWT
JWT_SECRET=change-me-before-production
JWT_EXPIRY_HOURS=72

# Soubory
AUDIO_ROOT=/var/lib/libriter/audio
COVER_ROOT=/var/lib/libriter/covers
MAX_UPLOAD_MB=500
ENV

echo "→ .env vytvořen"

# -------------------------------------------------------------
#  .gitignore
# -------------------------------------------------------------

cat > .gitignore << 'GIT'
.env
*.exe
*.exe~
*.dll
*.so
*.dylib
dist/
tmp/
GIT

# -------------------------------------------------------------
#  internal/config/config.go
# -------------------------------------------------------------

cat > internal/config/config.go << 'GO'
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	DB       DBConfig
	JWT      JWTConfig
	Storage  StorageConfig
}

type ServerConfig struct {
	Port int
	Env  string
}

type DBConfig struct {
	Path string // cesta k souboru SQLite databáze
}

type JWTConfig struct {
	Secret      string
	ExpiryHours time.Duration
}

type StorageConfig struct {
	AudioRoot   string
	CoverRoot   string
	MaxUploadMB int64
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Server: ServerConfig{
			Port: envInt("SERVER_PORT", 8080),
			Env:  envStr("SERVER_ENV", "development"),
		},
		DB: DBConfig{
			Path: envStr("DB_PATH", "./data/libriter.db"),
		},
		JWT: JWTConfig{
			Secret:      envStr("JWT_SECRET", ""),
			ExpiryHours: time.Duration(envInt("JWT_EXPIRY_HOURS", 72)) * time.Hour,
		},
		Storage: StorageConfig{
			AudioRoot:   envStr("AUDIO_ROOT", "/var/lib/libriter/audio"),
			CoverRoot:   envStr("COVER_ROOT", "/var/lib/libriter/covers"),
			MaxUploadMB: int64(envInt("MAX_UPLOAD_MB", 500)),
		},
	}

	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET není nastaven")
	}

	return cfg, nil
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
GO

# -------------------------------------------------------------
#  internal/db/db.go
# -------------------------------------------------------------

cat > internal/db/db.go << 'GO'
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"libriter/internal/config"
)

func Open(ctx context.Context, cfg config.DBConfig) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("db: vytvoření adresáře: %w", err)
	}

	dsn := "file:" + cfg.Path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_time_format=sqlite"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return db, nil
}
GO

# -------------------------------------------------------------
#  internal/model/model.go
# -------------------------------------------------------------

cat > internal/model/model.go << 'GO'
package model

import (
	"time"

	"github.com/google/uuid"
)

type Author struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Bio       *string   `json:"bio,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Series struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Book struct {
	ID              uuid.UUID  `json:"id"`
	AuthorID        uuid.UUID  `json:"author_id"`
	SeriesID        *uuid.UUID `json:"series_id,omitempty"`
	SeriesPosition  *int       `json:"series_position,omitempty"`
	Title           string     `json:"title"`
	Narrator        *string    `json:"narrator,omitempty"`
	DurationSeconds int        `json:"duration_seconds"`
	FilePath        string     `json:"-"`
	CoverPath       *string    `json:"cover_path,omitempty"`
	Language        string     `json:"language"`
	Description     *string    `json:"description,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Chapter struct {
	ID                  uuid.UUID `json:"id"`
	BookID              uuid.UUID `json:"book_id"`
	Position            int       `json:"position"`
	Title               string    `json:"title"`
	StartOffsetSeconds  int       `json:"start_offset_seconds"`
	DurationSeconds     int       `json:"duration_seconds"`
}

type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PlaybackPosition struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	BookID          uuid.UUID  `json:"book_id"`
	DeviceID        *uuid.UUID `json:"device_id,omitempty"`
	PositionSeconds int        `json:"position_seconds"`
	PlaybackSpeed   float64    `json:"playback_speed"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Bookmark struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	BookID          uuid.UUID  `json:"book_id"`
	ChapterID       *uuid.UUID `json:"chapter_id,omitempty"`
	PositionSeconds int        `json:"position_seconds"`
	Note            *string    `json:"note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}
GO

# -------------------------------------------------------------
#  internal/api/handler/health.go
# -------------------------------------------------------------

cat > internal/api/handler/health.go << 'GO'
package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

func Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}
GO

# -------------------------------------------------------------
#  internal/api/middleware/logger.go
# -------------------------------------------------------------

cat > internal/api/middleware/logger.go << 'GO'
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: 200}

		next.ServeHTTP(ww, r)

		slog.Info("request",
			"method",   r.Method,
			"path",     r.URL.Path,
			"status",   ww.status,
			"duration", time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
GO

# -------------------------------------------------------------
#  cmd/server/main.go
# -------------------------------------------------------------

cat > cmd/server/main.go << 'GO'
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

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(middleware.Logger)

	r.Get("/health", handler.Health)

	// TODO: v dalším kroku přidáme routy pro books, authors, playback

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // delší kvůli audio streamingu
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
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()

	if err := srv.Shutdown(shutCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
	slog.Info("server zastaven")
}
GO

# -------------------------------------------------------------
#  Migrations - SQLite schéma se vkládá přes go:embed
# -------------------------------------------------------------

mkdir -p internal/db/migrations
echo "→ Vlož SQLite schéma do internal/db/migrations/001_init.sql (viz internal/db/migrate.go v repozitáři)"

# -------------------------------------------------------------
#  go mod tidy (s novými uuid importy)
# -------------------------------------------------------------

go get github.com/google/uuid
go mod tidy

echo ""
echo "✓ Projekt $PROJECT je připraven"
echo ""
echo "  Struktura:"
find . -type f | sort | sed 's/^/    /'
echo ""
echo "  Další kroky:"
echo "    1. Zkontroluj DB_PATH a AUDIO_ROOT v .env"
echo "    2. go run ./cmd/server"
echo "    3. curl http://localhost:8080/health"