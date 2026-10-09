package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"mediater/internal/core/model"
	"mediater/internal/core/storage"
)

// SettingRegistration je klíč nastavení registrace v tabulce settings.
const SettingRegistration = "registration"

// RegistrationSettings řídí veřejnou registraci.
type RegistrationSettings struct {
	Enabled     bool   `json:"enabled"`
	DefaultRole string `json:"default_role"`
}

// RegistrationSource dodává aktuální nastavení registrace. Handler přihlášení
// díky tomu nezávisí na konkrétní službě.
type RegistrationSource interface {
	Registration(ctx context.Context) (RegistrationSettings, error)
}

// RegistrationService čte a zapisuje nastavení veřejné registrace. Platí pro
// všechny produkty – účet je společný.
type RegistrationService struct {
	store *storage.Store
}

func NewRegistration(store *storage.Store) *RegistrationService {
	return &RegistrationService{store: store}
}

// Registration vrátí nastavení registrace (výchozí: zapnutá, role reader).
func (s *RegistrationService) Registration(ctx context.Context) (RegistrationSettings, error) {
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
func (s *RegistrationService) SetRegistration(ctx context.Context, in RegistrationSettings) (RegistrationSettings, error) {
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
