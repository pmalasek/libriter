package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// TrashDir je složka koše v každé knihovně. Začíná tečkou, takže ji průchod
// i sledování změn přeskakují (Ignored) a soubor v ní jde ručně vrátit.
const TrashDir = ".trash"

// ErrLastFile znamená, že soubor je jediný soubor filmu – ten se z aplikace
// nemaže, aby nešlo omylem smazat celý film.
var ErrLastFile = errors.New("jediný soubor filmu nelze smazat")

// TrashFile přesune soubor (a jeho titulky) do koše knihovny – podsložky
// .trash se zachovanou cestou – a odebere ho z knihovny. Film nebo díl bez
// souborů zůstane kvůli historii sledování, ve výpisech se nezobrazuje.
// Vrací cestu v koši (relativní k VIDEO_ROOT).
func (s *Scanner) TrashFile(ctx context.Context, mediaID uuid.UUID) (string, error) {
	s.ingest.Lock()
	defer s.ingest.Unlock()

	rec, err := s.store.MediaByID(ctx, mediaID)
	if err != nil {
		return "", err
	}
	if rec.MovieID != nil {
		paths, err := s.store.MovieFilePaths(ctx, *rec.MovieID)
		if err != nil {
			return "", err
		}
		if len(paths) < 2 {
			return "", ErrLastFile
		}
	}
	top, rest, ok := strings.Cut(rec.FilePath, "/")
	if !ok {
		return "", ErrRenumberInvalid
	}
	dest := path.Join(top, TrashDir, rest)
	dest = s.freeName(dest)
	if err := os.MkdirAll(filepath.Dir(s.abs(dest)), 0o755); err != nil {
		return "", fmt.Errorf("koš: %w", err)
	}
	// Disk napřed: kdyby přesun selhal, soubor zůstane v knihovně. Sledování
	// změn starou cestu smaže samo, DeleteMedia je pak bez účinku.
	if err := os.Rename(s.abs(rec.FilePath), s.abs(dest)); err != nil {
		return "", fmt.Errorf("přesun do koše %s: %w", rec.FilePath, err)
	}
	stem := strings.TrimSuffix(path.Base(rec.FilePath), path.Ext(rec.FilePath))
	destStem := strings.TrimSuffix(path.Base(dest), path.Ext(dest))
	for _, sub := range s.subtitleNames(rec.FilePath) {
		from := path.Join(path.Dir(rec.FilePath), sub)
		to := path.Join(path.Dir(dest), destStem+sub[len(stem):])
		if err := os.Rename(s.abs(from), s.abs(to)); err != nil {
			s.log.Warn("přesun titulků do koše", "path", from, "err", err)
		}
	}
	if err := s.store.DeleteMedia(ctx, mediaID); err != nil {
		return "", err
	}
	s.log.Info("soubor přesunut do koše", "z", rec.FilePath, "do", dest)
	return dest, nil
}

// freeName vrátí rel, nebo – když už na disku existuje – variantu
// "název (2).ext", "název (3).ext"…
func (s *Scanner) freeName(rel string) string {
	if _, err := os.Lstat(s.abs(rel)); err != nil {
		return rel
	}
	ext := path.Ext(rel)
	base := strings.TrimSuffix(rel, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Lstat(s.abs(candidate)); err != nil {
			return candidate
		}
	}
}
