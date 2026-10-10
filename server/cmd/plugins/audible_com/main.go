// Plugin metadat Audible.com – knihy.
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
		Name:      "audible_com",
		Title:     "Audible.com",
		Version:   version.String(),
		Languages: []string{"en"},
		URLPatterns: map[string][]string{
			plugin.KindBook: []string{`(?i)^https?://(www\.)?audible\.com/.*/[A-Z0-9]{10}/?$`},
		},
		ImageHosts: []string{"m.media-amazon.com"},
	}, func(_ map[string]string) metadata.Provider { return audible.NewClient(audible.MarketplaceCOM) })
}
