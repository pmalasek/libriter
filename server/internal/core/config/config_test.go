package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	absAudio := filepath.Join(tmp, "mnt", "books")
	s := StorageConfig{
		AudioRoot:       absAudio,
		CoverRoot:       "data/covers",
		AuthorImageRoot: "data/author-images",
		ImportRoot:      filepath.Join(tmp, "data", "import"),
	}
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{s.CoverRoot, s.AuthorImageRoot, s.ImportRoot} {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			t.Errorf("%s nebyl vytvořen: %v", dir, err)
		}
	}
	if _, err := os.Stat(absAudio); !os.IsNotExist(err) {
		t.Errorf("absolutní AUDIO_ROOT se nemá vytvářet: %v", err)
	}

	s.AudioRoot = "data/books"
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(s.AudioRoot); err != nil || !fi.IsDir() {
		t.Errorf("relativní AUDIO_ROOT nebyl vytvořen: %v", err)
	}
}

func TestAuthorImageRootDefaultsNextToCovers(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("LIBRITER_ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	t.Setenv("COVER_ROOT", "/opt/libriter-demo/data/covers")
	t.Setenv("AUTHOR_IMAGE_ROOT", "")

	cfg := load()
	if want := "/opt/libriter-demo/data/author-images"; cfg.Storage.AuthorImageRoot != want {
		t.Errorf("AuthorImageRoot = %q, chci %q", cfg.Storage.AuthorImageRoot, want)
	}
}

// unsetEnv odstraní proměnnou jen po dobu testu. t.Setenv zajistí, že se
// původní hodnota po testu vrátí – i když ji mezitím nastaví načtení .env.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

// loadWithEnvFile zavolá Load nad dočasným .env s danými řádky. Proměnné
// z prostředí vývojáře (a jeho server/.env) test neovlivní.
func loadWithEnvFile(t *testing.T, lines ...string) (*Config, error) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, key := range []string{"JWT_SECRET", "SERVER_ENV"} {
		unsetEnv(t, key)
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIBRITER_ENV_FILE", path)
	return Load()
}

// captureWarnings přesměruje výchozí slog do bufferu po dobu testu.
func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLoadJWTSecret(t *testing.T) {
	strong := strings.Repeat("ab", 32) // 64 znaků jako z `openssl rand -hex 32`
	short := "kratky-tajny-klic"

	tests := []struct {
		name    string
		env     string
		secret  string
		wantErr bool
		warn    bool
	}{
		{name: "chybí", env: "development", secret: "", wantErr: true},
		{name: "vzorová hodnota při vývoji", env: "development", secret: PlaceholderJWTSecret, wantErr: true},
		{name: "vzorová hodnota v produkci", env: "production", secret: PlaceholderJWTSecret, wantErr: true},
		{name: "krátký v produkci", env: "production", secret: short, wantErr: true},
		{name: "krátký při vývoji jen varuje", env: "development", secret: short, warn: true},
		{name: "31 bajtů v produkci", env: "production", secret: strings.Repeat("x", MinJWTSecretLen-1), wantErr: true},
		{name: "32 bajtů v produkci", env: "production", secret: strings.Repeat("x", MinJWTSecretLen)},
		{name: "silný v produkci", env: "production", secret: strong},
		{name: "silný při vývoji", env: "development", secret: strong},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureWarnings(t)
			cfg, err := loadWithEnvFile(t, "SERVER_ENV="+tc.env, "JWT_SECRET="+tc.secret)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Load prošel, chtěna chyba")
				}
				if !strings.Contains(err.Error(), "openssl rand -hex 32") {
					t.Errorf("chyba %q neradí, jak klíč vygenerovat", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.JWT.Secret != tc.secret || cfg.Server.Env != tc.env {
				t.Errorf("načteno secret=%q env=%q", cfg.JWT.Secret, cfg.Server.Env)
			}
			if warned := strings.Contains(logs.String(), "JWT_SECRET"); warned != tc.warn {
				t.Errorf("varování = %v, chtěno %v (log: %q)", warned, tc.warn, logs.String())
			}
		})
	}
}

// Skutečná proměnná prostředí přebíjí .env – i pro kontrolu JWT_SECRET.
func TestLoadJWTSecretFromEnvironment(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("LIBRITER_ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	t.Setenv("SERVER_ENV", "production")

	t.Setenv("JWT_SECRET", PlaceholderJWTSecret)
	if _, err := Load(); err == nil {
		t.Error("vzorová hodnota z prostředí prošla")
	}

	t.Setenv("JWT_SECRET", strings.Repeat("f", 64))
	if _, err := Load(); err != nil {
		t.Errorf("silný klíč z prostředí: %v", err)
	}
}

// LoadCLI JWT_SECRET nekontroluje – správa uživatelů tokeny nepodepisuje.
func TestLoadCLIIgnoresJWTSecret(t *testing.T) {
	if _, err := loadWithEnvFile(t, "SERVER_ENV=production", "JWT_SECRET="+PlaceholderJWTSecret); err == nil {
		t.Fatal("Load prošel se vzorovou hodnotou")
	}
	if _, err := LoadCLI(); err != nil {
		t.Errorf("LoadCLI: %v", err)
	}
}
