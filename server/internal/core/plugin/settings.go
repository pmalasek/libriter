package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// SettingPrefix je předpona klíče nastavení pluginu v tabulce settings.
const SettingPrefix = "plugins."

// SettingsStore je výřez core storage, který nastavení pluginů potřebuje.
type SettingsStore interface {
	GetSetting(ctx context.Context, key string) (json.RawMessage, error)
	SetSetting(ctx context.Context, key string, value json.RawMessage) error
}

// ErrInvalidConfig znamená neznámý klíč konfigurace.
var ErrInvalidConfig = errors.New("neplatná konfigurace pluginu")

// Settings ukládá konfiguraci pluginů do společné tabulky settings jádra.
// Hodnota uložená v administraci má přednost před proměnnou prostředí
// uvedenou v manifestu (ConfigField.Env).
type Settings struct {
	store    SettingsStore
	notFound error
}

// NewSettings vytvoří úložiště konfigurace; notFound je chyba, kterou store
// vrací pro chybějící klíč.
func NewSettings(store SettingsStore, notFound error) *Settings {
	return &Settings{store: store, notFound: notFound}
}

func (s *Settings) stored(ctx context.Context, name string) (map[string]string, error) {
	raw, err := s.store.GetSetting(ctx, SettingPrefix+name)
	if errors.Is(err, s.notFound) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("nastavení pluginu %s: %w", name, err)
	}
	return out, nil
}

// PluginConfig vrátí platnou konfiguraci pluginu (ConfigSource).
func (s *Settings) PluginConfig(ctx context.Context, p *Plugin) (map[string]string, error) {
	stored, err := s.stored(ctx, p.Name())
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(p.Manifest.Config))
	for _, f := range p.Manifest.Config {
		v := stored[f.Key]
		if v == "" && f.Env != "" {
			v = strings.TrimSpace(os.Getenv(f.Env))
		}
		if v != "" {
			out[f.Key] = v
		}
	}
	return out, nil
}

// FieldState je stav jedné položky konfigurace pro administraci. Tajná
// hodnota se nevrací, jen informace, že je nastavená.
type FieldState struct {
	ConfigField
	Value string `json:"value,omitempty"`
	IsSet bool   `json:"is_set"`
	// FromEnv říká, že hodnota pochází z proměnné prostředí, ne z administrace.
	FromEnv bool `json:"from_env"`
}

// State vrátí stav konfigurace pluginu pro administraci.
func (s *Settings) State(ctx context.Context, p *Plugin) ([]FieldState, error) {
	stored, err := s.stored(ctx, p.Name())
	if err != nil {
		return nil, err
	}
	out := make([]FieldState, 0, len(p.Manifest.Config))
	for _, f := range p.Manifest.Config {
		st := FieldState{ConfigField: f}
		v := stored[f.Key]
		if v == "" && f.Env != "" {
			if v = strings.TrimSpace(os.Getenv(f.Env)); v != "" {
				st.FromEnv = true
			}
		}
		st.IsSet = v != ""
		if !f.Secret {
			st.Value = v
		}
		out = append(out, st)
	}
	return out, nil
}

// Set uloží konfiguraci pluginu. Klíč, který v values chybí, zůstane beze
// změny (tajnou hodnotu klient nezná a posílat ji zpět nemusí); prázdný
// řetězec hodnotu smaže.
func (s *Settings) Set(ctx context.Context, p *Plugin, values map[string]*string) error {
	known := map[string]bool{}
	for _, f := range p.Manifest.Config {
		known[f.Key] = true
	}
	stored, err := s.stored(ctx, p.Name())
	if err != nil {
		return err
	}
	for k, v := range values {
		if !known[k] {
			return fmt.Errorf("%w: %s", ErrInvalidConfig, k)
		}
		if v == nil {
			continue
		}
		if t := strings.TrimSpace(*v); t != "" {
			stored[k] = t
		} else {
			delete(stored, k)
		}
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return s.store.SetSetting(ctx, SettingPrefix+p.Name(), raw)
}

// SetDefault uloží hodnotu jen tehdy, když plugin ještě žádnou nemá.
// Slouží k jednorázovému převzetí dřívějšího nastavení (klíč Google Books).
func (s *Settings) SetDefault(ctx context.Context, name, key, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	stored, err := s.stored(ctx, name)
	if err != nil {
		return err
	}
	if stored[key] != "" {
		return nil
	}
	stored[key] = strings.TrimSpace(value)
	raw, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return s.store.SetSetting(ctx, SettingPrefix+name, raw)
}

// Value vrátí uloženou hodnotu jednoho klíče (bez výchozí z prostředí).
func (s *Settings) Value(ctx context.Context, name, key string) (string, error) {
	stored, err := s.stored(ctx, name)
	if err != nil {
		return "", err
	}
	return stored[key], nil
}

// SetValue uloží hodnotu jednoho klíče; prázdná hodnota ho smaže.
func (s *Settings) SetValue(ctx context.Context, name, key, value string) error {
	stored, err := s.stored(ctx, name)
	if err != nil {
		return err
	}
	if v := strings.TrimSpace(value); v != "" {
		stored[key] = v
	} else {
		delete(stored, key)
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	return s.store.SetSetting(ctx, SettingPrefix+name, raw)
}
