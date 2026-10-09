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

	first := registry.Rebuild(Profiles{Default: []string{"jenknihy"}}, ProviderConfig{})
	if got := registry.Chain().Providers(); !reflect.DeepEqual(got, []string{"jenknihy"}) {
		t.Errorf("po prvním sestavení = %v, chtěno [jenknihy]", got)
	}

	registry.Rebuild(Profiles{Default: []string{"sautory", "jenknihy"}}, ProviderConfig{})
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
	registry.Rebuild(Profiles{Default: []string{"preklep", "jenknihy"}}, ProviderConfig{})

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

func TestRegistryChainForLanguage(t *testing.T) {
	registry := NewRegistry(testFactories())
	registry.Rebuild(Profiles{
		Default:    []string{"sautory"},
		ByLanguage: map[string][]string{"en": {"jenknihy", "sautory"}, "de": {}},
	}, ProviderConfig{})

	tests := []struct {
		lang string
		want []string
	}{
		{"en", []string{"jenknihy", "sautory"}},
		{"cs", []string{"sautory"}}, // jazyk bez profilu jede na výchozím
		{"", []string{"sautory"}},
		{"de", []string{}}, // profil se vším vypnutým nic nehledá – nepadá na výchozí
	}
	for _, tt := range tests {
		if got := registry.ChainFor(tt.lang).Providers(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ChainFor(%q) = %v, chtěno %v", tt.lang, got, tt.want)
		}
	}

	// Chain() je sjednocení všech profilů – výchozí pořadí napřed.
	if got := registry.Chain().Providers(); !reflect.DeepEqual(got, []string{"sautory", "jenknihy"}) {
		t.Errorf("Chain() = %v, chtěno [sautory jenknihy]", got)
	}
}

// Profily sdílejí jednu instanci zdroje – odstup mezi požadavky na web musí
// platit napříč jazyky.
func TestRegistrySharesInstancesAcrossProfiles(t *testing.T) {
	created := 0
	registry := NewRegistry(map[string]Factory{
		"zdroj": func(ProviderConfig) Provider {
			created++
			return &fakeProvider{name: "zdroj"}
		},
	})
	created = 0 // NewRegistry si zdroj vytvoří kvůli zjištění schopností

	registry.Rebuild(Profiles{
		Default:    []string{"zdroj"},
		ByLanguage: map[string][]string{"en": {"zdroj"}, "de": {"zdroj"}},
	}, ProviderConfig{})

	if created != 1 {
		t.Errorf("zdroj vznikl %d×, chtěno 1×", created)
	}
	en, _ := registry.ChainFor("en").Provider("zdroj")
	def, _ := registry.ChainFor("").Provider("zdroj")
	if en != def {
		t.Error("profily nesdílejí instanci zdroje")
	}
}

func TestRegistryAuthorChainFor(t *testing.T) {
	factories := map[string]Factory{
		"cesky": func(ProviderConfig) Provider {
			return &fakeAuthorProvider{fakeProvider: fakeProvider{name: "cesky"}}
		},
		"anglicky": func(ProviderConfig) Provider {
			return &fakeAuthorProvider{fakeProvider: fakeProvider{name: "anglicky"}}
		},
		"nemecky": func(ProviderConfig) Provider {
			return &fakeAuthorProvider{fakeProvider: fakeProvider{name: "nemecky"}}
		},
		"zaloha": func(ProviderConfig) Provider {
			return &fakeAuthorProvider{fakeProvider: fakeProvider{name: "zaloha"}}
		},
		"jenknihy": func(ProviderConfig) Provider { return &fakeProvider{name: "jenknihy"} },
	}
	registry := NewRegistry(factories)
	registry.Rebuild(Profiles{
		Default: []string{"zaloha"},
		ByLanguage: map[string][]string{
			"cs": {"cesky"},
			"en": {"jenknihy", "anglicky"},
			"de": {"nemecky"},
			"fr": {"jenknihy"}, // profil bez zdroje autorů se přeskočí
		},
	}, ProviderConfig{})

	tests := []struct {
		ui   string
		want []string
	}{
		{"cs", []string{"cesky"}},    // vlastní profil
		{"de", []string{"nemecky"}},  // vlastní profil
		{"en", []string{"anglicky"}}, // vlastní profil, jen zdroje autorů
		{"es", []string{"anglicky"}}, // bez profilu → angličtina, ne výchozí pořadí
		{"fr", []string{"anglicky"}}, // profil bez autorů → angličtina
		{"", []string{"anglicky"}},
	}
	for _, tt := range tests {
		if got := registry.AuthorChainFor(tt.ui).AuthorProviders(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("AuthorChainFor(%q) = %v, chtěno %v", tt.ui, got, tt.want)
		}
	}

	// Bez anglického profilu zbude výchozí pořadí.
	registry.Rebuild(Profiles{Default: []string{"zaloha"}}, ProviderConfig{})
	if got := registry.AuthorChainFor("es").AuthorProviders(); !reflect.DeepEqual(got, []string{"zaloha"}) {
		t.Errorf("bez profilu en = %v, chtěno [zaloha]", got)
	}
}
