// Package web vestavuje sestavené webové rozhraní Libriteru do binárky.
package web

import (
	"embed"
	"io/fs"
)

// Prefix all: zahrne i soubory začínající tečkou (.gitkeep), takže embed
// nikdy není prázdný a `go build` funguje i bez sestaveného frontendu.
//
//go:embed all:dist
var distFS embed.FS

// FS vrátí obsah adresáře dist (kořen SPA).
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return sub
}
