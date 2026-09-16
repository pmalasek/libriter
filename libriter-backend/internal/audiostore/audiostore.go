// Package audiostore skládá bezpečné cesty k audio souborům knihovny.
//
// Na rozdíl od obrázků drží databáze u kapitoly celou relativní cestu včetně
// podadresářů (autor/kniha/01.mp3), takže Resolve musí povolit oddělovače a
// hlídat jen to, aby výsledek nevylezl z kořene knihovny.
package audiostore

import (
	"mime"
	"path/filepath"
	"strings"
)

// Resolve ověří, že relPath ukazuje na soubor uvnitř root, a vrátí jeho
// absolutní cestu. Hodnota z databáze může pocházet ze scanu i z ruční
// editace, proto se kontroluje pokaždé.
func Resolve(root, relPath string) (string, bool) {
	if root == "" || relPath == "" {
		return "", false
	}
	// Windows oddělovač by na Linuxu prošel jako součást názvu a schoval by
	// tak `..\` v cestě; v knihovně nemá co dělat.
	if strings.ContainsRune(relPath, '\\') || filepath.IsAbs(relPath) {
		return "", false
	}
	for _, part := range strings.Split(relPath, "/") {
		if part == ".." {
			return "", false
		}
	}

	// root může být relativní cesta (viz config.env.path).
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}

	abs := filepath.Join(absRoot, relPath)
	if !strings.HasPrefix(abs, absRoot+string(filepath.Separator)) {
		return "", false
	}
	return abs, true
}

// Formáty, které scanner do knihovny pouští a prohlížeč umí přehrát.
// mime.TypeByExtension je zná jen částečně a podle systémové tabulky, proto
// vlastní seznam.
var audioTypes = map[string]string{
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".m4b":  "audio/mp4",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".opus": "audio/ogg",
	".flac": "audio/flac",
	".wav":  "audio/wav",
}

// ContentType vrátí MIME typ audio souboru podle přípony.
func ContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if t, ok := audioTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}
