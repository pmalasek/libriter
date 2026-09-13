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
	"os"
	"path/filepath"
	"strings"

	"libriter/internal/imagestore"
	"libriter/internal/model"

	"github.com/dhowden/tag"
	"github.com/google/uuid"
)

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
	return imagestore.Exists(s.coverRoot, *book.CoverPath)
}

// resolveCover najde obálku a zkopíruje ji do COVER_ROOT.
// Vrací relativní cestu od COVER_ROOT a popis zdroje ("" = nic nenalezeno).
func (s *Scanner) resolveCover(bookID uuid.UUID, absDir, absFile string) (string, string, error) {
	// 1. Obrázek v adresáři – největší
	if src := largestImageInDir(absDir); src != "" {
		rel, err := imagestore.CopyFile(s.coverRoot, bookID.String(), src)
		return rel, "adresář: " + filepath.Base(src), err
	}

	// 2. Obrázek vložený v audio souboru
	if pic := embeddedCover(absFile); pic != nil {
		rel, err := imagestore.Write(s.coverRoot, bookID.String(), pic.Data, "."+strings.ToLower(pic.Ext))
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
		if e.IsDir() || !imagestore.IsImageExt(filepath.Ext(e.Name())) {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() == 0 || info.Size() > imagestore.MaxBytes {
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
	if pic == nil || len(pic.Data) == 0 || len(pic.Data) > imagestore.MaxBytes {
		return nil
	}
	return pic
}
