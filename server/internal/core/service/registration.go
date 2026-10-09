package service

import "context"

// RegistrationSettings řídí veřejnou registraci.
type RegistrationSettings struct {
	Enabled     bool   `json:"enabled"`
	DefaultRole string `json:"default_role"`
}

// RegistrationSource dodává aktuální nastavení registrace. Implementuje ho
// služba nastavení; handler přihlášení na ní tak nezávisí přímo.
type RegistrationSource interface {
	Registration(ctx context.Context) (RegistrationSettings, error)
}
