package storage

import (
	"context"
	"errors"
	"testing"

	"libriter/internal/model"
)

// Poslední admin musí zůstat: jinak by knihovna zůstala bez správce a role
// by šla vrátit jen přes CLI.
func TestLastAdminCannotBeDemotedOrDeleted(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	admin, err := store.CreateUser(ctx, "Admin", "admin@example.com", "hash", model.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := store.CreateUser(ctx, "Čtenář", "reader@example.com", "hash", model.RoleReader); err != nil {
		t.Fatalf("CreateUser čtenáře: %v", err)
	}

	if err := store.SetUserRole(ctx, admin.ID, model.RoleReader); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("SetUserRole posledního admina = %v, chtěno ErrLastAdmin", err)
	}
	if err := store.DeleteUser(ctx, admin.ID); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("DeleteUser posledního admina = %v, chtěno ErrLastAdmin", err)
	}

	// Role zůstala nedotčená i po odmítnutém pokusu.
	role, err := store.GetUserRole(ctx, admin.ID)
	if err != nil {
		t.Fatalf("GetUserRole: %v", err)
	}
	if role != model.RoleAdmin {
		t.Errorf("role = %q, chtěno admin", role)
	}
}

func TestSecondAdminAllowsDemotionAndDeletion(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	first, err := store.CreateUser(ctx, "Admin", "admin@example.com", "hash", model.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	second, err := store.CreateUser(ctx, "Admin 2", "admin2@example.com", "hash", model.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser druhého admina: %v", err)
	}

	if err := store.SetUserRole(ctx, first.ID, model.RoleEditor); err != nil {
		t.Fatalf("SetUserRole: %v", err)
	}
	// Po snížení role prvního zbyl jediný admin – toho už smazat nejde.
	if err := store.DeleteUser(ctx, second.ID); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("DeleteUser posledního admina = %v, chtěno ErrLastAdmin", err)
	}
	// Uživatel bez role admin se maže bez omezení.
	if err := store.DeleteUser(ctx, first.ID); err != nil {
		t.Errorf("DeleteUser editora: %v", err)
	}
}

func TestCountUsersByRole(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if _, err := store.CreateUser(ctx, "Admin", "admin@example.com", "hash", model.RoleAdmin); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := store.CreateUser(ctx, "Editor", "editor@example.com", "hash", model.RoleEditor); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	admins, err := store.CountUsersByRole(ctx, model.RoleAdmin)
	if err != nil {
		t.Fatalf("CountUsersByRole: %v", err)
	}
	if admins != 1 {
		t.Errorf("adminů = %d, chtěn 1", admins)
	}

	readers, err := store.CountUsersByRole(ctx, model.RoleReader)
	if err != nil {
		t.Fatalf("CountUsersByRole: %v", err)
	}
	if readers != 0 {
		t.Errorf("čtenářů = %d, chtěno 0", readers)
	}
}
