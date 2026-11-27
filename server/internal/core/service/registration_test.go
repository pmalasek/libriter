package service

import (
	"context"
	"testing"

	"mediater/internal/core/model"
)

// Nový server má registraci vypnutou, dokud ji admin nezapne.
func TestRegistrationDisabledByDefault(t *testing.T) {
	ctx := context.Background()
	svc := NewRegistration(newDeviceTestStore(t))

	got, err := svc.Registration(ctx)
	if err != nil {
		t.Fatalf("Registration: %v", err)
	}
	if got.Enabled || got.DefaultRole != model.RoleReader {
		t.Errorf("výchozí nastavení = %+v, chtěna vypnutá registrace s rolí reader", got)
	}

	if _, err := svc.SetRegistration(ctx, RegistrationSettings{Enabled: true, DefaultRole: model.RoleReader}); err != nil {
		t.Fatalf("SetRegistration: %v", err)
	}
	got, err = svc.Registration(ctx)
	if err != nil {
		t.Fatalf("Registration po uložení: %v", err)
	}
	if !got.Enabled {
		t.Errorf("nastavení = %+v, chtěna zapnutá registrace", got)
	}
}
