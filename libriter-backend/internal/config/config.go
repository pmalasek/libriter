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
