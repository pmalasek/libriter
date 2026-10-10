// Plugin metadat Československá bibliografická databáze (cbdb.cz) – knihy a autoři.
package main

import (
	"mediater/internal/core/plugin"
	"mediater/internal/core/version"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/metadata/bookplugin"
	"mediater/internal/libriter/metadata/cbdb"
)

func main() {
	bookplugin.Run(plugin.Manifest{
		Name:      "cbdb",
		Title:     "Československá bibliografická databáze (cbdb.cz)",
		Version:   version.String(),
		Languages: []string{"cs", "sk"},
		URLPatterns: map[string][]string{
			plugin.KindBook:   []string{`(?i)^https?://(www\.)?cbdb\.cz/`},
			plugin.KindAuthor: []string{`(?i)^https?://(www\.)?cbdb\.cz/(.*/)?autor-\d+-`},
		},
		ImageHosts:    []string{"cbdb.cz"},
		MinIntervalMS: 3000,
	}, func(_ map[string]string) metadata.Provider { return cbdb.NewClient() })
}
