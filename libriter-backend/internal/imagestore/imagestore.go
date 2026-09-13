// Package imagestore ukládá obrázky (obálky knih, fotky autorů) do adresáře
// na disku a bezpečně z něj skládá cesty zpátky.
//
// V databázi se drží jen holý název souboru, nikdy celá cesta – tím je
// adresář přenositelný a Resolve má co ověřovat.
package imagestore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxBytes je strop na velikost obrázku; větší soubor obálka ani fotka není.
const MaxBytes = 20 << 20 // 20 MB

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".webp": true, ".gif": true, ".bmp": true,
}

// IsImageExt řekne, jestli přípona patří podporovanému obrázku.
func IsImageExt(ext string) bool {
	return imageExts[strings.ToLower(ext)]
}

// NormalizeExt vrátí známou příponu obrázku, jinak ".jpg".
func NormalizeExt(ext string) string {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if imageExts[ext] {
		return ext
	}
	return ".jpg"
}

// ExtForContentType převede MIME typ na příponu souboru.
func ExtForContentType(contentType string) string {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}

	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	default:
		return ""
	}
}

// IsImageContentType říká, jestli MIME typ patří podporovanému obrázku.
func IsImageContentType(contentType string) bool {
	return ExtForContentType(contentType) != ""
}

// Write uloží data pod názvem <name><ext> do root a vrátí název souboru.
// Zapisuje přes dočasný soubor a přejmenování, aby na disku nikdy neležel
// obrázek stažený jen zpola.
func Write(root, name string, data []byte, ext string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("adresář pro obrázky není nastaven")
	}

	fileName := name + NormalizeExt(ext)

	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("vytvoření %s: %w", root, err)
	}

	dst := filepath.Join(root, fileName)
	tmp := dst + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("zápis obrázku: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("přesun obrázku: %w", err)
	}

	return fileName, nil
}

// CopyFile zkopíruje obrázek ze srcPath do root pod názvem name.
func CopyFile(root, name, srcPath string) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("čtení obrázku %s: %w", srcPath, err)
	}
	return Write(root, name, data, filepath.Ext(srcPath))
}

// ReadLimited načte nejvýš MaxBytes dat; větší vstup skončí chybou, aby
// cizí server nemohl zaplnit disk.
func ReadLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("obrázek je větší než %d MB", MaxBytes>>20)
	}
	return data, nil
}

// Remove smaže soubor obrázku. Neexistující soubor není chyba.
func Remove(root, fileName string) error {
	path, ok := Resolve(root, fileName)
	if !ok {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("mazání obrázku: %w", err)
	}
	return nil
}

// Resolve ověří, že fileName je holý název souboru, a vrátí absolutní cestu
// uvnitř root. Chrání před path traversal – hodnota v databázi může pocházet
// z editace přes API.
func Resolve(root, fileName string) (string, bool) {
	if root == "" || fileName == "" || fileName == "." || fileName == ".." {
		return "", false
	}
	if strings.ContainsAny(fileName, `/\`) || filepath.Base(fileName) != fileName {
		return "", false
	}

	// root může být relativní cesta (viz config.env.path).
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}

	abs := filepath.Join(absRoot, fileName)
	if !strings.HasPrefix(abs, absRoot+string(filepath.Separator)) {
		return "", false
	}
	return abs, true
}

// Exists řekne, jestli soubor obrázku existuje a je to běžný soubor.
func Exists(root, fileName string) bool {
	path, ok := Resolve(root, fileName)
	if !ok {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
