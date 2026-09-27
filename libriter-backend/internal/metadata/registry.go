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

// Profiles říká, které zdroje a v jakém pořadí se zkoušejí pro který jazyk
// knihy. Jazyk bez vlastního profilu používá Default. Klíče ByLanguage jsou
// kódy ISO 639-1 malými písmeny.
type Profiles struct {
	Default    []string
	ByLanguage map[string][]string
}

// chainSet je jedna neměnná sada řetězců sestavená z Profiles.
type chainSet struct {
	def    *Chain
	byLang map[string]*Chain
	// all obsahuje každý zdroj zapnutý aspoň v jednom profilu – výchozí
	// v jejich pořadí, pak zbytek. Slouží jako allowlist adres (detail knihy,
	// autor, fotka), protože výsledek hledání mohl přijít z kteréhokoli profilu.
	all *Chain
}

// Registry drží aktuální řetězce zdrojů a umí je za běhu vyměnit, když admin
// změní zapnuté zdroje nebo jejich pořadí.
//
// Řetězce jsou neměnné a mění se jen ukazatel na ně, takže rozpracovaný
// požadavek dojede na tom řetězci, se kterým začal – žádné zamykání.
type Registry struct {
	factories map[string]Factory
	info      []ProviderInfo
	chains    atomic.Pointer[chainSet]
}

// NewRegistry zjistí schopnosti všech známých zdrojů a začne s prázdnými
// řetězci; ty se naplní prvním voláním Rebuild.
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

	r.chains.Store(&chainSet{def: NewChain(), all: NewChain()})
	return r
}

// Chain vrátí řetězec všech zapnutých zdrojů. Volá se při každém požadavku,
// aby změna nastavení platila okamžitě.
func (r *Registry) Chain() *Chain {
	return r.chains.Load().all
}

// ChainFor vrátí řetězec pro jazyk knihy; jazyk bez vlastního profilu
// (i prázdný) dostane výchozí řetězec.
func (r *Registry) ChainFor(language string) *Chain {
	c := r.chains.Load()
	if chain, ok := c.byLang[language]; ok {
		return chain
	}
	return c.def
}

// AuthorChainFor vybere řetězec pro hledání autorů podle jazyka rozhraní
// toho, kdo hledá – životopis má být v jazyce, kterému rozumí:
//  1. profil jazyka rozhraní,
//  2. anglický profil, když pro jazyk rozhraní metadata nemáme,
//  3. výchozí profil jako poslední záchrana.
//
// Profil bez jediného zdroje, který umí autory, se přeskakuje.
func (r *Registry) AuthorChainFor(uiLanguage string) *Chain {
	c := r.chains.Load()
	for _, chain := range []*Chain{c.byLang[uiLanguage], c.byLang["en"], c.def} {
		if chain != nil && len(chain.AuthorProviders()) > 0 {
			return chain
		}
	}
	return c.def
}

// Rebuild sestaví nové řetězce ze zapnutých zdrojů a nasadí je. Každý zdroj
// vznikne jen jednou a profily ho sdílejí – klient si drží odstup mezi
// požadavky na svůj web a ten musí platit napříč jazyky.
func (r *Registry) Rebuild(p Profiles, cfg ProviderConfig) *Chain {
	instances := make(map[string]Provider)
	build := func(names []string) *Chain {
		providers := make([]Provider, 0, len(names))
		used := make(map[string]bool, len(names))
		for _, name := range names {
			if used[name] {
				continue // stejný zdroj uvedený dvakrát nemá smysl volat dvakrát
			}
			factory, ok := r.factories[name]
			if !ok {
				slog.Warn("neznámý zdroj metadat", "zdroj", name, "známé", knownNames(r.factories))
				continue
			}
			used[name] = true
			if instances[name] == nil {
				instances[name] = factory(cfg)
			}
			providers = append(providers, instances[name])
		}
		return NewChain(providers...)
	}

	next := &chainSet{def: build(p.Default), byLang: make(map[string]*Chain, len(p.ByLanguage))}
	allNames := append([]string(nil), p.Default...)
	languages := make([]string, 0, len(p.ByLanguage))
	for lang := range p.ByLanguage {
		languages = append(languages, lang)
	}
	sort.Strings(languages)
	for _, lang := range languages {
		next.byLang[lang] = build(p.ByLanguage[lang])
		allNames = append(allNames, p.ByLanguage[lang]...)
		slog.Info("zdroje metadat pro jazyk", "jazyk", lang, "pořadí", next.byLang[lang].Providers())
	}
	next.all = build(allNames)

	r.chains.Store(next)
	slog.Info("zdroje metadat", "pořadí", next.def.Providers())
	return next.def
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
