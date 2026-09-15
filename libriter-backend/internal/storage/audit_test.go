package storage

import (
	"context"
	"encoding/json"
	"testing"

	"libriter/internal/model"
)

func TestAuditListNewestFirstWithCursor(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	for _, action := range []string{"první", "druhá", "třetí"} {
		if _, err := store.InsertAudit(ctx, AuditInput{Action: action}); err != nil {
			t.Fatalf("InsertAudit %s: %v", action, err)
		}
	}

	page, err := store.ListAudit(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("na stránce %d záznamů, chtěny 2", len(page))
	}
	if page[0].Action != "třetí" || page[1].Action != "druhá" {
		t.Errorf("pořadí = %s, %s; chtěno třetí, druhá", page[0].Action, page[1].Action)
	}

	next, err := store.ListAudit(ctx, 2, page[1].ID)
	if err != nil {
		t.Fatalf("ListAudit s kurzorem: %v", err)
	}
	if len(next) != 1 || next[0].Action != "první" {
		t.Errorf("druhá stránka = %v, chtěna jen „první“", next)
	}
}

func TestAuditKeepsEmailAfterActorDeleted(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Druhý admin je tu proto, aby první šel smazat.
	actor, err := store.CreateUser(ctx, "Admin", "admin@example.com", "hash", model.RoleAdmin)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := store.CreateUser(ctx, "Admin 2", "admin2@example.com", "hash", model.RoleAdmin); err != nil {
		t.Fatalf("CreateUser druhého admina: %v", err)
	}

	if _, err := store.InsertAudit(ctx, AuditInput{
		ActorID:    &actor.ID,
		ActorEmail: actor.Email,
		Action:     "user.delete",
		Details:    json.RawMessage(`{"role":"reader"}`),
	}); err != nil {
		t.Fatalf("InsertAudit: %v", err)
	}

	if err := store.DeleteUser(ctx, actor.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	entries, err := store.ListAudit(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("záznamů = %d, chtěn 1", len(entries))
	}
	if entries[0].ActorID != nil {
		t.Errorf("actor_id = %v, chtěno nil po smazání účtu", entries[0].ActorID)
	}
	if entries[0].ActorEmail != "admin@example.com" {
		t.Errorf("actor_email = %q, chtěno admin@example.com", entries[0].ActorEmail)
	}
	if string(entries[0].Details) != `{"role":"reader"}` {
		t.Errorf("details = %s", entries[0].Details)
	}
}

func TestAuditDefaultDetails(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	entry, err := store.InsertAudit(ctx, AuditInput{Action: "scanner.rescan"})
	if err != nil {
		t.Fatalf("InsertAudit: %v", err)
	}
	if string(entry.Details) != "{}" {
		t.Errorf("details = %s, chtěno {}", entry.Details)
	}
}
