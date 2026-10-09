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

	return NewSettings(storage.New(conn), cfg, config.LibraryConfig{}, testProviders)
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

// Uložené nastavení má přednost před .env a Profiles z něj dělá
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

	profiles, key, err := svc.Profiles(ctx)
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	if !reflect.DeepEqual(profiles.Default, []string{"openlibrary"}) {
		t.Errorf("zapnuté zdroje = %v, chtěno [openlibrary]", profiles.Default)
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

// Jazykové profily z konfigurace: uvedené zdroje zapnuté v pořadí, zbytek
// vypnutý, neznámé jméno (zdroj z novější verze) se zahodí.
func TestMetadataLanguageDefaultsFromConfig(t *testing.T) {
	svc := newSettingsService(t, config.MetadataConfig{
		Providers: []string{"databazeknih"},
		LanguageProviders: map[string][]string{
			"en": {"googlebooks", "neexistuje", "openlibrary"},
		},
	})

	settings, err := svc.Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}

	want := []LanguageProfile{{
		Language: "en",
		Providers: []ProviderSetting{
			{Name: "googlebooks", Enabled: true},
			{Name: "openlibrary", Enabled: true},
			{Name: "cbdb", Enabled: false},
			{Name: "databazeknih", Enabled: false},
		},
	}}
	if !reflect.DeepEqual(settings.Languages, want) {
		t.Errorf("jazyky = %+v, chtěno %+v", settings.Languages, want)
	}

	profiles := settings.Profiles()
	if !reflect.DeepEqual(profiles.ByLanguage["en"], []string{"googlebooks", "openlibrary"}) {
		t.Errorf("profil en = %v", profiles.ByLanguage["en"])
	}
	if !reflect.DeepEqual(profiles.Default, []string{"databazeknih"}) {
		t.Errorf("výchozí profil = %v", profiles.Default)
	}
}

func TestSetMetadataLanguages(t *testing.T) {
	ctx := context.Background()
	svc := newSettingsService(t, config.MetadataConfig{
		Providers:         testProviders,
		LanguageProviders: map[string][]string{"en": {"googlebooks"}},
	})

	saved, err := svc.SetMetadata(ctx, MetadataSettings{
		Providers: []ProviderSetting{{Name: "databazeknih", Enabled: true}},
		Languages: []LanguageProfile{
			{Language: " DE ", Providers: []ProviderSetting{{Name: "googlebooks", Enabled: true}}},
			{Language: "en", Providers: []ProviderSetting{{Name: "openlibrary", Enabled: true}}},
		},
	})
	if err != nil {
		t.Fatalf("SetMetadata: %v", err)
	}
	if len(saved.Languages) != 2 || saved.Languages[0].Language != "de" || saved.Languages[1].Language != "en" {
		t.Fatalf("uložené jazyky = %+v, chtěno de a en", saved.Languages)
	}

	// Uložené profily nahrazují ty z konfigurace.
	profiles, _, err := svc.Profiles(ctx)
	if err != nil {
		t.Fatalf("Profiles: %v", err)
	}
	want := map[string][]string{"de": {"googlebooks"}, "en": {"openlibrary"}}
	if !reflect.DeepEqual(profiles.ByLanguage, want) {
		t.Errorf("profily = %v, chtěno %v", profiles.ByLanguage, want)
	}

	// Odebrání všech profilů se uloží jako prázdný seznam, ne návrat k .env.
	if _, err := svc.SetMetadata(ctx, MetadataSettings{Providers: []ProviderSetting{}}); err != nil {
		t.Fatalf("SetMetadata bez jazyků: %v", err)
	}
	profiles, _, _ = svc.Profiles(ctx)
	if len(profiles.ByLanguage) != 0 {
		t.Errorf("profily po odebrání = %v, chtěno žádné", profiles.ByLanguage)
	}
}

func TestSetMetadataRejectsBadLanguages(t *testing.T) {
	ctx := context.Background()
	svc := newSettingsService(t, config.MetadataConfig{Providers: testProviders})

	cases := map[string][]LanguageProfile{
		"neznámý jazyk": {{Language: "xx"}},
		"prázdný jazyk": {{Language: " "}},
		"dvakrát jazyk": {{Language: "en"}, {Language: "EN"}},
		"neznámý zdroj": {{Language: "en", Providers: []ProviderSetting{{Name: "neexistuje"}}}},
		"dvakrát zdroj": {{Language: "en", Providers: []ProviderSetting{{Name: "cbdb"}, {Name: "cbdb"}}}},
	}
	for name, languages := range cases {
		_, err := svc.SetMetadata(ctx, MetadataSettings{Languages: languages})
		if !errors.Is(err, ErrInvalidSetting) {
			t.Errorf("%s: %v, chtěno ErrInvalidSetting", name, err)
		}
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

func TestLibrarySettings(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, config.DBConfig{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatalf("otevření databáze: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	svc := NewSettings(storage.New(conn), config.MetadataConfig{}, config.LibraryConfig{DefaultLanguage: "sk"}, testProviders)

	// Dokud admin nic neuloží, platí hodnota z konfigurace.
	if got := svc.DefaultLanguage(ctx); got != "sk" {
		t.Errorf("výchozí jazyk = %q, chtěno sk (z konfigurace)", got)
	}

	saved, err := svc.SetLibrary(ctx, LibrarySettings{DefaultLanguage: " EN "})
	if err != nil {
		t.Fatalf("SetLibrary: %v", err)
	}
	if saved.DefaultLanguage != "en" || svc.DefaultLanguage(ctx) != "en" {
		t.Errorf("uloženo %q, čteno %q, chtěno en", saved.DefaultLanguage, svc.DefaultLanguage(ctx))
	}

	for _, bad := range []string{"", "xx"} {
		if _, err := svc.SetLibrary(ctx, LibrarySettings{DefaultLanguage: bad}); !errors.Is(err, ErrInvalidSetting) {
			t.Errorf("jazyk %q: %v, chtěno ErrInvalidSetting", bad, err)
		}
	}
}
