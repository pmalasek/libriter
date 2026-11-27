package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/config"
	"mediater/internal/core/middleware"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/model"
	"mediater/internal/libriter/service"
)

// memKeyStore nahrazuje konfiguraci pluginu googlebooks (plugin.Settings).
type memKeyStore struct{ values map[string]string }

func (m *memKeyStore) Value(_ context.Context, plugin, key string) (string, error) {
	return m.values[plugin+"/"+key], nil
}

func (m *memKeyStore) SetValue(_ context.Context, plugin, key, value string) error {
	if v := strings.TrimSpace(value); v != "" {
		m.values[plugin+"/"+key] = v
	} else {
		delete(m.values, plugin+"/"+key)
	}
	return nil
}

type metadataSettingsBody struct {
	Providers []struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	} `json:"providers"`
	KeySet     *bool `json:"google_books_api_key_set"`
	KeyFromEnv *bool `json:"google_books_api_key_from_env"`
}

// newMetadataSettingsEnv postaví admin router jen s nastavením zdrojů metadat
// nad nastavením s danou konfigurací (.env) a klíčem v konfiguraci pluginu.
func newMetadataSettingsEnv(t *testing.T, cfg config.MetadataConfig) (*adminTestEnv, *memKeyStore, string) {
	t.Helper()
	env := newAdminTestEnv(t)
	keys := &memKeyStore{values: map[string]string{}}
	settingsSvc := service.NewSettings(env.store, cfg, config.LibraryConfig{}, []string{"googlebooks", "openlibrary"}).
		WithGoogleKeyStore(keys)
	registry := metadata.NewRegistry(map[string]metadata.Factory{})
	adminH := NewAdmin(env.users, settingsSvc, registry, env.scanner,
		service.NewSystem(env.store, &config.Config{}, time.Now()), env.audit, service.NewListening(env.store))

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(env.auth))
		r.Use(middleware.RequireRole(model.RoleAdmin))
		r.Get("/admin/settings/metadata", adminH.MetadataSettings)
		r.Put("/admin/settings/metadata", adminH.SetMetadataSettings)
	})
	env.router = r
	token, _ := env.login(t, "admin@example.com", model.RoleAdmin)
	return env, keys, token
}

func (e *adminTestEnv) metadataSettings(t *testing.T, method, token string, body any) (metadataSettingsBody, string) {
	t.Helper()
	rec := e.do(t, method, "/admin/settings/metadata", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s nastavení zdrojů: %d %s", method, rec.Code, rec.Body.String())
	}
	var out metadataSettingsBody
	decodeJSON(t, rec.Body.Bytes(), &out)
	if out.KeySet == nil || out.KeyFromEnv == nil {
		t.Fatalf("odpověď bez google_books_api_key_set/_from_env: %s", rec.Body.String())
	}
	return out, rec.Body.String()
}

const googleKey = "AIza-tajny-klic"

// Uložení pořadí zdrojů bez klíče (tak ho posílá web, když klíč neměníte)
// klíč zachová; API klíč nikdy nevrací.
func TestMetadataSettingsKeepsGoogleKey(t *testing.T) {
	env, keys, token := newMetadataSettingsEnv(t, config.MetadataConfig{Providers: []string{"googlebooks"}})
	providers := []map[string]any{{"name": "openlibrary", "enabled": true}, {"name": "googlebooks", "enabled": true}}

	got, raw := env.metadataSettings(t, http.MethodPut, token, map[string]any{
		"providers": providers, "languages": []any{}, "google_books_api_key": "  " + googleKey + "  ",
	})
	if !*got.KeySet || *got.KeyFromEnv {
		t.Errorf("po nastavení klíče: set=%v from_env=%v", *got.KeySet, *got.KeyFromEnv)
	}
	if strings.Contains(raw, googleKey) {
		t.Errorf("PUT vrátil klíč: %s", raw)
	}

	// Jen pořadí zdrojů – pole klíče chybí.
	providers[0], providers[1] = providers[1], providers[0]
	got, _ = env.metadataSettings(t, http.MethodPut, token, map[string]any{"providers": providers, "languages": []any{}})
	if len(got.Providers) == 0 || got.Providers[0].Name != "googlebooks" {
		t.Errorf("pořadí zdrojů se neuložilo: %+v", got.Providers)
	}
	if !*got.KeySet {
		t.Error("uložení pořadí bez klíče klíč smazalo")
	}
	if v := keys.values["googlebooks/api_key"]; v != googleKey {
		t.Errorf("klíč v konfiguraci pluginu = %q", v)
	}

	// null znamená totéž co chybějící pole.
	got, _ = env.metadataSettings(t, http.MethodPut, token, map[string]any{"providers": providers, "google_books_api_key": nil})
	if !*got.KeySet {
		t.Error("null klíč smazal")
	}

	got, raw = env.metadataSettings(t, http.MethodGet, token, nil)
	if !*got.KeySet || strings.Contains(raw, googleKey) {
		t.Errorf("GET: set=%v, odpověď %s", *got.KeySet, raw)
	}

	// Prázdný řetězec klíč smaže.
	got, _ = env.metadataSettings(t, http.MethodPut, token, map[string]any{"providers": providers, "google_books_api_key": ""})
	if *got.KeySet {
		t.Error("prázdný řetězec klíč nesmazal")
	}
	if _, ok := keys.values["googlebooks/api_key"]; ok {
		t.Error("klíč zůstal v konfiguraci pluginu")
	}
}

// Klíč z .env se hlásí jako nastavený z konfigurace a uložení bez klíče ho
// nepřebije prázdnou hodnotou.
func TestMetadataSettingsKeyFromEnv(t *testing.T) {
	env, _, token := newMetadataSettingsEnv(t, config.MetadataConfig{
		Providers: []string{"googlebooks"}, GoogleBooksAPIKey: googleKey,
	})

	got, raw := env.metadataSettings(t, http.MethodGet, token, nil)
	if !*got.KeySet || !*got.KeyFromEnv {
		t.Errorf("klíč z .env: set=%v from_env=%v", *got.KeySet, *got.KeyFromEnv)
	}
	if strings.Contains(raw, googleKey) {
		t.Errorf("GET vrátil klíč z .env: %s", raw)
	}

	got, _ = env.metadataSettings(t, http.MethodPut, token, map[string]any{
		"providers": []map[string]any{{"name": "openlibrary", "enabled": true}},
	})
	if !*got.KeySet || !*got.KeyFromEnv {
		t.Errorf("po uložení bez klíče: set=%v from_env=%v", *got.KeySet, *got.KeyFromEnv)
	}

	// Vlastní klíč z administrace má přednost před .env.
	got, _ = env.metadataSettings(t, http.MethodPut, token, map[string]any{
		"providers": []map[string]any{{"name": "openlibrary", "enabled": true}}, "google_books_api_key": "jiny-klic",
	})
	if !*got.KeySet || *got.KeyFromEnv {
		t.Errorf("po uložení vlastního klíče: set=%v from_env=%v", *got.KeySet, *got.KeyFromEnv)
	}
}
