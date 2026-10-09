package importer

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
)

// maxZipEntries omezuje počet souborů v jednom archivu (ochrana proti zip bombě
// z milionu prázdných souborů).
const maxZipEntries = 20000

// zipEntry je soubor archivu, který se rozbalí.
type zipEntry struct {
	file *zip.File
	rel  string // vyčištěná cesta uvnitř archivu
}

// extractZip rozbalí archiv zipRel (relativně k filesDir) vedle něj a archiv
// smaže. Když má archiv jediný kořenový adresář („Kniha/…“), rozbalí se
// tak, jak je; volně ležící soubory dostanou adresář pojmenovaný po archivu,
// aby z nich šlo poznat název knihy. Vrací počet zapsaných bajtů.
func extractZip(filesDir, zipRel string, limit int64) (int64, error) {
	zipAbs := filepath.Join(filesDir, filepath.FromSlash(zipRel))
	zr, err := zip.OpenReader(zipAbs)
	if err != nil {
		return 0, fmt.Errorf("%s: archiv nelze otevřít: %w", path.Base(zipRel), err)
	}
	defer zr.Close()

	if len(zr.File) > maxZipEntries {
		return 0, fmt.Errorf("%s: archiv obsahuje příliš mnoho souborů", path.Base(zipRel))
	}

	var entries []zipEntry
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Go označí za „non-UTF8“ i platné UTF-8 bez příznaku v hlavičce –
		// tak balí macOS. Překódovává se jen to, co UTF-8 opravdu není.
		name := f.Name
		if f.NonUTF8 && !utf8.ValidString(name) {
			name = decodeCP852(name)
		}
		rel, err := CleanRelPath(name)
		if err != nil || isJunk(rel) {
			continue
		}
		// Archiv v archivu se nerozbaluje.
		if !isAccepted(rel) || strings.EqualFold(path.Ext(rel), ".zip") {
			continue
		}
		entries = append(entries, zipEntry{file: f, rel: rel})
	}

	base := path.Dir(zipRel)
	if !singleRoot(entries) {
		base = path.Join(base, strings.TrimSuffix(path.Base(zipRel), path.Ext(zipRel)))
	}

	var written int64
	for _, e := range entries {
		dst := filepath.Join(filesDir, filepath.FromSlash(path.Join(base, e.rel)))
		n, err := writeEntry(e.file, dst, limit-written)
		written += n
		if err != nil {
			return written, err
		}
	}

	zr.Close()
	_ = os.Remove(zipAbs)
	return written, nil
}

// singleRoot vrátí true, když všechny soubory leží v jednom kořenovém adresáři.
func singleRoot(entries []zipEntry) bool {
	root := ""
	for _, e := range entries {
		first, _, nested := strings.Cut(e.rel, "/")
		if !nested {
			return false
		}
		if root == "" {
			root = first
		} else if root != first {
			return false
		}
	}
	return root != ""
}

func writeEntry(f *zip.File, dst string, limit int64) (int64, error) {
	if limit <= 0 {
		return 0, ErrTooLarge
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	// Deklarované velikosti v hlavičce se nevěří – počítá se, co opravdu teče.
	n, err := io.Copy(out, io.LimitReader(rc, limit+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > limit {
		err = ErrTooLarge
	}
	if err != nil {
		_ = os.Remove(dst)
		return n, err
	}
	return n, nil
}

// decodeCP852 převede název ze staršího zipu z české Windows (bez UTF-8
// příznaku) na UTF-8.
func decodeCP852(s string) string {
	out, err := charmap.CodePage852.NewDecoder().String(s)
	if err != nil {
		return s
	}
	return out
}
