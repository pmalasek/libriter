package main

import (
	"testing"

	"libriter/internal/metadata"
)

// Obálka z metadat se stahuje jen z adres, které některý zdroj prohlásí za
// své. Zdroj bez allowlistu by obálky tiše nestahoval – proto ho musí mít každý.
func TestAllProvidersSupportCovers(t *testing.T) {
	for name, factory := range providerFactories() {
		if _, ok := factory(metadata.ProviderConfig{}).(metadata.CoverProvider); !ok {
			t.Errorf("zdroj %s neimplementuje metadata.CoverProvider", name)
		}
	}
}
