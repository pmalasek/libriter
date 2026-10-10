// Plugin metadat Goodreads – knihy.
package main

import (
	"mediater/internal/core/plugin"
	"mediater/internal/core/version"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/metadata/bookplugin"
	"mediater/internal/libriter/metadata/goodreads"
)

func main() {
	bookplugin.Run(plugin.Manifest{
		Name:      "goodreads",
		Title:     "Goodreads",
		Version:   version.String(),
		Languages: []string{"en"},
		URLPatterns: map[string][]string{
			plugin.KindBook: []string{`(?i)^https?://(www\.)?goodreads\.com/book/show/\d+`},
		},
		ImageHosts:    []string{"m.media-amazon.com", "i.gr-assets.com", "images.gr-assets.com", "s.gr-assets.com"},
		MinIntervalMS: 3000,
	}, func(_ map[string]string) metadata.Provider { return goodreads.NewClient() })
}
