// internal/scanner/cover.go
//
// Hledání a ukládání obálek knih do COVER_ROOT.
//
// Pořadí hledání (při vytvoření nebo prvním ingestu knihy):
//  1. obrázek v adresáři s audio soubory → vybere se největší (podle velikosti)
//  2. obrázek vložený v tagu audio souboru (APIC / covr)
//
// Nalezený obrázek se zkopíruje do COVER_ROOT jako <book_id>.<ext>
// a relativní cesta se uloží do books.cover_path.

package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"libriter/internal/model"

	"github.com/dhowden/tag"
	"github.com/google/uuid"
)

const maxCoverBytes = 20 << 20 // 20 MB – větší soubor není obálka

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".webp": true, ".gif": true, ".bmp": true,
}

// ensureCover doplní knize obálku, pokud ji ještě nemá.
// Chyby pouze loguje – ingest kapitoly kvůli obálce neselže.
func (s *Scanner) ensureCover(ctx context.Context, book *model.Book, absDir, absFile string) {
	if s.coverRoot == "" || s.hasCover(book) {
		return
	}

	relCover, source, err := s.resolveCover(book.ID, absDir, absFile)
	if err != nil {
		s.log.Warn("uložení obálky selhalo", "book", book.Title, "err", err)
		return
	}
	if relCover == "" {
		s.log.Debug("obálka nenalezena", "book", book.Title, "dir", s.relPath(absDir))
		return
	}

	if err := s.store.UpdateBookCoverPath(ctx, book.ID, relCover); err != nil {
		s.log.Warn("zápis cesty k obálce selhal", "book", book.Title, "err", err)
		return
	}
	book.CoverPath = &relCover

	s.log.Info("obálka uložena", "book", book.Title, "cover", relCover, "zdroj", source)
}

// hasCover vrátí true, pokud kniha má obálku, která na disku stále existuje.
func (s *Scanner) hasCover(book *model.Book) bool {
	if book.CoverPath == nil || *book.CoverPath == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(s.coverRoot, *book.CoverPath))
	return err == nil
}

// resolveCover najde obálku a zkopíruje ji do COVER_ROOT.
// Vrací relativní cestu od COVER_ROOT a popis zdroje ("" = nic nenalezeno).
func (s *Scanner) resolveCover(bookID uuid.UUID, absDir, absFile string) (string, string, error) {
	// 1. Obrázek v adresáři – největší
	if src := largestImageInDir(absDir); src != "" {
		rel, err := s.copyCoverFile(bookID, src)
		return rel, "adresář: " + filepath.Base(src), err
	}

	// 2. Obrázek vložený v audio souboru
	if pic := embeddedCover(absFile); pic != nil {
		rel, err := s.writeCover(bookID, pic.Data, "."+strings.ToLower(pic.Ext))
		return rel, "tag: " + filepath.Base(absFile), err
	}

	return "", "", nil
}

// largestImageInDir vrátí cestu k největšímu obrázku v adresáři (nerekurzivně).
// Prázdný řetězec = žádný obrázek.
func largestImageInDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var best string
	var bestSize int64

	for _, e := range entries {
		if e.IsDir() || !imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 || info.Size() > maxCoverBytes {
			continue
		}
		if info.Size() > bestSize {
			best, bestSize = filepath.Join(dir, e.Name()), info.Size()
		}
	}
	return best
}

// embeddedCover přečte obálku z tagů audio souboru. nil = žádná není.
func embeddedCover(absPath string) *tag.Picture {
	f, err := os.Open(absPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil
	}

	pic := m.Picture()
	if pic == nil || len(pic.Data) == 0 || len(pic.Data) > maxCoverBytes {
		return nil
	}
	return pic
}

// copyCoverFile zkopíruje obrázek z disku do COVER_ROOT.
func (s *Scanner) copyCoverFile(bookID uuid.UUID, srcPath string) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("čtení obálky %s: %w", srcPath, err)
	}
	return s.writeCover(bookID, data, filepath.Ext(srcPath))
}

// writeCover uloží data obálky do COVER_ROOT jako <book_id><ext>
// a vrátí cestu relativní ke COVER_ROOT.
func (s *Scanner) writeCover(bookID uuid.UUID, data []byte, ext string) (string, error) {
	ext = normalizeImageExt(ext)
	name := bookID.String() + ext

	if err := os.MkdirAll(s.coverRoot, 0o755); err != nil {
		return "", fmt.Errorf("vytvoření %s: %w", s.coverRoot, err)
	}

	dst := filepath.Join(s.coverRoot, name)
	tmp := dst + ".tmp"

	if err := os.WriteFile(tmp, data, fs.FileMode(0o644)); err != nil {
		return "", fmt.Errorf("zápis obálky: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("přesun obálky: %w", err)
	}

	return name, nil
}

// normalizeImageExt vrátí známou příponu obrázku, jinak ".jpg".
func normalizeImageExt(ext string) string {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if imageExts[ext] {
		return ext
	}
	return ".jpg"
}
