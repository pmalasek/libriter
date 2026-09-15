package metadata

import (
	"reflect"
	"testing"
)

// testFactories staví dva zdroje: jeden umí jen knihy, druhý i autory –
// podle toho se pozná, že Registry schopnosti čte správně.
func testFactories() map[string]Factory {
	return map[string]Factory{
		"jenknihy": func(ProviderConfig) Provider { return &fakeProvider{name: "jenknihy"} },
		"sautory": func(ProviderConfig) Provider {
			return &fakeAuthorProvider{fakeProvider: fakeProvider{name: "sautory"}}
		},
	}
}

func TestRegistryRebuildSwapsChain(t *testing.T) {
	registry := NewRegistry(testFactories())

	if got := registry.Chain().Providers(); len(got) != 0 {
		t.Errorf("čerstvé registry = %v, chtěn prázdný řetězec", got)
	}

	first := registry.Rebuild([]string{"jenknihy"}, ProviderConfig{})
	if got := registry.Chain().Providers(); !reflect.DeepEqual(got, []string{"jenknihy"}) {
		t.Errorf("po prvním sestavení = %v, chtěno [jenknihy]", got)
	}

	registry.Rebuild([]string{"sautory", "jenknihy"}, ProviderConfig{})
	if got := registry.Chain().Providers(); !reflect.DeepEqual(got, []string{"sautory", "jenknihy"}) {
		t.Errorf("po výměně = %v, chtěno [sautory jenknihy]", got)
	}

	// Starý řetězec drží požadavek, který začal před výměnou – musí zůstat
	// použitelný a nezměněný.
	if got := first.Providers(); !reflect.DeepEqual(got, []string{"jenknihy"}) {
		t.Errorf("starý řetězec = %v, chtěno [jenknihy]", got)
	}
}

func TestRegistryKnownCapabilities(t *testing.T) {
	registry := NewRegistry(testFactories())

	want := []ProviderInfo{
		{Name: "jenknihy", SupportsAuthors: false, SupportsImages: false},
		{Name: "sautory", SupportsAuthors: true, SupportsImages: true},
	}
	if got := registry.Known(); !reflect.DeepEqual(got, want) {
		t.Errorf("Known() = %+v, chtěno %+v", got, want)
	}
	if got := registry.KnownNames(); !reflect.DeepEqual(got, []string{"jenknihy", "sautory"}) {
		t.Errorf("KnownNames() = %v", got)
	}
	if !registry.IsKnown("sautory") || registry.IsKnown("neexistuje") {
		t.Error("IsKnown vrací špatné odpovědi")
	}
}

func TestRegistrySkipsUnknownProvider(t *testing.T) {
	registry := NewRegistry(testFactories())
	registry.Rebuild([]string{"preklep", "jenknihy"}, ProviderConfig{})

	if got := registry.Chain().Providers(); !reflect.DeepEqual(got, []string{"jenknihy"}) {
		t.Errorf("řetězec = %v, chtěno [jenknihy] (neznámý zdroj se přeskočí)", got)
	}
}

func TestChainProviderLookup(t *testing.T) {
	chain := NewChain(&fakeProvider{name: "prvni"}, &fakeProvider{name: "druhy"})

	p, ok := chain.Provider("druhy")
	if !ok || p.Name() != "druhy" {
		t.Errorf("Provider(druhy) = %v, %v", p, ok)
	}
	if _, ok := chain.Provider("neexistuje"); ok {
		t.Error("Provider našel zdroj, který v řetězci není")
	}
}
