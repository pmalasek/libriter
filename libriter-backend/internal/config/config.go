package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server  ServerConfig
	DB      DBConfig
	JWT     JWTConfig
	Storage StorageConfig
}

type ServerConfig struct {
	Port int
	Env  string
}

// DBConfig popisuje SQLite databázi – jediný soubor na disku.
type DBConfig struct {
	Path string // cesta k souboru databáze (vytvoří se automaticky)
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
