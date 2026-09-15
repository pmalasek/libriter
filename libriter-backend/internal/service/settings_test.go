package service

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/storage"
)

var testProviders = []string{"cbdb", "databazeknih", "googlebooks", "openlibrary"}

func newSettingsService(t *testing.T, cfg config.MetadataConfig) *SettingsService {
	t.Helper()

	conn, err := db.Open(context.Background(), config.DBConfig{
		Path: filepath.Join(t.TempDir(), "test.db"),
	})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return NewSettings(storage.New(conn), cfg, testProviders)
}

// Dokud admin nic neuloží, platí hodnoty z konfigurace (.env).
func TestMetadataDefaultsFromConfig(t *testing.T) {
	svc := newSettingsService(t, config.MetadataConfig{
		Providers:         []string{"databazeknih", "openlibrary"},
		GoogleBooksAPIKey: "klic-z-env",
	})

	settings, err := svc.Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}

	want := []ProviderSetting{
		{Name: "databazeknih", Enabled: true},
		{Name: "openlibrary", Enabled: true},
		{Name: "cbdb", Enabled: false},
		{Name: "googlebooks", Enabled: false},
	}
	if !reflect.DeepEqual(settings.Providers, want) {
		t.Errorf("zdroje = %+v, chtěno %+v", settings.Providers, want)
	}
	if settings.GoogleBooksAPIKey != "klic-z-env" {
		t.Errorf("klíč = %q, chtěno klic-z-env", settings.GoogleBooksAPIKey)
	}
}

// Uložené nastavení má přednost před .env a EnabledProviders z něj dělá
// pořadí pro řetězec zdrojů.
func TestSetMetadataOverridesConfig(t *testing.T) {
	ctx := context.Background()
	svc := newSettingsService(t, config.MetadataConfig{
		Providers:         []string{"databazeknih", "cbdb", "openlibrary", "googlebooks"},
		GoogleBooksAPIKey: "klic-z-env",
	})

	saved, err := svc.SetMetadata(ctx, MetadataSettings{
		Providers: []ProviderSetting{
			{Name: "openlibrary", Enabled: true},
			{Name: "databazeknih", Enabled: false},
		},
		GoogleBooksAPIKey: "  novy-klic  ",
	})
	if err != nil {
		t.Fatalf("SetMetadata: %v", err)
	}

	// Neuvedené známé zdroje se doplní jako vypnuté, pořadí uvedených zůstává.
	want := []ProviderSetting{
		{Name: "openlibrary", Enabled: true},
		{Name: "databazeknih", Enabled: false},
		{Name: "cbdb", Enabled: false},
		{Name: "googlebooks", Enabled: false},
	}
	if !reflect.DeepEqual(saved.Providers, want) {
		t.Errorf("uložené zdroje = %+v, chtěno %+v", saved.Providers, want)
	}
	if saved.GoogleBooksAPIKey != "novy-klic" {
		t.Errorf("klíč = %q, chtěno novy-klic (ořezaný)", saved.GoogleBooksAPIKey)
	}

	names, key, err := svc.EnabledProviders(ctx)
	if err != nil {
		t.Fatalf("EnabledProviders: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"openlibrary"}) {
		t.Errorf("zapnuté zdroje = %v, chtěno [openlibrary]", names)
	}
	if key != "novy-klic" {
		t.Errorf("klíč = %q, chtěno novy-klic", key)
	}
}

func TestSetMetadataRejectsUnknownAndDuplicate(t *testing.T) {
	ctx := context.Background()
	svc := newSettingsService(t, config.MetadataConfig{Providers: testProviders})

	_, err := svc.SetMetadata(ctx, MetadataSettings{
		Providers: []ProviderSetting{{Name: "neexistuje", Enabled: true}},
	})
	if !errors.Is(err, ErrInvalidSetting) {
		t.Errorf("neznámý zdroj = %v, chtěno ErrInvalidSetting", err)
	}

	_, err = svc.SetMetadata(ctx, MetadataSettings{
		Providers: []ProviderSetting{
			{Name: "cbdb", Enabled: true},
			{Name: "cbdb", Enabled: false},
		},
	})
	if !errors.Is(err, ErrInvalidSetting) {
		t.Errorf("duplicitní zdroj = %v, chtěno ErrInvalidSetting", err)
	}
}

func TestRegistrationSettings(t *testing.T) {
	ctx := context.Background()
	svc := newSettingsService(t, config.MetadataConfig{Providers: testProviders})

	settings, err := svc.Registration(ctx)
	if err != nil {
		t.Fatalf("Registration: %v", err)
	}
	if !settings.Enabled || settings.DefaultRole != model.RoleReader {
		t.Errorf("výchozí nastavení = %+v, chtěno zapnutá registrace s rolí reader", settings)
	}

	if _, err := svc.SetRegistration(ctx, RegistrationSettings{Enabled: false, DefaultRole: model.RoleEditor}); err != nil {
		t.Fatalf("SetRegistration: %v", err)
	}
	settings, err = svc.Registration(ctx)
	if err != nil {
		t.Fatalf("Registration po uložení: %v", err)
	}
	if settings.Enabled || settings.DefaultRole != model.RoleEditor {
		t.Errorf("nastavení = %+v, chtěno vypnutá registrace s rolí editor", settings)
	}

	// Admina přes registraci rozdávat nelze.
	if _, err := svc.SetRegistration(ctx, RegistrationSettings{Enabled: true, DefaultRole: model.RoleAdmin}); !errors.Is(err, ErrInvalidSetting) {
		t.Errorf("role admin = %v, chtěno ErrInvalidSetting", err)
	}
}
