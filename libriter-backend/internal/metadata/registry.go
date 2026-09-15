package metadata

import (
	"fmt"
	"log/slog"
	"sort"
	"sync/atomic"
)

// ProviderConfig jsou hodnoty, které zdroje potřebují při vzniku. Předává se
// při každém sestavení řetězce, protože klíč Google Books mění admin za běhu.
type ProviderConfig struct {
	GoogleBooksAPIKey string
}

// Factory vytvoří poskytovatele. Registry drží tovární funkce, ne hotové
// klienty – zapojí se jen ty zdroje, které jsou zapnuté.
type Factory func(cfg ProviderConfig) Provider

// ProviderInfo popisuje, co který zdroj umí. Administrace podle toho ukazuje,
// že Google Books hledá jen knihy, ne autory.
type ProviderInfo struct {
	Name            string `json:"name"`
	SupportsAuthors bool   `json:"supports_authors"`
	SupportsImages  bool   `json:"supports_images"`
}

// BuildChain sestaví řetězec podle jmen v names, ve stejném pořadí.
// Neznámé jméno se přeskočí s varováním, aby překlep v .env neshodil server.
func BuildChain(names []string, factories map[string]Factory, cfg ProviderConfig) *Chain {
	providers := make([]Provider, 0, len(names))
	used := make(map[string]bool, len(names))

	for _, name := range names {
		factory, ok := factories[name]
		if !ok {
			slog.Warn("neznámý zdroj metadat", "zdroj", name, "známé", knownNames(factories))
			continue
		}
		if used[name] {
			continue // stejný zdroj uvedený dvakrát nemá smysl volat dvakrát
		}
		used[name] = true
		providers = append(providers, factory(cfg))
	}

	return NewChain(providers...)
}

func knownNames(factories map[string]Factory) string {
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return fmt.Sprint(names)
}

// Registry drží aktuální řetězec zdrojů a umí ho za běhu vyměnit, když admin
// změní zapnuté zdroje nebo jejich pořadí.
//
// Řetězec je neměnný a mění se jen ukazatel na něj, takže rozpracovaný
// požadavek dojede na tom řetězci, se kterým začal – žádné zamykání.
type Registry struct {
	factories map[string]Factory
	info      []ProviderInfo
	chain     atomic.Pointer[Chain]
}

// NewRegistry zjistí schopnosti všech známých zdrojů a začne s prázdným
// řetězcem; ten se naplní prvním voláním Rebuild.
func NewRegistry(factories map[string]Factory) *Registry {
	r := &Registry{factories: factories}

	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		_, isAuthorProvider := factories[name](ProviderConfig{}).(AuthorProvider)
		r.info = append(r.info, ProviderInfo{
			Name:            name,
			SupportsAuthors: isAuthorProvider,
			SupportsImages:  isAuthorProvider,
		})
	}

	r.chain.Store(NewChain())
	return r
}

// Chain vrátí aktuální řetězec. Volá se při každém požadavku, aby změna
// nastavení platila okamžitě.
func (r *Registry) Chain() *Chain {
	return r.chain.Load()
}

// Rebuild sestaví nový řetězec ze zapnutých zdrojů a nasadí ho.
func (r *Registry) Rebuild(names []string, cfg ProviderConfig) *Chain {
	chain := BuildChain(names, r.factories, cfg)
	r.chain.Store(chain)
	slog.Info("zdroje metadat", "pořadí", chain.Providers())
	return chain
}

// Known vrací všechny známé zdroje (i vypnuté) seřazené podle jména.
func (r *Registry) Known() []ProviderInfo {
	out := make([]ProviderInfo, len(r.info))
	copy(out, r.info)
	return out
}

// KnownNames vrací jména všech známých zdrojů.
func (r *Registry) KnownNames() []string {
	names := make([]string, 0, len(r.info))
	for _, i := range r.info {
		names = append(names, i.Name)
	}
	return names
}

// IsKnown říká, jestli zdroj tohoto jména vůbec existuje.
func (r *Registry) IsKnown(name string) bool {
	_, ok := r.factories[name]
	return ok
}
