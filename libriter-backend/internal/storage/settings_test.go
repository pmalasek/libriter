package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestSettingsMissingKey(t *testing.T) {
	store := newTestStore(t)

	if _, err := store.GetSetting(context.Background(), "neexistuje"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSetting chybějícího klíče = %v, chtěno ErrNotFound", err)
	}
}

func TestSettingsRoundTripAndOverwrite(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if err := store.SetSetting(ctx, "zdroje", json.RawMessage(`["a","b"]`)); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	raw, err := store.GetSetting(ctx, "zdroje")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if string(raw) != `["a","b"]` {
		t.Errorf("hodnota = %s, chtěno [\"a\",\"b\"]", raw)
	}

	// Druhý zápis stejného klíče hodnotu přepíše, nezaloží další řádek.
	if err := store.SetSetting(ctx, "zdroje", json.RawMessage(`["c"]`)); err != nil {
		t.Fatalf("SetSetting podruhé: %v", err)
	}
	raw, err = store.GetSetting(ctx, "zdroje")
	if err != nil {
		t.Fatalf("GetSetting po přepisu: %v", err)
	}
	if string(raw) != `["c"]` {
		t.Errorf("hodnota po přepisu = %s, chtěno [\"c\"]", raw)
	}
}
