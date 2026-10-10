// Plugin metadat Open Library – knihy a autoři.
package main

import (
	"mediater/internal/core/plugin"
	"mediater/internal/core/version"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/metadata/bookplugin"
	"mediater/internal/libriter/metadata/openlibrary"
)

func main() {
	bookplugin.Run(plugin.Manifest{
		Name:      "openlibrary",
		Title:     "Open Library",
		Version:   version.String(),
		Languages: []string{"*"},
		URLPatterns: map[string][]string{
			plugin.KindBook:   []string{`(?i)^https?://(www\.)?openlibrary\.org/`},
			plugin.KindAuthor: []string{`(?i)^https?://(www\.)?openlibrary\.org/authors/[^/]+`},
		},
		ImageHosts: []string{"covers.openlibrary.org"},
	}, func(_ map[string]string) metadata.Provider { return openlibrary.NewClient() })
}
