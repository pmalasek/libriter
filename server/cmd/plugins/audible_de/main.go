// Plugin metadat Audible.de – knihy.
package main

import (
	"mediater/internal/core/plugin"
	"mediater/internal/core/version"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/metadata/audible"
	"mediater/internal/libriter/metadata/bookplugin"
)

func main() {
	bookplugin.Run(plugin.Manifest{
		Name:      "audible_de",
		Title:     "Audible.de",
		Version:   version.String(),
		Languages: []string{"de"},
		URLPatterns: map[string][]string{
			plugin.KindBook: []string{`(?i)^https?://(www\.)?audible\.de/.*/[A-Z0-9]{10}/?$`},
		},
		ImageHosts: []string{"m.media-amazon.com"},
	}, func(_ map[string]string) metadata.Provider { return audible.NewClient(audible.MarketplaceDE) })
}
