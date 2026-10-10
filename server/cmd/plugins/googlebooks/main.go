// Plugin metadat Google Books – knihy.
package main

import (
	"mediater/internal/core/plugin"
	"mediater/internal/core/version"
	"mediater/internal/libriter/metadata"
	"mediater/internal/libriter/metadata/bookplugin"
	"mediater/internal/libriter/metadata/googlebooks"
)

func main() {
	bookplugin.Run(plugin.Manifest{
		Name:      "googlebooks",
		Title:     "Google Books",
		Version:   version.String(),
		Languages: []string{"*"},
		URLPatterns: map[string][]string{
			plugin.KindBook: []string{`(?i)^https?://(www\.)?googleapis\.com/.*books/v1/volumes/`},
		},
		ImageHosts: []string{"books.google.com", "books.googleusercontent.com"},
		Config:     []plugin.ConfigField{{Key: "api_key", Label: "API key", Secret: true, Env: "GOOGLE_BOOKS_API_KEY"}},
	}, func(cfg map[string]string) metadata.Provider { return googlebooks.NewClient(cfg["api_key"]) })
}
