// internal/scanner/scanner.go
//
// Sleduje AUDIO_ROOT a automaticky ingestuje nové audio soubory do DB.
//
// Životní cyklus souboru:
//  1. fsnotify hlásí CREATE nebo WRITE na audio soubor
//  2. Scanner čeká, dokud se velikost souboru nepřestane měnit
//     (soubor je kompletně zkopírován / přenesen)
//  3. Extrahuje metadata z audio tagů (dhowden/tag) + délku (ffprobe)
//  4. Najde nebo vytvoří autora a vloží knihu do DB

package scanner

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"libriter/internal/storage"

	"github.com/fsnotify/fsnotify"
)

const (
	stabilizeEvery = 3 * time.Second  // jak často kontrolovat velikost souboru
	stabilizeFor   = 10 * time.Second // jak dlouho musí být velikost beze změny
	ingestTimeout  = 30 * time.Minute // maximální čekání (velké soubory přes pomalou síť)
)

var audioExts = map[string]bool{
	".mp3": true, ".m4a": true, ".m4b": true,
	".ogg": true, ".flac": true, ".opus": true,
	".aac": true, ".wav": true,
}

// Scanner sleduje AUDIO_ROOT a při detekci nového audio souboru ho ingestuje do DB.
type Scanner struct {
	audioRoot string
	store     *storage.Store
	log       *slog.Logger

	mu      sync.Mutex
	pending map[string]struct{} // soubory, pro které běží goroutina waitAndProcess

	ingestMu sync.Mutex // serialisuje zápisy do DB (GetOrCreateAuthor není idempotentní bez UNIQUE)
}

// New vytvoří nový Scanner.
func New(audioRoot string, store *storage.Store) *Scanner {
	return &Scanner{
		audioRoot: audioRoot,
		store:     store,
		log:       slog.Default().With("component", "scanner"),
		pending:   make(map[string]struct{}),
	}
}

// Start spustí počáteční scan a file watcher na pozadí.
// Vrací okamžitě; veškerá práce běží v goroutinách, které respektují ctx.
func (s *Scanner) Start(ctx context.Context) {
	go s.runInitialScan(ctx)
	go s.runWatcher(ctx)
}

// ---- interní pomocné metody ----

func (s *Scanner) isAudio(path string) bool {
	return audioExts[strings.ToLower(filepath.Ext(path))]
}

// relPath vrátí cestu relativní k audioRoot pro ukládání do DB.
func (s *Scanner) relPath(absPath string) string {
	rel, err := filepath.Rel(s.audioRoot, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

// markPending přidá soubor do setu a vrátí true, pokud ještě nebyl přidán.
func (s *Scanner) markPending(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[path]; ok {
		return false
	}
	s.pending[path] = struct{}{}
	return true
}

func (s *Scanner) clearPending(path string) {
	s.mu.Lock()
	delete(s.pending, path)
	s.mu.Unlock()
}

// ---- počáteční scan ----

// runInitialScan projde AUDIO_ROOT a ingestuje soubory, které v DB chybí.
// Soubory se zpracovávají sériově, aby nebylo zatížení při startu příliš velké.
func (s *Scanner) runInitialScan(ctx context.Context) {
	s.log.Info("spouštím počáteční scan", "root", s.audioRoot)
	count := 0

	err := filepath.WalkDir(s.audioRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !s.isAudio(path) {
			return nil
		}
		select {
		case <-ctx.Done():
			return filepath.SkipAll
		default:
		}
		s.processFile(ctx, path)
		count++
		return nil
	})

	if err != nil {
		s.log.Warn("počáteční scan ukončen s chybou", "err", err)
	} else {
		s.log.Info("počáteční scan dokončen", "zpracováno_souborů", count)
	}
}

// ---- file watcher ----

// runWatcher sleduje AUDIO_ROOT pomocí fsnotify a reaguje na nové soubory.
func (s *Scanner) runWatcher(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Error("nelze spustit file watcher", "err", err)
		return
	}
	defer watcher.Close()

	// Přidáme root a všechny stávající podadresáře
	_ = filepath.WalkDir(s.audioRoot, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = watcher.Add(path)
		}
		return nil
	})

	s.log.Info("file watcher aktivní", "root", s.audioRoot)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			s.handleFSEvent(ctx, watcher, event)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			s.log.Warn("watcher chyba", "err", err)
		}
	}
}

func (s *Scanner) handleFSEvent(ctx context.Context, watcher *fsnotify.Watcher, event fsnotify.Event) {
	// Nový adresář → přidat do watcheru, aby byly sledovány i soubory v něm
	if event.Has(fsnotify.Create) {
		if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
			_ = watcher.Add(event.Name)
			s.log.Debug("přidán do watcheru", "dir", event.Name)
			return
		}
	}

	// Nový nebo zapsaný audio soubor → čekej na stabilitu a ingestuj
	if (event.Has(fsnotify.Create) || event.Has(fsnotify.Write)) && s.isAudio(event.Name) {
		if s.markPending(event.Name) {
			s.log.Info("detekován nový soubor, čekám na dokončení přenosu",
				"path", s.relPath(event.Name))
			go s.waitAndProcess(ctx, event.Name)
		}
	}
}

// ---- čekání na stabilitu souboru ----

// waitAndProcess opakovaně kontroluje velikost souboru.
// Jakmile se nepřestane měnit po dobu stabilizeFor, soubor ingestuje.
func (s *Scanner) waitAndProcess(ctx context.Context, absPath string) {
	defer s.clearPending(absPath)

	deadline := time.Now().Add(ingestTimeout)
	var lastSize int64 = -1
	var stableFor time.Duration

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(stabilizeEvery):
		}

		if time.Now().After(deadline) {
			s.log.Warn("soubor se nestabilizoval v časovém limitu, přeskakuji",
				"path", s.relPath(absPath), "limit", ingestTimeout)
			return
		}

		fi, err := os.Stat(absPath)
		if err != nil {
			// Soubor byl smazán nebo přesunut
			s.log.Debug("soubor zmizel před ingestem", "path", s.relPath(absPath))
			return
		}

		currentSize := fi.Size()
		if currentSize > 0 && currentSize == lastSize {
			stableFor += stabilizeEvery
			s.log.Debug("soubor stabilní", "path", s.relPath(absPath),
				"size_bytes", currentSize, "stable_for", stableFor)
			if stableFor >= stabilizeFor {
				s.processFile(ctx, absPath)
				return
			}
		} else {
			if currentSize != lastSize {
				s.log.Debug("soubor se mění", "path", s.relPath(absPath),
					"size_bytes", currentSize)
			}
			lastSize = currentSize
			stableFor = 0
		}
	}
}

// ---- ingest pipeline ----

// processFile ověří, zda kapitola v DB chybí, a případně ji ingestuje.
func (s *Scanner) processFile(ctx context.Context, absPath string) {
	rel := s.relPath(absPath)

	// Rychlá kontrola mimo zámek – kapitola identifikována cestou k souboru
	exists, err := s.store.ChapterExistsByFilePath(ctx, rel)
	if err != nil {
		s.log.Error("chyba dotazu do DB", "path", rel, "err", err)
		return
	}
	if exists {
		s.log.Debug("soubor již v DB, přeskakuji", "path", rel)
		return
	}

	// Serialisovaný zápis
	s.ingestMu.Lock()
	defer s.ingestMu.Unlock()

	// Dvojitá kontrola po získání zámku
	exists, err = s.store.ChapterExistsByFilePath(ctx, rel)
	if err != nil || exists {
		return
	}

	if err := s.ingest(ctx, absPath, rel); err != nil {
		s.log.Error("ingest selhal", "path", rel, "err", err)
	}
}
