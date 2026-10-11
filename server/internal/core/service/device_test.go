package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/core/storage"

	"github.com/google/uuid"
)

func newDeviceTestStore(t *testing.T) *storage.Store {
	t.Helper()
	conn, err := db.Open(context.Background(), config.DBConfig{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return storage.New(conn)
}

func TestNormalizeUserCode(t *testing.T) {
	for in, want := range map[string]string{
		"acde-fghj":   "ACDE-FGHJ",
		" ACDE FGHJ ": "ACDE-FGHJ",
		"acdefghj":    "ACDE-FGHJ",
		"ACD":         "ACD",
	} {
		if got := NormalizeUserCode(in); got != want {
			t.Errorf("%q: %q, chci %q", in, got, want)
		}
	}
}

func TestDeviceFlow(t *testing.T) {
	ctx := context.Background()
	store := newDeviceTestStore(t)
	user, err := createUser(ctx, store, "Divák", "divak@example.com", "", "tajneheslo", "reader")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	clock := func() time.Time { return now }
	a := NewAuth(store, config.JWTConfig{Secret: "x"})
	a.now = clock

	start, err := a.StartDeviceAuth(ctx, "Obývák")
	if err != nil {
		t.Fatal(err)
	}
	if len(start.UserCode) != 9 || start.UserCode[4] != '-' || start.Interval != 5 {
		t.Fatalf("start: %+v", start)
	}

	if _, _, _, err := a.PollDeviceToken(ctx, start.DeviceCode); !errors.Is(err, ErrDevicePending) {
		t.Fatalf("první dotaz: %v", err)
	}
	now = now.Add(time.Second)
	if _, _, _, err := a.PollDeviceToken(ctx, start.DeviceCode); !errors.Is(err, ErrDeviceSlowDown) {
		t.Fatalf("častý dotaz: %v", err)
	}

	// Restart serveru: nová služba nad stejnou databází párování dokončí.
	b := NewAuth(store, config.JWTConfig{Secret: "x"})
	b.now = clock
	now = now.Add(6 * time.Second)
	info, err := b.DeviceByUserCode(ctx, start.UserCode)
	if err != nil || info.DeviceName != "Obývák" {
		t.Fatalf("info po restartu: %+v %v", info, err)
	}
	if _, err := b.ApproveDevice(ctx, uuid.New(), "nesmysl"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("neznámý kód: %v", err)
	}
	if _, err := b.ApproveDevice(ctx, user.ID, start.UserCode); err != nil {
		t.Fatalf("schválení: %v", err)
	}
	if _, err := b.ApproveDevice(ctx, user.ID, start.UserCode); !errors.Is(err, ErrNotFound) {
		t.Fatalf("druhé schválení: %v", err)
	}
	u, token, _, err := b.PollDeviceToken(ctx, start.DeviceCode)
	if err != nil || u.ID != user.ID {
		t.Fatalf("token: %v", err)
	}
	if c, err := b.ParseToken(token); err != nil || c.Scope != ScopeTV {
		t.Fatalf("TV token do API: %+v %v", c, err)
	}
	if _, err := b.ParseStreamToken(token); err == nil {
		t.Fatal("TV token nesmí projít jako streamovací")
	}
	now = now.Add(6 * time.Second)
	if _, _, _, err := b.PollDeviceToken(ctx, start.DeviceCode); !errors.Is(err, ErrDeviceExpired) {
		t.Fatalf("druhá výměna: %v", err)
	}
	if _, _, _, err := b.PollDeviceToken(ctx, "nesmysl"); !errors.Is(err, ErrDeviceExpired) {
		t.Fatalf("neznámý device_code: %v", err)
	}

	// Vypršení: po 10 minutách kód nejde schválit ani vyměnit.
	old, err := b.StartDeviceAuth(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Minute)
	if _, err := b.ApproveDevice(ctx, user.ID, old.UserCode); !errors.Is(err, ErrNotFound) {
		t.Fatalf("schválení prošlého kódu: %v", err)
	}
	if _, _, _, err := b.PollDeviceToken(ctx, old.DeviceCode); !errors.Is(err, ErrDeviceExpired) {
		t.Fatalf("prošlý kód: %v", err)
	}
}
