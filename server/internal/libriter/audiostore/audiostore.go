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

	"mediater/internal/core/fsroot"
)

// Resolve ověří, že relPath ukazuje dovnitř root, a vrátí jeho absolutní
// cestu. Hodnota z databáze může pocházet ze scanu i z ruční editace, proto
// se kontroluje pokaždé.
//
// Kontroluje se i cíl symbolických odkazů: odkaz uvnitř knihovny projde,
// odkaz ven ne. Neexistující soubor (smazaný, teprve vznikající) projde –
// volající to pozná sám; obsah se čte přes fsroot.Open, který ověří i typ
// souboru.
func Resolve(root, relPath string) (string, bool) {
	// Windows oddělovač by na Linuxu prošel jako součást názvu a schoval by
	// tak `..\` v cestě; v knihovně nemá co dělat.
	if strings.ContainsRune(relPath, '\\') {
		return "", false
	}
	abs, err := fsroot.Join(root, relPath)
	if err != nil || !fsroot.Contains(root, abs) {
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
