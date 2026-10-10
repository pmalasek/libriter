// Plugin metadat Databáze knih (databazeknih.cz) – knihy a autoři.
package main

import (
	"mediater/internal/core/plugin"
	"mediater/internal/core/version"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/metadata/bookplugin"
	"mediater/internal/libriter/metadata/databazeknih"
)

func main() {
	bookplugin.Run(plugin.Manifest{
		Name:      "databazeknih",
		Title:     "Databáze knih (databazeknih.cz)",
		Version:   version.String(),
		Languages: []string{"cs", "sk"},
		URLPatterns: map[string][]string{
			plugin.KindBook:   []string{`(?i)^https?://(www\.)?databazeknih\.cz/`},
			plugin.KindAuthor: []string{`(?i)^https?://(www\.)?databazeknih\.cz/(.*/)?autori/`},
		},
		ImageHosts:    []string{"databazeknih.cz"},
		MinIntervalMS: 3000,
	}, func(_ map[string]string) metadata.Provider { return databazeknih.NewClient() })
}
