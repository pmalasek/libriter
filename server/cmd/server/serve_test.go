package main

import (
	"testing"

	"mediater/internal/libriter"
	"mediater/internal/libriter/metadata"
)

// Obálka z metadat se stahuje jen z adres, které některý zdroj prohlásí za
// své. Zdroj bez allowlistu by obálky tiše nestahoval – proto ho musí mít každý.
func TestAllProvidersSupportCovers(t *testing.T) {
	for name, factory := range libriter.ProviderFactories() {
		if _, ok := factory(metadata.ProviderConfig{}).(metadata.CoverProvider); !ok {
			t.Errorf("zdroj %s neimplementuje metadata.CoverProvider", name)
		}
	}
}
