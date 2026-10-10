package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"

	"mediater/internal/core/plugin"
	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/scanner"
	"mediater/internal/sceniter/storage"
)

// ErrConflict znamená, že složka už knihovnou je.
var ErrConflict = errors.New("složka už je knihovnou")

var (
	languageRe = regexp.MustCompile(`^[a-z]{2,3}$`)
	// Název ikony z nabídky webu; server ho jen ukládá.
	iconRe = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)
)

// Libraries spravuje knihovny – podsložky VIDEO_ROOT s typem obsahu, jazykem
// a zdrojem metadat. Nastavení je globální a mění ho jen admin.
type Libraries struct {
	store   *storage.Store
	root    string
	plugins *plugin.Host
	// changed se zavolá po každé změně (spustí průchod knihovnou).
	changed func()
}

func NewLibraries(store *storage.Store, root string, plugins *plugin.Host, changed func()) *Libraries {
	if changed == nil {
		changed = func() {}
	}
	return &Libraries{store: store, root: root, plugins: plugins, changed: changed}
}

// List vrátí knihovny; onlyEnabled vynechá vypnuté.
func (l *Libraries) List(ctx context.Context, onlyEnabled bool) ([]model.Library, error) {
	return l.store.ListLibraries(ctx, onlyEnabled)
}

// Candidates vrátí podsložky VIDEO_ROOT, které ještě nejsou knihovnou.
func (l *Libraries) Candidates(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(l.root)
	if err != nil {
		return nil, fmt.Errorf("VIDEO_ROOT: %w", err)
	}
	libs, err := l.store.ListLibraries(ctx, false)
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, lib := range libs {
		used[lib.Dir] = true
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() || scanner.Ignored(e.Name()) || used[e.Name()] {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

// LibraryRequest jsou vlastnosti knihovny od admina. Dir a Kind se berou
// jen při založení.
type LibraryRequest struct {
	Dir      string  `json:"dir"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Language string  `json:"language"`
	Provider *string `json:"provider"`
	// Icon: nil = beze změny, "" = výchozí podle typu.
	Icon    *string `json:"icon"`
	Enabled *bool   `json:"enabled"`
}

func (l *Libraries) validate(kind string, in *LibraryRequest, out *storage.LibraryInput) error {
	out.Name = strings.TrimSpace(in.Name)
	if out.Name == "" {
		out.Name = out.Dir
	}
	if len([]rune(out.Name)) > 100 {
		return fmt.Errorf("%w: název je příliš dlouhý", ErrInvalid)
	}
	out.Language = strings.ToLower(strings.TrimSpace(in.Language))
	if out.Language == "" {
		out.Language = "cs"
	}
	if !languageRe.MatchString(out.Language) {
		return fmt.Errorf("%w: jazyk musí být kód ISO 639-1 (cs, en, …)", ErrInvalid)
	}
	if in.Provider != nil {
		out.Provider = strings.TrimSpace(*in.Provider)
	}
	if out.Provider != "" {
		if l.plugins == nil {
			return fmt.Errorf("%w: zdroj metadat %q není nainstalovaný", ErrInvalid, out.Provider)
		}
		p, ok := l.plugins.Get(out.Provider)
		if !ok {
			return fmt.Errorf("%w: zdroj metadat %q není nainstalovaný", ErrInvalid, out.Provider)
		}
		if !p.Manifest.Supports(kind) {
			return fmt.Errorf("%w: zdroj %q neumí %s", ErrInvalid, out.Provider, kindLabel(kind))
		}
	}
	if in.Icon != nil {
		out.Icon = strings.TrimSpace(*in.Icon)
	}
	if out.Icon != "" && !iconRe.MatchString(out.Icon) {
		return fmt.Errorf("%w: neplatná ikona %q", ErrInvalid, out.Icon)
	}
	if in.Enabled != nil {
		out.Enabled = *in.Enabled
	}
	return nil
}

func kindLabel(kind string) string {
	if kind == model.KindShow {
		return "seriály"
	}
	return "filmy"
}

// Create založí knihovnu ze složky přímo pod VIDEO_ROOT.
func (l *Libraries) Create(ctx context.Context, in LibraryRequest) (*model.Library, error) {
	dir := strings.TrimSpace(in.Dir)
	if dir == "" || dir == "." || dir == ".." || strings.ContainsAny(dir, `/\`) || scanner.Ignored(dir) {
		return nil, fmt.Errorf("%w: neplatná složka %q", ErrInvalid, in.Dir)
	}
	if fi, err := os.Stat(filepath.Join(l.root, dir)); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("%w: složka %q ve VIDEO_ROOT neexistuje", ErrInvalid, dir)
	}
	if in.Kind != model.KindMovie && in.Kind != model.KindShow {
		return nil, fmt.Errorf("%w: typ musí být movie nebo show", ErrInvalid)
	}
	out := storage.LibraryInput{Dir: dir, Kind: in.Kind, Enabled: true}
	if err := l.validate(in.Kind, &in, &out); err != nil {
		return nil, err
	}
	lib, err := l.store.CreateLibrary(ctx, out)
	if errors.Is(err, storage.ErrConflict) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	l.changed()
	return lib, nil
}

// Update změní název, jazyk, zdroj a zapnutí knihovny.
func (l *Libraries) Update(ctx context.Context, id uuid.UUID, in LibraryRequest) (*model.Library, error) {
	cur, err := l.store.GetLibrary(ctx, id)
	if err != nil {
		return nil, err
	}
	out := storage.LibraryInput{Dir: cur.Dir, Name: cur.Name, Language: cur.Language, Enabled: cur.Enabled}
	if cur.Provider != nil {
		out.Provider = *cur.Provider
	}
	if cur.Icon != nil {
		out.Icon = *cur.Icon
	}
	if in.Name == "" {
		in.Name = cur.Name
	}
	if in.Language == "" {
		in.Language = cur.Language
	}
	if in.Provider == nil {
		in.Provider = cur.Provider
	}
	if err := l.validate(cur.Kind, &in, &out); err != nil {
		return nil, err
	}
	lib, err := l.store.UpdateLibrary(ctx, id, out)
	if err != nil {
		return nil, err
	}
	l.changed()
	return lib, nil
}

// Delete smaže knihovnu se všemi tituly, soubory i rozkoukaností. Soubory
// na disku zůstávají.
func (l *Libraries) Delete(ctx context.Context, id uuid.UUID) error {
	if err := l.store.DeleteLibrary(ctx, id); err != nil {
		return err
	}
	l.changed()
	return nil
}

// Reorder nastaví pořadí knihoven (menu, domovská stránka).
func (l *Libraries) Reorder(ctx context.Context, ids []uuid.UUID) error {
	err := l.store.ReorderLibraries(ctx, ids)
	if errors.Is(err, storage.ErrInvalid) {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return err
}
