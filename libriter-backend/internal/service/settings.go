package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"libriter/internal/config"
	"libriter/internal/model"
	"libriter/internal/storage"
)

// Klíče v tabulce settings.
const (
	SettingMetadataProviders = "metadata.providers"
	SettingGoogleBooksAPIKey = "metadata.googlebooks_api_key"
	SettingRegistration      = "registration"
)

// ErrInvalidSetting znamená, že poslané nastavení neprošlo kontrolou.
var ErrInvalidSetting = errors.New("neplatné nastavení")

// ProviderSetting je jeden zdroj metadat v nastavení – pořadí v seznamu je
// pořadí, ve kterém se zdroje zkoušejí.
type ProviderSetting struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// MetadataSettings je celé nastavení zdrojů metadat.
type MetadataSettings struct {
	Providers         []ProviderSetting `json:"providers"`
	GoogleBooksAPIKey string            `json:"google_books_api_key"`
}

// RegistrationSettings řídí veřejnou registraci.
type RegistrationSettings struct {
	Enabled     bool   `json:"enabled"`
	DefaultRole string `json:"default_role"`
}

// SettingsService čte a zapisuje nastavení, které lze měnit za běhu.
//
// Hodnoty z .env jsou jen výchozí: dokud v databázi nic není, vrací se
// konfigurace ze startu. Jakmile admin nastavení uloží, vyhrává databáze.
// Čtení nikdy do databáze nezapisuje, takže čerstvá instalace se chová
// stejně jako dřív.
type SettingsService struct {
	store *storage.Store
	// known jsou jména všech zdrojů, které binárka umí (i vypnutých).
	known []string
	// envDefaults je nastavení odvozené z konfigurace při startu.
	envDefaults MetadataSettings
}

func NewSettings(store *storage.Store, cfg config.MetadataConfig, known []string) *SettingsService {
	return &SettingsService{
		store:       store,
		known:       known,
		envDefaults: defaultMetadata(cfg, known),
	}
}

// defaultMetadata sestaví výchozí nastavení: zdroje z konfigurace v jejím
// pořadí jako zapnuté, zbytek známých zdrojů vypnutý.
func defaultMetadata(cfg config.MetadataConfig, known []string) MetadataSettings {
	out := MetadataSettings{
		Providers:         make([]ProviderSetting, 0, len(known)),
		GoogleBooksAPIKey: cfg.GoogleBooksAPIKey,
	}

	seen := make(map[string]bool, len(known))
	knownSet := make(map[string]bool, len(known))
	for _, name := range known {
		knownSet[name] = true
	}

	for _, name := range cfg.Providers {
		name = strings.ToLower(strings.TrimSpace(name))
		if !knownSet[name] || seen[name] {
			continue
		}
		seen[name] = true
		out.Providers = append(out.Providers, ProviderSetting{Name: name, Enabled: true})
	}
	for _, name := range known {
		if !seen[name] {
			out.Providers = append(out.Providers, ProviderSetting{Name: name, Enabled: false})
		}
	}
	return out
}

// Metadata vrátí nastavení zdrojů – z databáze, jinak výchozí z konfigurace.
func (s *SettingsService) Metadata(ctx context.Context) (MetadataSettings, error) {
	out := MetadataSettings{
		Providers:         append([]ProviderSetting(nil), s.envDefaults.Providers...),
		GoogleBooksAPIKey: s.envDefaults.GoogleBooksAPIKey,
	}

	raw, err := s.store.GetSetting(ctx, SettingMetadataProviders)
	switch {
	case err == nil:
		var stored []ProviderSetting
		if err := json.Unmarshal(raw, &stored); err != nil {
			return MetadataSettings{}, fmt.Errorf("nastavení zdrojů metadat: %w", err)
		}
		out.Providers = s.normalize(stored)
	case !errors.Is(err, storage.ErrNotFound):
		return MetadataSettings{}, err
	}

	raw, err = s.store.GetSetting(ctx, SettingGoogleBooksAPIKey)
	switch {
	case err == nil:
		var key string
		if err := json.Unmarshal(raw, &key); err != nil {
			return MetadataSettings{}, fmt.Errorf("klíč Google Books: %w", err)
		}
		out.GoogleBooksAPIKey = key
	case !errors.Is(err, storage.ErrNotFound):
		return MetadataSettings{}, err
	}

	return out, nil
}

// SetMetadata ověří a uloží nastavení zdrojů. Vrací uloženou podobu.
func (s *SettingsService) SetMetadata(ctx context.Context, in MetadataSettings) (MetadataSettings, error) {
	seen := make(map[string]bool, len(in.Providers))
	for i := range in.Providers {
		name := strings.ToLower(strings.TrimSpace(in.Providers[i].Name))
		if !s.isKnown(name) {
			return MetadataSettings{}, fmt.Errorf("%w: neznámý zdroj metadat %q", ErrInvalidSetting, in.Providers[i].Name)
		}
		if seen[name] {
			return MetadataSettings{}, fmt.Errorf("%w: zdroj %q je v seznamu dvakrát", ErrInvalidSetting, name)
		}
		seen[name] = true
		in.Providers[i].Name = name
	}

	providers := s.normalize(in.Providers)
	value, err := json.Marshal(providers)
	if err != nil {
		return MetadataSettings{}, err
	}
	if err := s.store.SetSetting(ctx, SettingMetadataProviders, value); err != nil {
		return MetadataSettings{}, err
	}

	key := strings.TrimSpace(in.GoogleBooksAPIKey)
	keyValue, err := json.Marshal(key)
	if err != nil {
		return MetadataSettings{}, err
	}
	if err := s.store.SetSetting(ctx, SettingGoogleBooksAPIKey, keyValue); err != nil {
		return MetadataSettings{}, err
	}

	return MetadataSettings{Providers: providers, GoogleBooksAPIKey: key}, nil
}

// EnabledProviders vrátí jména zapnutých zdrojů v pořadí, ve kterém se mají
// zkoušet – přesně to, co potřebuje metadata.Registry.Rebuild.
func (s *SettingsService) EnabledProviders(ctx context.Context) ([]string, string, error) {
	settings, err := s.Metadata(ctx)
	if err != nil {
		return nil, "", err
	}

	names := make([]string, 0, len(settings.Providers))
	for _, p := range settings.Providers {
		if p.Enabled {
			names = append(names, p.Name)
		}
	}
	return names, settings.GoogleBooksAPIKey, nil
}

// Registration vrátí nastavení registrace (výchozí: zapnutá, role reader).
func (s *SettingsService) Registration(ctx context.Context) (RegistrationSettings, error) {
	out := RegistrationSettings{Enabled: true, DefaultRole: model.RoleReader}

	raw, err := s.store.GetSetting(ctx, SettingRegistration)
	if errors.Is(err, storage.ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return RegistrationSettings{}, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return RegistrationSettings{}, fmt.Errorf("nastavení registrace: %w", err)
	}
	if _, ok := model.RoleLevel[out.DefaultRole]; !ok {
		out.DefaultRole = model.RoleReader
	}
	return out, nil
}

// SetRegistration ověří a uloží nastavení registrace. Roli admin přes
// registraci rozdávat nelze – ta se přiděluje výhradně ručně.
func (s *SettingsService) SetRegistration(ctx context.Context, in RegistrationSettings) (RegistrationSettings, error) {
	in.DefaultRole = strings.ToLower(strings.TrimSpace(in.DefaultRole))
	if in.DefaultRole != model.RoleReader && in.DefaultRole != model.RoleEditor {
		return RegistrationSettings{}, fmt.Errorf("%w: výchozí role musí být reader nebo editor", ErrInvalidSetting)
	}

	value, err := json.Marshal(in)
	if err != nil {
		return RegistrationSettings{}, err
	}
	if err := s.store.SetSetting(ctx, SettingRegistration, value); err != nil {
		return RegistrationSettings{}, err
	}
	return in, nil
}

// normalize zahodí zdroje, které binárka nezná (zbyly po downgradu), a doplní
// chybějící známé jako vypnuté. Pořadí uložených zdrojů zůstává.
func (s *SettingsService) normalize(in []ProviderSetting) []ProviderSetting {
	out := make([]ProviderSetting, 0, len(s.known))
	seen := make(map[string]bool, len(s.known))

	for _, p := range in {
		name := strings.ToLower(strings.TrimSpace(p.Name))
		if !s.isKnown(name) || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, ProviderSetting{Name: name, Enabled: p.Enabled})
	}
	for _, name := range s.known {
		if !seen[name] {
			out = append(out, ProviderSetting{Name: name, Enabled: false})
		}
	}
	return out
}

func (s *SettingsService) isKnown(name string) bool {
	for _, known := range s.known {
		if known == name {
			return true
		}
	}
	return false
}
