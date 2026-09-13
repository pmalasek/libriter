package metadata

import (
	"fmt"
	"log/slog"
)

// Factory vytvoří poskytovatele. Registry drží tovární funkce, ne hotové
// klienty – zapojí se jen ty zdroje, které jsou v konfiguraci.
type Factory func() Provider

// BuildChain sestaví řetězec podle jmen v names, ve stejném pořadí.
// Neznámé jméno se přeskočí s varováním, aby překlep v .env neshodil server.
func BuildChain(names []string, factories map[string]Factory) *Chain {
	providers := make([]Provider, 0, len(names))
	used := make(map[string]bool, len(names))

	for _, name := range names {
		factory, ok := factories[name]
		if !ok {
			slog.Warn("neznámý zdroj metadat v METADATA_PROVIDERS", "zdroj", name,
				"známé", knownNames(factories))
			continue
		}
		if used[name] {
			continue // stejný zdroj uvedený dvakrát nemá smysl volat dvakrát
		}
		used[name] = true
		providers = append(providers, factory())
	}

	return NewChain(providers...)
}

func knownNames(factories map[string]Factory) string {
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	return fmt.Sprint(names)
}
