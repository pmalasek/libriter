package handler

import (
	"errors"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/plugin"
	"mediater/internal/core/service"
)

// PluginsHandler obsluhuje správu pluginů metadat. Pluginy jsou společné
// všem produktům – stejný adresář, stejná konfigurace.
type PluginsHandler struct {
	host     *plugin.Host
	settings *plugin.Settings
	audit    *service.AuditService
}

func NewPlugins(host *plugin.Host, settings *plugin.Settings, audit *service.AuditService) *PluginsHandler {
	return &PluginsHandler{host: host, settings: settings, audit: audit}
}

type pluginInfo struct {
	Name      string              `json:"name"`
	Title     string              `json:"title"`
	Version   string              `json:"version"`
	Kinds     []string            `json:"kinds"`
	Languages []string            `json:"languages"`
	Path      string              `json:"path"`
	Config    []plugin.FieldState `json:"config"`
}

type pluginsResponse struct {
	Dir     string       `json:"dir"`
	Plugins []pluginInfo `json:"plugins"`
}

// GET /api/v1/admin/plugins  (admin)
func (h *PluginsHandler) List(w http.ResponseWriter, r *http.Request) {
	out := pluginsResponse{Dir: h.host.Dir(), Plugins: []pluginInfo{}}
	for _, p := range h.host.Plugins() {
		info, err := h.info(r, p)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání nastavení pluginu")
			return
		}
		out.Plugins = append(out.Plugins, info)
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *PluginsHandler) info(r *http.Request, p *plugin.Plugin) (pluginInfo, error) {
	state, err := h.settings.State(r.Context(), p)
	if err != nil {
		return pluginInfo{}, err
	}
	m := p.Manifest
	langs := m.Languages
	if langs == nil {
		langs = []string{}
	}
	return pluginInfo{Name: m.Name, Title: m.Title, Version: m.Version, Kinds: m.Kinds,
		Languages: langs, Path: p.Path, Config: state}, nil
}

// PUT /api/v1/admin/plugins/{name}/config  (admin)
//
// Tělo {"klíč": "hodnota" | null}. Chybějící klíč nebo null = beze změny,
// prázdný řetězec hodnotu smaže (pak platí proměnná prostředí).
func (h *PluginsHandler) SetConfig(w http.ResponseWriter, r *http.Request) {
	p, ok := h.host.Get(chi.URLParam(r, "name"))
	if !ok {
		writeError(w, http.StatusNotFound, "plugin.not_found", "plugin není nainstalovaný")
		return
	}
	var values map[string]*string
	if err := readJSON(r, &values); err != nil {
		writeError(w, http.StatusBadRequest, "request.invalid_body", "neplatný formát požadavku")
		return
	}
	if err := h.settings.Set(r.Context(), p, values); err != nil {
		if errors.Is(err, plugin.ErrInvalidConfig) {
			writeError(w, http.StatusBadRequest, "validation.invalid", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "common.save_failed", "chyba při ukládání nastavení")
		return
	}

	// Do auditu jdou jen názvy změněných klíčů – hodnoty bývají tajné.
	keys := make([]string, 0, len(values))
	for k, v := range values {
		if v != nil {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	details := map[string]string{"plugin": p.Name()}
	for _, k := range keys {
		details[k] = "changed"
	}
	h.audit.Record(r.Context(), ActorID(r), service.AuditEvent{
		Action:      service.AuditSettingsPlugin,
		TargetType:  service.AuditTargetSettings,
		TargetID:    plugin.SettingPrefix + p.Name(),
		TargetLabel: p.Manifest.Title,
		Details:     details,
	})

	info, err := h.info(r, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání nastavení pluginu")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// POST /api/v1/admin/plugins/reload  (admin)
//
// Znovu projde adresář pluginů – po instalaci nebo aktualizaci pluginu
// není potřeba restartovat server.
func (h *PluginsHandler) Reload(w http.ResponseWriter, r *http.Request) {
	if err := h.host.Discover(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "plugin.discover_failed", err.Error())
		return
	}
	h.audit.Record(r.Context(), ActorID(r), service.AuditEvent{
		Action:     service.AuditPluginsReload,
		TargetType: service.AuditTargetSettings,
		TargetID:   "plugins",
	})
	h.List(w, r)
}
