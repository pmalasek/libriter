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
  migrations \
  scripts

echo "→ Adresáře vytvořeny"

# -------------------------------------------------------------
#  Go modul + závislosti
# -------------------------------------------------------------

go mod init "$MODULE"

go get \
  github.com/go-chi/chi/v5 \
  github.com/go-chi/chi/v5/middleware \
  github.com/jackc/pgx/v5 \
  github.com/jackc/pgx/v5/pgxpool \
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

# Databáze
DB_HOST=localhost
DB_PORT=5432
DB_NAME=libriter
DB_USER=postgres
DB_PASSWORD=
DB_POOL_MAX=10

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
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	PoolMax  int
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
			Host:     envStr("DB_HOST", "localhost"),
			Port:     envInt("DB_PORT", 5432),
			Name:     envStr("DB_NAME", "libriter"),
			User:     envStr("DB_USER", "postgres"),
			Password: envStr("DB_PASSWORD", ""),
			PoolMax:  envInt("DB_POOL_MAX", 10),
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

func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable pool_max_conns=%d",
		d.Host, d.Port, d.Name, d.User, d.Password, d.PoolMax,
	)
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
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"libriter/internal/config"
)

func Connect(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return pool, nil
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

	pool, err := db.Connect(ctx, cfg.DB)
	if err != nil {
		slog.Error("databáze", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	slog.Info("databáze připojena", "db", cfg.DB.Name)

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
#  Migrations - zkopírujeme init skript pokud existuje
# -------------------------------------------------------------

if [ -f "../libriter_init.sql" ]; then
  cp ../libriter_init.sql migrations/001_init.sql
  echo "→ Migration 001_init.sql zkopírována"
else
  touch migrations/001_init.sql
  echo "→ Vlož obsah libriter_init.sql do migrations/001_init.sql"
fi

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
echo "    1. Nastav DB_PASSWORD v .env"
echo "    2. go run ./cmd/server"
echo "    3. curl http://localhost:8080/health"