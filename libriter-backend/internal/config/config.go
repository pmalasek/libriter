package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	DB       DBConfig
	JWT      JWTConfig
	Storage  StorageConfig
	Metadata MetadataConfig

	// EnvFile je cesta k načtenému .env (prázdná, pokud se žádný nenašel).
	EnvFile string
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
	AudioRoot string
	CoverRoot string
	// AuthorImageRoot je adresář s fotkami autorů. Drží se odděleně od obálek,
	// aby se obsah obou adresářů nemíchal.
	AuthorImageRoot string
	MaxUploadMB     int64
}

// MetadataConfig řídí zdroje knižních metadat.
type MetadataConfig struct {
	// Providers je pořadí, ve kterém se zdroje zkoušejí. Zdroj, který v
	// seznamu není, je vypnutý; prázdný seznam vypne metadata úplně.
	Providers []string
	// GoogleBooksAPIKey je nepovinný – bez něj se jede na sdílenou anonymní
	// kvótu Google Books, která se u sdílené IP snadno vyčerpá.
	GoogleBooksAPIKey string
}

// DefaultMetadataProviders je výchozí pořadí: nejdřív české zdroje, pak
// zahraniční API jako záloha.
var DefaultMetadataProviders = []string{"databazeknih", "cbdb", "openlibrary", "googlebooks"}

// Load načte konfiguraci pro server. Vyžaduje JWT_SECRET, protože server
// podepisuje a ověřuje tokeny.
func Load() (*Config, error) {
	cfg := load()
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET není nastaven (hledal jsem .env: %s)", envFileOrNone(cfg.EnvFile))
	}
	return cfg, nil
}

// LoadCLI načte konfiguraci pro příkazy, které nepracují s tokeny
// (například správa uživatelů). JWT_SECRET proto nevyžaduje.
func LoadCLI() (*Config, error) {
	return load(), nil
}

func load() *Config {
	env := loadDotEnv()

	return &Config{
		Server: ServerConfig{
			Port: envInt("SERVER_PORT", 8080),
			Env:  envStr("SERVER_ENV", "development"),
		},
		DB: DBConfig{
			Path: env.path("DB_PATH", "./data/libriter.db"),
		},
		JWT: JWTConfig{
			Secret:      envStr("JWT_SECRET", ""),
			ExpiryHours: time.Duration(envInt("JWT_EXPIRY_HOURS", 72)) * time.Hour,
		},
		Storage: StorageConfig{
			AudioRoot:       env.path("AUDIO_ROOT", "/var/lib/libriter/audio"),
			CoverRoot:       env.path("COVER_ROOT", "/var/lib/libriter/covers"),
			AuthorImageRoot: env.path("AUTHOR_IMAGE_ROOT", "/var/lib/libriter/author-images"),
			MaxUploadMB:     int64(envInt("MAX_UPLOAD_MB", 500)),
		},
		Metadata: MetadataConfig{
			Providers:         envList("METADATA_PROVIDERS", DefaultMetadataProviders),
			GoogleBooksAPIKey: envStr("GOOGLE_BOOKS_API_KEY", ""),
		},
		EnvFile: env.file,
	}
}

// dotEnv drží výsledek načtení .env: odkud se čerpalo a které klíče z něj přišly.
type dotEnv struct {
	file     string
	baseDir  string
	fromFile map[string]bool
}

// path vrátí cestu z konfigurace. Relativní cesty zapsané v .env se vztahují
// k adresáři toho .env, ne k aktuálnímu adresáři – binárku tak lze spustit
// odkudkoli (např. bin/libriter z korene repozitáře) a pořád míří na stejná data.
// Cesty předané skutečnou proměnnou prostředí se nechávají být.
func (d dotEnv) path(key, fallback string) string {
	value := envStr(key, fallback)
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	if d.baseDir == "" || !d.fromFile[key] {
		return value
	}
	return filepath.Join(d.baseDir, value)
}

// loadDotEnv najde a načte .env. Skutečné proměnné prostředí mají přednost.
func loadDotEnv() dotEnv {
	empty := dotEnv{fromFile: map[string]bool{}}

	path := findEnvFile()
	if path == "" {
		return empty
	}

	values, err := godotenv.Read(path)
	if err != nil {
		slog.Warn("nepodařilo se načíst .env", "soubor", path, "err", err)
		return empty
	}

	fromFile := make(map[string]bool, len(values))
	for key, value := range values {
		if _, exists := os.LookupEnv(key); exists {
			continue // proměnná prostředí přebíjí .env
		}
		if err := os.Setenv(key, value); err != nil {
			continue
		}
		fromFile[key] = true
	}

	return dotEnv{file: path, baseDir: filepath.Dir(path), fromFile: fromFile}
}

// findEnvFile hledá .env v aktuálním adresáři i v nadřazených, a v každém z nich
// také v podadresáři libriter-backend/. Explicitní volbu lze vynutit přes
// LIBRITER_ENV_FILE.
func findEnvFile() string {
	if explicit := os.Getenv("LIBRITER_ENV_FILE"); explicit != "" {
		return explicit
	}

	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		for _, candidate := range []string{
			filepath.Join(dir, ".env"),
			filepath.Join(dir, "libriter-backend", ".env"),
		} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func envFileOrNone(path string) string {
	if path == "" {
		return "žádný nenalezen"
	}
	return path
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envList načte seznam oddělený čárkami. Prázdné položky se zahazují,
// hodnoty se normalizují na malá písmena bez okolních mezer.
//
// Rozlišuje "nenastaveno" (použije se fallback) od "nastaveno na prázdno"
// (prázdný seznam) – jen tak jde METADATA_PROVIDERS= vypnout úplně.
func envList(key string, fallback []string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	var values []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.ToLower(strings.TrimSpace(part)); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
