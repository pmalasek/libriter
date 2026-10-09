// internal/scanner/import.go
//
// Založení knihy z importu (nahrání přes administraci).
//
// Na rozdíl od běžného ingestu tady o knize rozhoduje importér: ví, které
// soubory k sobě patří, v jakém pořadí a s jakými metadaty. Scanner proto
// knihu nehledá podle album tagu, jen soubory přesune do AUDIO_ROOT a založí
// knihu i kapitoly přesně podle zadání.
//
// Přesun i zápis běží pod ingestMu. Watcher sice nové soubory uvidí, ale
// processFile na stejném zámku počká a pak zjistí, že kapitola už existuje.

package scanner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// ImportFile je soubor, který se při importu přesune do knihovny.
type ImportFile struct {
	SrcPath string // absolutní cesta ve stagingu
	RelPath string // cílová cesta relativně k AUDIO_ROOT
}

// ImportChapter je audio soubor knihy s už známými metadaty.
type ImportChapter struct {
	ImportFile
	Title           string
	DurationSeconds int
	SizeBytes       int64
}

// ImportBook je kniha připravená importérem. Chapters jsou v cílovém pořadí,
// Extras jsou doprovodné soubory (obálky, bookinfo.html), které se jen přesunou.
type ImportBook struct {
	Book     storage.BookInput // FilePath = adresář knihy relativně k AUDIO_ROOT
	Chapters []ImportChapter
	Extras   []ImportFile
}

// ErrImportTargetExists znamená, že cílový soubor už v knihovně je.
var ErrImportTargetExists = errors.New("cílový soubor už v knihovně existuje")

// AudioRoot vrátí kořen knihovny (importér podle něj hlídá kolize).
func (s *Scanner) AudioRoot() string {
	return s.audioRoot
}

// ImportBook přesune soubory knihy do AUDIO_ROOT a založí knihu s kapitolami.
func (s *Scanner) ImportBook(ctx context.Context, in ImportBook) (*model.Book, error) {
	if len(in.Chapters) == 0 {
		return nil, errors.New("kniha nemá žádné kapitoly")
	}

	s.ingestMu.Lock()
	defer s.ingestMu.Unlock()

	// Kolize se hlídají před prvním přesunem, aby import nenechal knihu
	// v knihovně napůl.
	for _, ch := range in.Chapters {
		if exists, err := s.store.ChapterExistsByFilePath(ctx, ch.RelPath); err != nil {
			return nil, err
		} else if exists {
			return nil, fmt.Errorf("%w: %s", ErrImportTargetExists, ch.RelPath)
		}
		if _, err := os.Lstat(filepath.Join(s.audioRoot, ch.RelPath)); err == nil {
			return nil, fmt.Errorf("%w: %s", ErrImportTargetExists, ch.RelPath)
		}
	}

	for _, ch := range in.Chapters {
		if err := s.placeFile(ch.ImportFile); err != nil {
			return nil, err
		}
	}
	for _, extra := range in.Extras {
		// Doprovodný soubor není pro knihu nutný – chyba nemá shodit import.
		if err := s.placeFile(extra); err != nil {
			s.log.Warn("doprovodný soubor se nepodařilo přesunout", "path", extra.RelPath, "err", err)
		}
	}

	bookIn := in.Book
	bookIn.DurationSeconds = 0
	for _, ch := range in.Chapters {
		bookIn.DurationSeconds += max(ch.DurationSeconds, 1)
	}

	var albumTag string
	if bookIn.AlbumTag != nil {
		albumTag = *bookIn.AlbumTag
	}
	similar, simErr := s.findSimilar(ctx, bookIn.Title, albumTag, bookIn.FilePath)
	if simErr != nil {
		s.log.Debug("kontrolu duplicity se nepodařilo provést", "err", simErr)
	}

	book, err := s.store.CreateBook(ctx, bookIn)
	if err != nil {
		return nil, fmt.Errorf("create book: %w", err)
	}

	for i, ch := range in.Chapters {
		if _, err := s.store.UpsertChapter(ctx, storage.ChapterInput{
			BookID:          book.ID,
			Position:        i + 1,
			Title:           ch.Title,
			FilePath:        ch.RelPath,
			DurationSeconds: max(ch.DurationSeconds, 1),
			SizeBytes:       ch.SizeBytes,
		}); err != nil {
			return book, fmt.Errorf("upsert chapter: %w", err)
		}
	}

	// Pořadí z importu se uloží jako ruční – oprava kapitol soubory načítá
	// znovu a bez toho by je seřadila podle tagů nebo názvů.
	if err := s.pinChapterOrder(ctx, book.ID); err != nil {
		s.log.Warn("pořadí kapitol se nepodařilo uložit", "book", book.Title, "err", err)
	}

	first := filepath.Join(s.audioRoot, in.Chapters[0].RelPath)
	s.ensureCover(ctx, book, filepath.Join(s.audioRoot, bookIn.FilePath), first)

	s.log.Info("kniha importována", "title", book.Title, "dir", book.FilePath,
		"kapitol", len(in.Chapters))

	if similar != nil {
		s.addSuspect(Suspect{
			BookID:       book.ID,
			Title:        book.Title,
			FilePath:     book.FilePath,
			ExistingID:   similar.ID,
			ExistingPath: similar.FilePath,
			DetectedAt:   time.Now().UTC(),
		})
	}

	return s.store.GetBook(ctx, book.ID)
}

// pinChapterOrder zapíše současné pořadí kapitol jako ruční pořadí.
func (s *Scanner) pinChapterOrder(ctx context.Context, bookID uuid.UUID) error {
	chapters, err := s.store.GetChaptersByBookID(ctx, bookID)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0, len(chapters))
	for _, ch := range chapters {
		ids = append(ids, ch.ID)
	}
	_, err = s.store.ReorderChapters(ctx, bookID, ids)
	return err
}

// placeFile přesune soubor ze stagingu do knihovny. Staging bývá na stejném
// disku a stačí přejmenování; jinak se soubor zkopíruje a originál smaže.
func (s *Scanner) placeFile(f ImportFile) error {
	dst := filepath.Join(s.audioRoot, f.RelPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("vytvoření adresáře: %w", err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("%w: %s", ErrImportTargetExists, f.RelPath)
	}
	if err := os.Rename(f.SrcPath, dst); err == nil {
		return nil
	}
	if err := copyFile(f.SrcPath, dst); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("přesun %s: %w", f.RelPath, err)
	}
	_ = os.Remove(f.SrcPath)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// CleanAlbumTitle odstraní číselný prefix z názvu alba ("01 - Kniha" → "Kniha").
func CleanAlbumTitle(s string) string {
	return cleanAlbumTitle(s)
}

// NaturalLess řadí názvy souborů tak, jak je čte člověk ("2" před "10").
func NaturalLess(a, b string) bool {
	return naturalLess(a, b)
}

// ReadTagMeta vrátí jen metadata z tagů (bez náhradních hodnot z názvů).
func ReadTagMeta(absPath string) (*AudioMeta, error) {
	return readTagMeta(absPath)
}

// ProbeDuration vrátí délku souboru v sekundách přes ffprobe (0 = neznámá).
func ProbeDuration(absPath string) int {
	return ffprobeDuration(absPath)
}

// FindSimilarBook najde knihu s podobným názvem, která už v knihovně je
// (nil = žádná). Importér podle ní v náhledu varuje před duplicitou.
func (s *Scanner) FindSimilarBook(ctx context.Context, title string) (*model.Book, error) {
	return s.findSimilar(ctx, title, "", "")
}

// HasEmbeddedCover vrátí true, když audio soubor nese obálku v tagu.
func HasEmbeddedCover(absPath string) bool {
	return embeddedCover(absPath) != nil
}

// EmbeddedCover vrátí obálku z tagu audio souboru a její MIME typ.
func EmbeddedCover(absPath string) ([]byte, string, bool) {
	pic := embeddedCover(absPath)
	if pic == nil {
		return nil, "", false
	}
	return pic.Data, pic.MIMEType, true
}
