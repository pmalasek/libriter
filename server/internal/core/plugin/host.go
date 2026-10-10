package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// callTimeout je strop jednoho volání pluginu (scraper může čekat na
	// odstup mezi požadavky na cizí web).
	callTimeout = 60 * time.Second
	// describeTimeout je strop příkazu describe – ten nesmí chodit na síť.
	describeTimeout = 10 * time.Second
	// maxOutput je strop odpovědi pluginu.
	maxOutput = 4 << 20
	// maxStderr je kolik ze stderr pluginu se drží pro log a chybovou zprávu.
	maxStderr = 16 << 10
	// maxImage je strop staženého obrázku.
	maxImage = 20 << 20
	// imageGap je odstup mezi obrázky téhož pluginu.
	imageGap = 200 * time.Millisecond
	// Pauza pluginu po odmítnutí zdrojem (HTTP 429, ochrana proti botům):
	// začíná na backoffMin, s každým dalším odmítnutím se zdvojí až na
	// backoffMax, úspěšné volání ji vynuluje.
	backoffMin = time.Minute
	backoffMax = 30 * time.Minute
)

// ErrUnknownPlugin znamená, že plugin s daným jménem není nainstalovaný.
var ErrUnknownPlugin = errors.New("plugin není nainstalovaný")

// ErrNotAllowed znamená, že adresa nepatří pluginu (allowlist).
var ErrNotAllowed = errors.New("adresa nepatří žádnému zdroji metadat")

// ErrUnsupported znamená, že plugin daný druh záznamů neumí.
var ErrUnsupported = errors.New("zdroj metadat tento druh záznamů neumí")

// ConfigSource dodá konfiguraci pluginu (API klíče) pro jedno volání.
type ConfigSource interface {
	PluginConfig(ctx context.Context, p *Plugin) (map[string]string, error)
}

// Plugin je jeden nainstalovaný plugin.
type Plugin struct {
	Manifest Manifest
	Path     string

	patterns map[string][]*regexp.Regexp

	// mu drží jedno volání pluginu najednou a s ním odstup mezi voláními.
	mu   sync.Mutex
	last time.Time

	// imgMu drží odstup mezi staženými obrázky (nezávisle na voláních).
	imgMu   sync.Mutex
	imgLast time.Time

	// Pauza po odmítnutí zdrojem – společná pro volání i obrázky.
	pauseMu     sync.Mutex
	pausedUntil time.Time
	backoff     time.Duration
}

// PausedUntil vrátí, do kdy je plugin po odmítnutí zdrojem pozastavený
// (nulový čas = není).
func (p *Plugin) PausedUntil() time.Time {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()
	if time.Now().After(p.pausedUntil) {
		return time.Time{}
	}
	return p.pausedUntil
}

// pausedErr vrátí chybu rate_limited, dokud pauza trvá.
func (p *Plugin) pausedErr() error {
	if until := p.PausedUntil(); !until.IsZero() {
		return &Error{Plugin: p.Manifest.Name, Code: ErrCodeRateLimited,
			Message: "zdroj odmítá požadavky, pozastaveno do " + until.Format("15:04:05")}
	}
	return nil
}

// penalize prodlouží pauzu po odmítnutí zdrojem a vrátí její délku.
func (p *Plugin) penalize(lo, hi time.Duration) time.Duration {
	p.pauseMu.Lock()
	defer p.pauseMu.Unlock()
	p.backoff = min(max(lo, 2*p.backoff), hi)
	p.pausedUntil = time.Now().Add(p.backoff)
	return p.backoff
}

// recovered vynuluje pauzu po úspěšném volání.
func (p *Plugin) recovered() {
	p.pauseMu.Lock()
	p.backoff = 0
	p.pauseMu.Unlock()
}

// Name je jméno pluginu z manifestu.
func (p *Plugin) Name() string { return p.Manifest.Name }

// SupportsURL říká, jestli plugin umí stáhnout detail z adresy daného druhu.
func (p *Plugin) SupportsURL(kind, rawURL string) bool {
	if !p.Manifest.Supports(kind) {
		return false
	}
	if u, err := url.Parse(rawURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	for _, re := range p.patterns[kind] {
		if re.MatchString(rawURL) {
			return true
		}
	}
	return false
}

// SupportsImageURL říká, jestli obrázek z adresy smí server stáhnout.
func (p *Plugin) SupportsImageURL(rawURL string) bool {
	for _, h := range p.Manifest.ImageHosts {
		if HostMatches(rawURL, h) {
			return true
		}
	}
	return false
}

// pluginSet je neměnná sada objevených pluginů.
type pluginSet struct {
	byName map[string]*Plugin
	sorted []*Plugin
}

// Host objevuje a spouští pluginy.
type Host struct {
	dir    string
	config ConfigSource
	http   *http.Client
	log    *slog.Logger

	set atomic.Pointer[pluginSet]

	listenMu  sync.Mutex
	listeners []func()
	// timeout a pauza po odmítnutí jdou v testech zkrátit.
	timeout    time.Duration
	backoffMin time.Duration
	backoffMax time.Duration
}

// NewHost vytvoří hostitele nad adresářem pluginů. Pluginy se načtou až
// voláním Discover.
func NewHost(dir string) *Host {
	h := &Host{
		dir:        dir,
		http:       &http.Client{Timeout: 30 * time.Second},
		log:        slog.Default().With("component", "plugins"),
		timeout:    callTimeout,
		backoffMin: backoffMin,
		backoffMax: backoffMax,
	}
	h.set.Store(&pluginSet{byName: map[string]*Plugin{}})
	return h
}

// SetConfigSource nastaví zdroj konfigurace pluginů.
func (h *Host) SetConfigSource(c ConfigSource) { h.config = c }

// OnChange zaregistruje funkci volanou po každém Discover – produkty podle
// ní přestaví seznamy zdrojů.
func (h *Host) OnChange(fn func()) {
	h.listenMu.Lock()
	h.listeners = append(h.listeners, fn)
	h.listenMu.Unlock()
}

// Dir vrátí adresář pluginů.
func (h *Host) Dir() string { return h.dir }

// Discover projde adresář pluginů, zeptá se každého na manifest a nasadí
// novou sadu. Plugin, který neodpoví nebo má neplatný manifest, se přeskočí
// s varováním. Chybějící adresář znamená prázdnou sadu.
func (h *Host) Discover(ctx context.Context) error {
	next := &pluginSet{byName: map[string]*Plugin{}}
	defer func() {
		sort.Slice(next.sorted, func(i, j int) bool { return next.sorted[i].Name() < next.sorted[j].Name() })
		h.set.Store(next)
		h.listenMu.Lock()
		listeners := append([]func(){}, h.listeners...)
		h.listenMu.Unlock()
		for _, fn := range listeners {
			fn()
		}
	}()

	if h.dir == "" {
		return nil
	}
	entries, err := os.ReadDir(h.dir)
	if errors.Is(err, os.ErrNotExist) {
		h.log.Warn("adresář pluginů neexistuje", "dir", h.dir)
		return nil
	}
	if err != nil {
		return fmt.Errorf("adresář pluginů %s: %w", h.dir, err)
	}

	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), FilePrefix) {
			continue
		}
		path := filepath.Join(h.dir, e.Name())
		p, err := h.load(ctx, path)
		if err != nil {
			h.log.Warn("plugin přeskočen", "path", path, "err", err)
			continue
		}
		if _, dup := next.byName[p.Name()]; dup {
			h.log.Warn("plugin se stejným jménem už je načtený", "path", path, "name", p.Name())
			continue
		}
		next.byName[p.Name()] = p
		next.sorted = append(next.sorted, p)
		h.log.Info("plugin načten", "name", p.Name(), "version", p.Manifest.Version, "kinds", p.Manifest.Kinds)
	}
	return nil
}

// load ověří soubor a načte manifest.
func (h *Host) load(ctx context.Context, path string) (*Plugin, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("není běžný soubor")
	}
	if fi.Mode().Perm()&0o100 == 0 {
		return nil, errors.New("není spustitelný")
	}
	// Soubor, do kterého smí zapisovat kdokoli další, by mohl kdokoli
	// vyměnit za vlastní program spouštěný serverem.
	if fi.Mode().Perm()&0o022 != 0 {
		return nil, errors.New("je zapisovatelný pro skupinu nebo ostatní")
	}

	p := &Plugin{Path: path}
	ctx, cancel := context.WithTimeout(ctx, describeTimeout)
	defer cancel()
	if err := h.run(ctx, p, CmdDescribe, nil, &p.Manifest); err != nil {
		return nil, err
	}
	patterns, err := p.Manifest.validate()
	if err != nil {
		return nil, err
	}
	want := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), FilePrefix), ".exe")
	if p.Manifest.Name != want {
		return nil, fmt.Errorf("jméno v manifestu %q neodpovídá souboru (%q)", p.Manifest.Name, want)
	}
	p.patterns = patterns
	return p, nil
}

// Plugins vrátí všechny načtené pluginy seřazené podle jména.
func (h *Host) Plugins() []*Plugin {
	return append([]*Plugin(nil), h.set.Load().sorted...)
}

// ByKind vrátí pluginy, které umí daný druh záznamů.
func (h *Host) ByKind(kind string) []*Plugin {
	var out []*Plugin
	for _, p := range h.set.Load().sorted {
		if p.Manifest.Supports(kind) {
			out = append(out, p)
		}
	}
	return out
}

// Get najde plugin podle jména.
func (h *Host) Get(name string) (*Plugin, bool) {
	p, ok := h.set.Load().byName[name]
	return p, ok
}

func (h *Host) plugin(name, kind string) (*Plugin, error) {
	p, ok := h.Get(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPlugin, name)
	}
	if kind != "" && !p.Manifest.Supports(kind) {
		return nil, fmt.Errorf("%w: %s/%s", ErrUnsupported, name, kind)
	}
	return p, nil
}

// Search zavolá příkaz search.
func (h *Host) Search(ctx context.Context, name string, req Request) ([]SearchResult, error) {
	p, err := h.plugin(name, req.Kind)
	if err != nil {
		return nil, err
	}
	var out SearchResponse
	if err := h.call(ctx, p, CmdSearch, req, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// Fetch zavolá příkaz fetch a rozbalí položku do item. Fetch podle URL
// projde jen adresa z allowlistu pluginu.
func (h *Host) Fetch(ctx context.Context, name string, req Request, item any) error {
	p, err := h.plugin(name, req.Kind)
	if err != nil {
		return err
	}
	if req.URL != "" && !p.SupportsURL(req.Kind, req.URL) {
		return ErrNotAllowed
	}
	var out FetchResponse
	if err := h.call(ctx, p, CmdFetch, req, &out); err != nil {
		return err
	}
	if len(out.Item) == 0 || string(out.Item) == "null" {
		return &Error{Plugin: name, Code: ErrCodeNotFound, Message: "prázdná odpověď"}
	}
	if err := json.Unmarshal(out.Item, item); err != nil {
		return fmt.Errorf("plugin %s: neplatná položka: %w", name, err)
	}
	return nil
}

// Season zavolá příkaz season.
func (h *Host) Season(ctx context.Context, name string, req Request) (*SeasonMetadata, error) {
	req.Kind = KindShow
	p, err := h.plugin(name, KindShow)
	if err != nil {
		return nil, err
	}
	var out SeasonResponse
	if err := h.call(ctx, p, CmdSeason, req, &out); err != nil {
		return nil, err
	}
	return &out.Season, nil
}

// DownloadImage stáhne obrázek z adresy, kterou vrátil plugin. Adresa musí
// patřit mezi ImageHosts pluginu.
func (h *Host) DownloadImage(ctx context.Context, name, rawURL string) ([]byte, string, error) {
	p, err := h.plugin(name, "")
	if err != nil {
		return nil, "", err
	}
	if !p.SupportsImageURL(rawURL) {
		return nil, "", ErrNotAllowed
	}
	if err := p.pausedErr(); err != nil {
		return nil, "", err
	}

	p.imgMu.Lock()
	defer p.imgMu.Unlock()
	if wait := time.Until(p.imgLast.Add(imageGap)); wait > 0 {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(wait):
		}
	}
	defer func() { p.imgLast = time.Now() }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mediater/1.0")
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
		d := p.penalize(h.backoffMin, h.backoffMax)
		h.log.Warn("zdroj odmítá obrázky, plugin pozastaven", "plugin", p.Name(), "status", resp.StatusCode, "pauza", d)
		return nil, "", &Error{Plugin: p.Name(), Code: ErrCodeRateLimited,
			Message: fmt.Sprintf("obrázek %s: HTTP %d", rawURL, resp.StatusCode)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("obrázek %s: HTTP %d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImage+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) > maxImage {
		return nil, "", fmt.Errorf("obrázek %s je větší než %d MB", rawURL, maxImage>>20)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// call doplní konfiguraci, počká na odstup a spustí plugin.
func (h *Host) call(ctx context.Context, p *Plugin, cmd string, req Request, out any) error {
	if h.config != nil {
		cfg, err := h.config.PluginConfig(ctx, p)
		if err != nil {
			return fmt.Errorf("konfigurace pluginu %s: %w", p.Name(), err)
		}
		req.Config = cfg
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if gap := time.Duration(p.Manifest.MinIntervalMS) * time.Millisecond; gap > 0 {
		if wait := time.Until(p.last.Add(gap)); wait > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
	}
	defer func() { p.last = time.Now() }()

	// Po odmítnutí zdrojem se nečeká (ruční párování z administrace by
	// viselo), volání rovnou skončí chybou rate_limited.
	if err := p.pausedErr(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	err := h.run(ctx, p, cmd, req, out)
	switch {
	case IsRateLimited(err):
		d := p.penalize(h.backoffMin, h.backoffMax)
		h.log.Warn("zdroj odmítá požadavky, plugin pozastaven", "plugin", p.Name(), "cmd", cmd, "pauza", d, "err", err)
	case err == nil:
		p.recovered()
	}
	return err
}

// run spustí plugin s jedním příkazem a přečte odpověď.
func (h *Host) run(ctx context.Context, p *Plugin, cmd string, in any, out any) error {
	var stdin []byte
	if in != nil {
		var err error
		if stdin, err = json.Marshal(in); err != nil {
			return err
		}
	} else {
		stdin = []byte("{}")
	}

	stdout := &limitedBuffer{max: maxOutput}
	stderr := &limitedBuffer{max: maxStderr, truncate: true}
	c := exec.CommandContext(ctx, p.Path, cmd)
	c.Stdin = bytes.NewReader(stdin)
	c.Stdout = stdout
	c.Stderr = stderr
	c.Env = pluginEnv()
	c.WaitDelay = 2 * time.Second

	runErr := c.Run()
	if s := strings.TrimSpace(stderr.String()); s != "" {
		h.log.Debug("stderr pluginu", "plugin", p.Path, "cmd", cmd, "stderr", s)
	}
	if stdout.overflow {
		return fmt.Errorf("plugin %s: odpověď je větší než %d MB", p.Path, maxOutput>>20)
	}
	if ctx.Err() != nil {
		return fmt.Errorf("plugin %s %s: %w", filepath.Base(p.Path), cmd, ctx.Err())
	}

	var errResp ErrorResponse
	_ = json.Unmarshal(stdout.Bytes(), &errResp)
	if errResp.Error != nil {
		return &Error{Plugin: p.Manifest.Name, Code: errResp.Error.Code, Message: errResp.Error.Message}
	}
	if runErr != nil {
		msg := lastLine(stderr.String())
		if msg == "" {
			msg = runErr.Error()
		}
		return &Error{Plugin: filepath.Base(p.Path), Code: ErrCodeUnavailable, Message: msg}
	}
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		return fmt.Errorf("plugin %s %s: neplatný JSON: %w", filepath.Base(p.Path), cmd, err)
	}
	return nil
}

// pluginEnv je prostředí pluginu. Celé prostředí serveru se nepředává –
// obsahuje JWT_SECRET a další tajemství, která plugin nepotřebuje.
func pluginEnv() []string {
	keep := []string{"PATH", "HOME", "TMPDIR", "LANG", "LC_ALL", "TZ",
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy",
		"SSL_CERT_FILE", "SSL_CERT_DIR"}
	var env []string
	for _, k := range keep {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

// limitedBuffer je bytes.Buffer se stropem. Bez truncate přetečení zaznamená
// a další data zahodí; s truncate drží jen začátek (stderr).
type limitedBuffer struct {
	bytes.Buffer
	max      int
	truncate bool
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := b.max - b.Len(); room < len(p) {
		if room > 0 {
			b.Buffer.Write(p[:room])
		}
		if !b.truncate {
			b.overflow = true
		}
		return n, nil
	}
	b.Buffer.Write(p)
	return n, nil
}
