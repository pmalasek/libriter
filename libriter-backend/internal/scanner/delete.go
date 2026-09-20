// internal/scanner/delete.go
//
// Mazání knihy z knihovny, volitelně i s audio soubory na disku.
//
// Patří ke scanneru ze dvou důvodů: zná AUDIO_ROOT a hlavně drží ingestMu,
// takže mu watcher nestihne smazané soubory ingestovat zpátky. Bez smazání
// souborů je mazání knihy jen dočasné – další průchod knihovnou ji ze
// zbylých souborů založí znovu.

package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"libriter/internal/audiostore"
	"libriter/internal/imagestore"

	"github.com/google/uuid"
)

// DeleteResult shrnuje, co po sobě mazání zanechalo.
type DeleteResult struct {
	Title        string `json:"title"`
	FilePath     string `json:"file_path"`
	Chapters     int    `json:"chapters"`
	DeletedFiles int    `json:"deleted_files"`
	FreedBytes   int64  `json:"freed_bytes"`
}

// DeleteBook smaže knihu z databáze. Při deleteFiles smaže i její audio
// soubory, obálku a adresáře, které po nich zůstaly prázdné.
//
// Soubory se mažou dřív než záznam: kdyby mazání selhalo, kniha v knihovně
// zůstane a je vidět, že se na disku neuklidilo. Opačné pořadí by nechalo
// osiřelé soubory, které by scanner vzápětí ingestoval jako novou knihu.
func (s *Scanner) DeleteBook(ctx context.Context, id uuid.UUID, deleteFiles bool) (DeleteResult, error) {
	// Zrušené spojení nesmí přerušit mazání v půli.
	ctx = context.WithoutCancel(ctx)

	s.ingestMu.Lock()
	defer s.ingestMu.Unlock()

	book, err := s.store.GetBook(ctx, id)
	if err != nil {
		return DeleteResult{}, err
	}

	result := DeleteResult{Title: book.Title, FilePath: book.FilePath}

	if deleteFiles {
		chapters, err := s.store.GetChaptersByBookID(ctx, id)
		if err != nil {
			return result, fmt.Errorf("kapitoly knihy: %w", err)
		}
		result.Chapters = len(chapters)

		// chapters.file_path má unikátní index, takže soubor nemůže patřit
		// zároveň jiné knize – stačí projít kapitoly téhle.
		dirs := map[string]bool{}
		for _, c := range chapters {
			abs, ok := audiostore.Resolve(s.audioRoot, c.FilePath)
			if !ok {
				s.log.Warn("nepoužitelná cesta ke kapitole, soubor se nemaže",
					"path", c.FilePath)
				continue
			}
			size := int64(0)
			if fi, err := os.Stat(abs); err == nil {
				size = fi.Size()
			}
			if err := os.Remove(abs); err != nil {
				if !os.IsNotExist(err) {
					return result, fmt.Errorf("mazání souboru %s: %w", c.FilePath, err)
				}
				continue
			}
			result.DeletedFiles++
			result.FreedBytes += size
			dirs[filepath.Dir(c.FilePath)] = true
		}

		s.removeEmptyDirs(dirs, book.FilePath)
	}

	if s.coverRoot != "" && book.CoverPath != nil && *book.CoverPath != "" {
		// Nepovedený úklid obrázku nesmí shodit mazání knihy.
		_ = imagestore.Remove(s.coverRoot, *book.CoverPath)
	}

	if err := s.store.DeleteBook(ctx, id); err != nil {
		return result, err
	}

	s.log.Info("kniha smazána", "title", book.Title, "dir", book.FilePath,
		"soubory", deleteFiles, "smazáno_souborů", result.DeletedFiles)
	return result, nil
}

// removeEmptyDirs uklidí adresáře, které po smazání souborů zůstaly prázdné.
// Postupuje odspoda nahoru a končí u adresáře knihy; nad něj nesahá, protože
// výš už jsou adresáře autora a AUDIO_ROOT.
//
// Adresáře mimo adresář knihy se nemažou vůbec – kapitola z cizí složky je
// legitimní stav (dvě vydání slepená do jedné knihy) a ta složka patří někomu
// jinému. os.Remove na neprázdném adresáři navíc selže sám, což drží
// sourozeneckou knihu ve stejné složce (celá série v jednom adresáři) v bezpečí.
func (s *Scanner) removeEmptyDirs(dirs map[string]bool, bookDir string) {
	if bookDir == "" || bookDir == "." {
		return
	}
	for dir := range dirs {
		for rel := dir; rel == bookDir || isSubdir(bookDir, rel); rel = filepath.Dir(rel) {
			abs, ok := audiostore.Resolve(s.audioRoot, rel)
			if !ok {
				break
			}
			if err := os.Remove(abs); err != nil {
				break // neprázdný nebo nesmazatelný – výš už nemá smysl jít
			}
			if rel == bookDir {
				break // adresář knihy je poslední, který smíme smazat
			}
		}
	}
}
