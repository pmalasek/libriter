package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"libriter/internal/config"
	"libriter/internal/metadata"
	"libriter/internal/model"
	"libriter/internal/storage"
)

// Klíče v tabulce settings.
const (
	SettingMetadataProviders = "metadata.providers"
	SettingLanguageProviders = "metadata.language_providers"
	SettingGoogleBooksAPIKey = "metadata.googlebooks_api_key"
	SettingRegistration      = "registration"
	SettingLibrary           = "library"
)

// ErrInvalidSetting znamená, že poslané nastavení neprošlo kontrolou.
var ErrInvalidSetting = errors.New("neplatné nastavení")

// ProviderSetting je jeden zdroj metadat v nastavení – pořadí v seznamu je
// pořadí, ve kterém se zdroje zkoušejí.
type ProviderSetting struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// LanguageProfile je vlastní pořadí zdrojů pro knihy v jednom jazyce.
type LanguageProfile struct {
	// Language je kód ISO 639-1 z číselníku jazyků.
	Language  string            `json:"language"`
	Providers []ProviderSetting `json:"providers"`
}

// MetadataSettings je celé nastavení zdrojů metadat. Providers je výchozí
// pořadí, které platí pro každý jazyk bez vlastního profilu v Languages.
type MetadataSettings struct {
	Providers         []ProviderSetting `json:"providers"`
	Languages         []LanguageProfile `json:"languages"`
	GoogleBooksAPIKey string            `json:"google_books_api_key"`
}

// Profiles převede nastavení na jména zapnutých zdrojů – přesně to, co
// potřebuje metadata.Registry.Rebuild.
func (m MetadataSettings) Profiles() metadata.Profiles {
	out := metadata.Profiles{
		Default:    enabledNames(m.Providers),
		ByLanguage: make(map[string][]string, len(m.Languages)),
	}
	for _, l := range m.Languages {
		out.ByLanguage[l.Language] = enabledNames(l.Providers)
	}
	return out
}

func enabledNames(providers []ProviderSetting) []string {
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		if p.Enabled {
			names = append(names, p.Name)
		}
	}
	return names
}

// LibrarySettings jsou nastavení knihovny jako celku.
type LibrarySettings struct {
	// DefaultLanguage je jazyk nové knihy (ISO 639-1), když ho neuvádějí
	// tagy ani ten, kdo knihu zakládá.
	DefaultLanguage string `json:"default_language"`
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
	// libraryDefaults je nastavení knihovny z konfigurace.
	libraryDefaults LibrarySettings
}

func NewSettings(store *storage.Store, cfg config.MetadataConfig, library config.LibraryConfig, known []string) *SettingsService {
	lang := library.DefaultLanguage
	if lang == "" {
		lang = "cs"
	}
	return &SettingsService{
		store:           store,
		known:           known,
		envDefaults:     defaultMetadata(cfg, known),
		libraryDefaults: LibrarySettings{DefaultLanguage: lang},
	}
}

// defaultMetadata sestaví výchozí nastavení: zdroje z konfigurace v jejím
// pořadí jako zapnuté, zbytek známých zdrojů vypnutý. Stejně vzniknou
// jazykové profily, seřazené podle kódu jazyka.
func defaultMetadata(cfg config.MetadataConfig, known []string) MetadataSettings {
	out := MetadataSettings{
		Providers:         defaultProviders(cfg.Providers, known),
		Languages:         make([]LanguageProfile, 0, len(cfg.LanguageProviders)),
		GoogleBooksAPIKey: cfg.GoogleBooksAPIKey,
	}

	languages := make([]string, 0, len(cfg.LanguageProviders))
	for lang := range cfg.LanguageProviders {
		languages = append(languages, lang)
	}
	sort.Strings(languages)
	for _, lang := range languages {
		out.Languages = append(out.Languages, LanguageProfile{
			Language:  lang,
			Providers: defaultProviders(cfg.LanguageProviders[lang], known),
		})
	}
	return out
}

func defaultProviders(enabled, known []string) []ProviderSetting {
	out := make([]ProviderSetting, 0, len(known))
	seen := make(map[string]bool, len(known))
	knownSet := make(map[string]bool, len(known))
	for _, name := range known {
		knownSet[name] = true
	}

	for _, name := range enabled {
		name = strings.ToLower(strings.TrimSpace(name))
		if !knownSet[name] || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, ProviderSetting{Name: name, Enabled: true})
	}
	for _, name := range known {
		if !seen[name] {
			out = append(out, ProviderSetting{Name: name, Enabled: false})
		}
	}
	return out
}

// Metadata vrátí nastavení zdrojů – z databáze, jinak výchozí z konfigurace.
func (s *SettingsService) Metadata(ctx context.Context) (MetadataSettings, error) {
	out := MetadataSettings{
		Providers:         append([]ProviderSetting(nil), s.envDefaults.Providers...),
		Languages:         append([]LanguageProfile(nil), s.envDefaults.Languages...),
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

	raw, err = s.store.GetSetting(ctx, SettingLanguageProviders)
	switch {
	case err == nil:
		var stored []LanguageProfile
		if err := json.Unmarshal(raw, &stored); err != nil {
			return MetadataSettings{}, fmt.Errorf("nastavení zdrojů podle jazyka: %w", err)
		}
		out.Languages = s.normalizeLanguages(stored)
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
	if err := s.checkProviders(in.Providers); err != nil {
		return MetadataSettings{}, err
	}

	seenLang := make(map[string]bool, len(in.Languages))
	for i := range in.Languages {
		lang := strings.ToLower(strings.TrimSpace(in.Languages[i].Language))
		if lang == "" {
			return MetadataSettings{}, fmt.Errorf("%w: jazykový profil bez jazyka", ErrInvalidSetting)
		}
		if seenLang[lang] {
			return MetadataSettings{}, fmt.Errorf("%w: jazyk %q je v seznamu dvakrát", ErrInvalidSetting, lang)
		}
		seenLang[lang] = true
		ok, err := s.store.LanguageExists(ctx, lang)
		if err != nil {
			return MetadataSettings{}, err
		}
		if !ok {
			return MetadataSettings{}, fmt.Errorf("%w: neznámý kód jazyka %q", ErrInvalidSetting, lang)
		}
		if err := s.checkProviders(in.Languages[i].Providers); err != nil {
			return MetadataSettings{}, err
		}
		in.Languages[i].Language = lang
	}

	providers := s.normalize(in.Providers)
	if err := s.setJSON(ctx, SettingMetadataProviders, providers); err != nil {
		return MetadataSettings{}, err
	}

	languages := s.normalizeLanguages(in.Languages)
	if err := s.setJSON(ctx, SettingLanguageProviders, languages); err != nil {
		return MetadataSettings{}, err
	}

	key := strings.TrimSpace(in.GoogleBooksAPIKey)
	if err := s.setJSON(ctx, SettingGoogleBooksAPIKey, key); err != nil {
		return MetadataSettings{}, err
	}

	return MetadataSettings{Providers: providers, Languages: languages, GoogleBooksAPIKey: key}, nil
}

// checkProviders odmítne neznámý nebo zdvojený zdroj. Jména normalizuje na
// místě, aby je normalize převzal beze změny.
func (s *SettingsService) checkProviders(providers []ProviderSetting) error {
	seen := make(map[string]bool, len(providers))
	for i := range providers {
		name := strings.ToLower(strings.TrimSpace(providers[i].Name))
		if !s.isKnown(name) {
			return fmt.Errorf("%w: neznámý zdroj metadat %q", ErrInvalidSetting, providers[i].Name)
		}
		if seen[name] {
			return fmt.Errorf("%w: zdroj %q je v seznamu dvakrát", ErrInvalidSetting, name)
		}
		seen[name] = true
		providers[i].Name = name
	}
	return nil
}

func (s *SettingsService) setJSON(ctx context.Context, key string, v any) error {
	value, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.store.SetSetting(ctx, key, value)
}

// Profiles vrátí zapnuté zdroje pro každý profil a klíč Google Books –
// přesně to, co potřebuje metadata.Registry.Rebuild.
func (s *SettingsService) Profiles(ctx context.Context) (metadata.Profiles, string, error) {
	settings, err := s.Metadata(ctx)
	if err != nil {
		return metadata.Profiles{}, "", err
	}
	return settings.Profiles(), settings.GoogleBooksAPIKey, nil
}

// Library vrátí nastavení knihovny – z databáze, jinak výchozí z konfigurace.
func (s *SettingsService) Library(ctx context.Context) (LibrarySettings, error) {
	out := s.libraryDefaults

	raw, err := s.store.GetSetting(ctx, SettingLibrary)
	if errors.Is(err, storage.ErrNotFound) {
		return out, nil
	}
	if err != nil {
		return LibrarySettings{}, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return LibrarySettings{}, fmt.Errorf("nastavení knihovny: %w", err)
	}
	if out.DefaultLanguage == "" {
		out.DefaultLanguage = s.libraryDefaults.DefaultLanguage
	}
	return out, nil
}

// SetLibrary ověří a uloží nastavení knihovny. Vrací uloženou podobu.
func (s *SettingsService) SetLibrary(ctx context.Context, in LibrarySettings) (LibrarySettings, error) {
	in.DefaultLanguage = strings.ToLower(strings.TrimSpace(in.DefaultLanguage))
	if in.DefaultLanguage == "" {
		return LibrarySettings{}, fmt.Errorf("%w: chybí výchozí jazyk", ErrInvalidSetting)
	}
	ok, err := s.store.LanguageExists(ctx, in.DefaultLanguage)
	if err != nil {
		return LibrarySettings{}, err
	}
	if !ok {
		return LibrarySettings{}, fmt.Errorf("%w: neznámý kód jazyka %q", ErrInvalidSetting, in.DefaultLanguage)
	}
	if err := s.setJSON(ctx, SettingLibrary, in); err != nil {
		return LibrarySettings{}, err
	}
	return in, nil
}

// DefaultLanguage vrátí výchozí jazyk nových knih. Chyba čtení nastavení
// zakládání knih nezastaví – vrátí se prázdný řetězec a volající použije
// záložní jazyk.
func (s *SettingsService) DefaultLanguage(ctx context.Context) string {
	settings, err := s.Library(ctx)
	if err != nil {
		return ""
	}
	return settings.DefaultLanguage
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

// normalizeLanguages znormalizuje každý profil a zahodí profily bez jazyka
// nebo s jazykem, který už v seznamu byl. Profily se řadí podle kódu, ať je
// pořadí v administraci stálé.
func (s *SettingsService) normalizeLanguages(in []LanguageProfile) []LanguageProfile {
	out := make([]LanguageProfile, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, l := range in {
		lang := strings.ToLower(strings.TrimSpace(l.Language))
		if lang == "" || seen[lang] {
			continue
		}
		seen[lang] = true
		out = append(out, LanguageProfile{Language: lang, Providers: s.normalize(l.Providers)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Language < out[j].Language })
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
