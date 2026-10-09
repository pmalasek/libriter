// Package version zpřístupňuje verzi binárky. Hodnotu vkládá linker při
// buildu (`just build`); bez ní se sáhne po údajích z build info, aby i
// `go run` ukázal něco konkrétnějšího než „dev“.
package version

import (
	"runtime/debug"
)

// Version nastavuje linker: -X libriter/internal/version.Version=<verze>
var Version = "dev"

// String vrátí verzi k zobrazení v administraci a v logu při startu.
func String() string {
	if Version != "dev" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}

	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if revision == "" {
		return Version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified == "true" {
		revision += "-dirty"
	}
	return "dev-" + revision
}
