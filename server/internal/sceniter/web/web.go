// Package web vestavuje sestavené webové rozhraní Sceniteru do binárky.
package web

import (
	"embed"
	"io/fs"
)

// Prefix all: zahrne i .gitkeep, takže `go build` funguje i bez sestaveného
// frontendu (server pak vrací stránku s návodem).
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
