// Package importer nahrává knihy do knihovny přes administraci.
//
// Průběh jednoho importu (relace):
//  1. Create        – vznikne prázdný adresář ve stagingu (IMPORT_ROOT/<id>)
//  2. AddFile       – klient nahraje soubory po jednom (audio, zip, obrázky,
//     bookinfo.html, playlist.pls) i s relativní cestou ze složky
//  3. Analyze       – na pozadí se rozbalí zipy a rozpoznají knihy (náhled)
//  4. Commit        – uživatel náhled upraví a potvrdí; knihy se přesunou do
//     AUDIO_ROOT/<Autor>/<Název> a založí přes scanner
//
// Relace žijí jen v paměti. Staging leží mimo AUDIO_ROOT, aby na rozpracované
// soubory nesahal watcher scanneru; po restartu serveru se celý smaže.
package importer

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// State je fáze relace.
type State string

const (
	StateUploading State = "uploading" // čeká na soubory
	StateAnalyzing State = "analyzing" // rozbaluje a rozpoznává knihy
	StateReady     State = "ready"     // náhled je hotový, čeká na potvrzení
	StateImporting State = "importing" // přesouvá knihy do knihovny
	StateDone      State = "done"      // hotovo (výsledky v Results)
	StateFailed    State = "failed"    // analýza selhala (Error)
)

// sessionTTL je doba, po které se nečinná relace i se soubory smaže.
const sessionTTL = 24 * time.Hour

var (
	ErrNotFound     = errors.New("import nenalezen")
	ErrWrongState   = errors.New("import je v jiné fázi")
	ErrInvalidPath  = errors.New("neplatná cesta souboru")
	ErrFileType     = errors.New("nepodporovaný typ souboru")
	ErrTooLarge     = errors.New("překročen limit velikosti importu")
	ErrNoBooks      = errors.New("nahrané soubory neobsahují žádnou knihu")
	ErrNothingChose = errors.New("není vybrána žádná kniha")
	ErrInvalidBook  = errors.New("neplatné údaje knihy")
)

// Chapter je audio soubor knihy v náhledu.
type Chapter struct {
	Path            string `json:"path"` // relativně k adresáři knihy
	Title           string `json:"title"`
	DurationSeconds int    `json:"duration_seconds"`
	SizeBytes       int64  `json:"size_bytes"`
}

// Book je rozpoznaná kniha. Editovatelná pole mění Commit podle uživatele.
type Book struct {
	Key       string `json:"key"`   // adresář knihy ve stagingu ("." = kořen)
	Group     string `json:"group"` // nadřazená složka (autor/série), "" = žádná
	Include   bool   `json:"include"`
	HasCover  bool   `json:"has_cover"`
	Publisher string `json:"publisher"` // jen informativně z bookinfo.html

	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	Narrator       string   `json:"narrator"`
	Description    string   `json:"description"`
	SeriesTitle    string   `json:"series_title"`
	SeriesPosition *int     `json:"series_position"`
	// Language je jazyk knihy (ISO 639-1) z audio tagů; prázdný = tagy ho
	// neuvádějí a náhled ho předvyplní sám.
	Language string `json:"language"`

	Chapters        []Chapter `json:"chapters"`
	DurationSeconds int       `json:"duration_seconds"`
	SizeBytes       int64     `json:"size_bytes"`

	// Warnings jsou stabilní kódy, klient je přeloží.
	Warnings []string `json:"warnings"`
	// SimilarTitle je název podobné knihy, která už v knihovně je.
	SimilarTitle string `json:"similar_title,omitempty"`
}

// Result je výsledek importu jedné knihy.
type Result struct {
	Key    string     `json:"key"`
	Title  string     `json:"title"`
	BookID *uuid.UUID `json:"book_id,omitempty"`
	Error  string     `json:"error,omitempty"`
}

// Progress je průběh analýzy nebo importu.
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// Session je jedna relace importu.
type Session struct {
	ID            uuid.UUID `json:"id"`
	State         State     `json:"state"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	UploadedFiles int       `json:"uploaded_files"`
	UploadedBytes int64     `json:"uploaded_bytes"`
	MaxBytes      int64     `json:"max_bytes"`
	Progress      Progress  `json:"progress"`
	Books         []Book    `json:"books"`
	Skipped       []string  `json:"skipped"` // soubory, které se do žádné knihy nedostaly
	Results       []Result  `json:"results"`
	Error         string    `json:"error,omitempty"`
}

// libraryScanner je to, co importér potřebuje od scanneru.
type libraryScanner interface {
	AudioRoot() string
	ImportBook(ctx context.Context, in scanner.ImportBook) (*model.Book, error)
	FindSimilarBook(ctx context.Context, title string) (*model.Book, error)
	DefaultLanguage(ctx context.Context) string
}

// Service spravuje relace importu.
type Service struct {
	root     string // IMPORT_ROOT
	maxBytes int64
	scanner  libraryScanner
	store    *storage.Store
	log      *slog.Logger
	appCtx   context.Context

	mu       sync.Mutex
	sessions map[uuid.UUID]*Session
}

// ErrRootInsideLibrary znamená, že staging leží v AUDIO_ROOT.
var ErrRootInsideLibrary = errors.New("IMPORT_ROOT nesmí ležet uvnitř AUDIO_ROOT")

// New vytvoří službu. Relace z minulého běhu se smažou – v paměti restart
// nepřežijí a jejich soubory by jen zabíraly místo. Maže se jen to, co
// vypadá jako relace (adresář s UUID), nic jiného v IMPORT_ROOT.
func New(appCtx context.Context, root string, maxBytes int64, scn libraryScanner, store *storage.Store) (*Service, error) {
	if inside, err := isWithin(scn.AudioRoot(), root); err != nil {
		return nil, err
	} else if inside {
		return nil, ErrRootInsideLibrary
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	if entries, err := os.ReadDir(root); err == nil {
		for _, e := range entries {
			if _, err := uuid.Parse(e.Name()); err == nil && e.IsDir() {
				_ = os.RemoveAll(filepath.Join(root, e.Name()))
			}
		}
	}
	s := &Service{
		root:     root,
		maxBytes: maxBytes,
		scanner:  scn,
		store:    store,
		log:      slog.Default().With("component", "importer"),
		appCtx:   appCtx,
		sessions: make(map[uuid.UUID]*Session),
	}
	go s.janitor()
	return s, nil
}

// Create založí prázdnou relaci.
func (s *Service) Create() (*Session, error) {
	now := time.Now().UTC()
	sess := &Session{
		ID:        uuid.New(),
		State:     StateUploading,
		CreatedAt: now,
		UpdatedAt: now,
		MaxBytes:  s.maxBytes,
		Books:     []Book{},
		Skipped:   []string{},
		Results:   []Result{},
	}
	if err := os.MkdirAll(s.filesDir(sess.ID), 0o755); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()

	snap := *sess
	return &snap, nil
}

// MaxBytes vrátí limit velikosti jednoho importu.
func (s *Service) MaxBytes() int64 {
	return s.maxBytes
}

// Get vrátí kopii relace.
func (s *Service) Get(id uuid.UUID) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	return snapshot(sess), nil
}

// List vrátí všechny relace od nejnovější.
func (s *Service) List() []Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, *snapshot(sess))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// Delete zruší relaci a smaže její soubory. Běžící import zrušit nejde.
func (s *Service) Delete(id uuid.UUID) error {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	if sess.State == StateAnalyzing || sess.State == StateImporting {
		s.mu.Unlock()
		return ErrWrongState
	}
	delete(s.sessions, id)
	s.mu.Unlock()

	return os.RemoveAll(s.sessionDir(id))
}

func (s *Service) sessionDir(id uuid.UUID) string {
	return filepath.Join(s.root, id.String())
}

// filesDir je kořen nahraných souborů relace.
func (s *Service) filesDir(id uuid.UUID) string {
	return filepath.Join(s.sessionDir(id), "files")
}

// update provede změnu relace pod zámkem.
func (s *Service) update(id uuid.UUID, fn func(*Session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		fn(sess)
		sess.UpdatedAt = time.Now().UTC()
	}
}

// janitor maže opuštěné relace.
func (s *Service) janitor() {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-s.appCtx.Done():
			return
		case <-t.C:
		}

		var stale []uuid.UUID
		s.mu.Lock()
		for id, sess := range s.sessions {
			busy := sess.State == StateAnalyzing || sess.State == StateImporting
			if !busy && time.Since(sess.UpdatedAt) > sessionTTL {
				stale = append(stale, id)
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()

		for _, id := range stale {
			_ = os.RemoveAll(s.sessionDir(id))
			s.log.Info("opuštěný import smazán", "id", id)
		}
	}
}

// snapshot vrátí hlubokou kopii relace pro čtení mimo zámek.
func snapshot(sess *Session) *Session {
	c := *sess
	c.Books = make([]Book, len(sess.Books))
	for i, b := range sess.Books {
		b.Authors = append([]string{}, b.Authors...)
		b.Chapters = append([]Chapter{}, b.Chapters...)
		b.Warnings = append([]string{}, b.Warnings...)
		if b.SeriesPosition != nil {
			p := *b.SeriesPosition
			b.SeriesPosition = &p
		}
		c.Books[i] = b
	}
	c.Skipped = append([]string{}, sess.Skipped...)
	c.Results = append([]Result{}, sess.Results...)
	return &c
}

// isWithin vrátí true, když path leží v parent (nebo je to týž adresář).
func isWithin(parent, p string) (bool, error) {
	absParent, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	absPath, err := filepath.Abs(p)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(absParent, absPath)
	if err != nil {
		return false, nil
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}
