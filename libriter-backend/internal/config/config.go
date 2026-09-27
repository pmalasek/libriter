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
	Library  LibraryConfig

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
	// MobileExpiry je platnost tokenu mobilní aplikace. Ta se nepřihlašuje
	// znovu každých pár dní – offline telefon by se k serveru nedostal a
	// přehrávání stažené knihy by uvázlo na přihlašovací obrazovce.
	MobileExpiry time.Duration
}

type StorageConfig struct {
	AudioRoot string
	CoverRoot string
	// AuthorImageRoot je adresář s fotkami autorů. Drží se odděleně od obálek,
	// aby se obsah obou adresářů nemíchal.
	AuthorImageRoot string
	// ImportRoot je staging pro nahrávané knihy. Musí ležet mimo AudioRoot
	// (jinak by rozpracované soubory ingestoval scanner) a ideálně na stejném
	// disku, aby se knihy do knihovny jen přejmenovaly, ne kopírovaly.
	ImportRoot  string
	MaxUploadMB int64 // limit jednoho importu (součet nahraných souborů)
}

// EnsureDirs vytvoří datové adresáře, které ještě neexistují. AUDIO_ROOT se
// vytváří jen jako relativní cesta (výchozí ./data/books) – absolutní cesta
// obvykle vede na externí disk a její chybění znamená nepřipojený disk.
// Prázdný adresář by se pak tvářil jako prázdná knihovna na systémovém disku.
func (s StorageConfig) EnsureDirs() error {
	dirs := []struct{ name, path string }{
		{"COVER_ROOT", s.CoverRoot},
		{"AUTHOR_IMAGE_ROOT", s.AuthorImageRoot},
		{"IMPORT_ROOT", s.ImportRoot},
	}
	if !filepath.IsAbs(s.AudioRoot) {
		dirs = append(dirs, struct{ name, path string }{"AUDIO_ROOT", s.AudioRoot})
	}
	for _, d := range dirs {
		if d.path == "" {
			continue
		}
		if err := os.MkdirAll(d.path, 0o755); err != nil {
			return fmt.Errorf("vytvoření %s %s: %w", d.name, d.path, err)
		}
	}
	return nil
}

// LibraryConfig jsou výchozí hodnoty nastavení knihovny. Stejně jako u zdrojů
// metadat platí jen do chvíle, kdy admin nastavení uloží v administraci.
type LibraryConfig struct {
	// DefaultLanguage je jazyk nové knihy (ISO 639-1), když ho neuvádějí
	// tagy ani ten, kdo knihu zakládá.
	DefaultLanguage string
}

// MetadataConfig řídí zdroje knižních metadat.
type MetadataConfig struct {
	// Providers je pořadí, ve kterém se zdroje zkoušejí. Zdroj, který v
	// seznamu není, je vypnutý; prázdný seznam vypne metadata úplně.
	Providers []string
	// LanguageProviders jsou vlastní pořadí zdrojů pro jazyky knih (klíč je
	// kód ISO 639-1). Jazyk, který tu není, používá Providers.
	LanguageProviders map[string][]string
	// GoogleBooksAPIKey je nepovinný – bez něj se jede na sdílenou anonymní
	// kvótu Google Books, která se u sdílené IP snadno vyčerpá.
	GoogleBooksAPIKey string
}

// DefaultMetadataProviders je výchozí pořadí: nejdřív české zdroje, pak
// zahraniční API jako záloha.
var DefaultMetadataProviders = []string{"databazeknih", "cbdb", "openlibrary", "googlebooks"}

// DefaultLanguageProviders jsou výchozí pořadí pro jazyky knih. Čeština má
// vlastní profil jako ostatní jazyky – výchozí pořadí (DefaultMetadataProviders)
// je jen záloha pro jazyky, které tu nejsou.
var DefaultLanguageProviders = map[string][]string{
	"cs": {"databazeknih", "cbdb", "openlibrary", "googlebooks"},
	"en": {"audible_com", "goodreads", "openlibrary", "googlebooks"},
	"de": {"audible_de", "googlebooks", "openlibrary"},
}

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

	cfg := &Config{
		Server: ServerConfig{
			Port: envInt("SERVER_PORT", 8080),
			Env:  envStr("SERVER_ENV", "development"),
		},
		DB: DBConfig{
			Path: env.path("DB_PATH", "./data/libriter.db"),
		},
		JWT: JWTConfig{
			Secret:       envStr("JWT_SECRET", ""),
			ExpiryHours:  time.Duration(envInt("JWT_EXPIRY_HOURS", 72)) * time.Hour,
			MobileExpiry: time.Duration(envInt("JWT_MOBILE_EXPIRY_DAYS", 365)) * 24 * time.Hour,
		},
		Storage: StorageConfig{
			AudioRoot:       env.path("AUDIO_ROOT", "/var/lib/libriter/audio"),
			CoverRoot:       env.path("COVER_ROOT", "/var/lib/libriter/covers"),
			AuthorImageRoot: env.path("AUTHOR_IMAGE_ROOT", "/var/lib/libriter/author-images"),
			ImportRoot:      env.path("IMPORT_ROOT", ""),
			MaxUploadMB:     int64(envInt("MAX_UPLOAD_MB", 3072)),
		},
		Metadata: MetadataConfig{
			Providers:         envList("METADATA_PROVIDERS", DefaultMetadataProviders),
			LanguageProviders: envLanguageLists("METADATA_LANGUAGE_PROVIDERS", DefaultLanguageProviders),
			GoogleBooksAPIKey: envStr("GOOGLE_BOOKS_API_KEY", ""),
		},
		Library: LibraryConfig{
			DefaultLanguage: strings.ToLower(strings.TrimSpace(envStr("DEFAULT_BOOK_LANGUAGE", "cs"))),
		},
		EnvFile: env.file,
	}
	// Staging importu vedle knihovny: mimo ni, ale nejspíš na stejném disku.
	if cfg.Storage.ImportRoot == "" {
		cfg.Storage.ImportRoot = filepath.Join(filepath.Dir(cfg.Storage.AudioRoot), "import")
	}
	return cfg
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

// envLanguageLists načte seznamy zdrojů podle jazyka ve tvaru
// "en=audible_com,goodreads;de=audible_de". Položka bez "=" nebo bez
// jazyka se přeskočí s varováním. Stejně jako envList rozlišuje
// "nenastaveno" (fallback) od "nastaveno na prázdno" (žádné jazykové profily).
func envLanguageLists(key string, fallback map[string][]string) map[string][]string {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	out := make(map[string][]string)
	for _, entry := range strings.Split(raw, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		lang, list, found := strings.Cut(entry, "=")
		lang = strings.ToLower(strings.TrimSpace(lang))
		if !found || lang == "" {
			slog.Warn("neplatná položka v "+key, "položka", entry)
			continue
		}
		var names []string
		for _, part := range strings.Split(list, ",") {
			if trimmed := strings.ToLower(strings.TrimSpace(part)); trimmed != "" {
				names = append(names, trimmed)
			}
		}
		out[lang] = names
	}
	return out
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
