package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// RepairBook je kniha v plánu opravy. Vlastní model.Book cestu k souborům
// v JSONu nevrací (patří scanneru), v administraci je ale potřeba – podle ní
// se pozná, o které vydání jde.
type RepairBook struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	FilePath string    `json:"file_path"`
}

// RepairPlan popisuje, co je potřeba smazat, aby scanner načetl data znovu.
type RepairPlan struct {
	Rescan     []RepairBook `json:"rescan"`     // knihy, jejichž kapitoly se smažou a načtou znovu
	Duplicates []RepairBook `json:"duplicates"` // nadbytečné knihy, které se smažou celé
}

func (p RepairPlan) IsEmpty() bool {
	return len(p.Rescan) == 0 && len(p.Duplicates) == 0
}

// RepairResult shrnuje provedenou opravu.
type RepairResult struct {
	Plan            RepairPlan `json:"plan"`
	DeletedChapters int        `json:"deleted_chapters"`
	DeletedBooks    int        `json:"deleted_books"`
}

// PlanRepair porovná kapitoly v DB se soubory v audioRoot a sestaví plán:
//
//   - knihy, jimž chybí soubory ležící na disku (dřívější verze scanneru si
//     kapitoly na stejné pozici navzájem přepisovaly)
//   - knihy s kapitolami z cizího adresáře (dvě vydání slepená do jedné knihy)
//   - dvě knihy se stejným album tagem v jednom adresáři (duplikát)
func PlanRepair(ctx context.Context, store *storage.Store, audioRoot string) (RepairPlan, error) {
	plan := RepairPlan{Rescan: []RepairBook{}, Duplicates: []RepairBook{}}

	books, err := store.ListBooks(ctx)
	if err != nil {
		return plan, fmt.Errorf("seznam knih: %w", err)
	}
	chapters, err := store.ListChapterFiles(ctx)
	if err != nil {
		return plan, fmt.Errorf("seznam kapitol: %w", err)
	}

	known := make(map[string]bool, len(chapters))
	booksInDir := make(map[string]map[uuid.UUID]bool)
	broken := make(map[uuid.UUID]bool)
	for _, c := range chapters {
		known[c.FilePath] = true
		dir := filepath.Dir(c.FilePath)
		if booksInDir[dir] == nil {
			booksInDir[dir] = make(map[uuid.UUID]bool)
		}
		booksInDir[dir][c.BookID] = true

		// Kapitola z adresáře, který s adresářem knihy nesouvisí.
		if !SameBookLocation(c.BookFilePath, dir) {
			broken[c.BookID] = true
		}
	}

	// Soubory na disku, které v DB chybí; jejich knihy je třeba načíst znovu.
	walkErr := filepath.WalkDir(audioRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !IsAudioFile(path) {
			return nil
		}
		rel, err := filepath.Rel(audioRoot, path)
		if err != nil || known[rel] {
			return nil
		}
		for bookID := range booksInDir[filepath.Dir(rel)] {
			broken[bookID] = true
		}
		return nil
	})
	if walkErr != nil {
		return plan, fmt.Errorf("průchod %s: %w", audioRoot, walkErr)
	}

	// Duplikáty: dvě knihy se stejným album tagem v jednom adresáři. Starší
	// zůstává (nese importovaná metadata), novější se maže.
	duplicates := []model.Book{}
	byAlbumDir := make(map[string][]model.Book)
	for _, b := range books {
		if b.AlbumTag == nil {
			continue
		}
		key := *b.AlbumTag + "\x00" + b.FilePath
		byAlbumDir[key] = append(byAlbumDir[key], b)
	}
	for _, group := range byAlbumDir {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].CreatedAt.Before(group[j].CreatedAt) })
		broken[group[0].ID] = true
		duplicates = append(duplicates, group[1:]...)
	}

	duplicateIDs := make(map[uuid.UUID]bool, len(duplicates))
	for _, b := range duplicates {
		duplicateIDs[b.ID] = true
		plan.Duplicates = append(plan.Duplicates, repairBook(b))
	}
	for _, b := range books {
		if broken[b.ID] && !duplicateIDs[b.ID] {
			plan.Rescan = append(plan.Rescan, repairBook(b))
		}
	}
	return plan, nil
}

// ApplyRepair smaže kapitoly dotčených knih a nadbytečné duplikáty. Kapitoly
// jsou odvozená data – scanner je při dalším průchodu načte znovu se
// správnými pozicemi.
func ApplyRepair(ctx context.Context, store *storage.Store, plan RepairPlan) (RepairResult, error) {
	result := RepairResult{Plan: plan}

	for _, b := range plan.Rescan {
		n, err := store.DeleteChaptersByBookID(ctx, b.ID)
		if err != nil {
			return result, fmt.Errorf("smazání kapitol knihy %q: %w", b.Title, err)
		}
		result.DeletedChapters += n
	}
	for _, b := range plan.Duplicates {
		n, err := store.DeleteChaptersByBookID(ctx, b.ID)
		if err != nil {
			return result, fmt.Errorf("smazání kapitol duplikátu %q: %w", b.Title, err)
		}
		result.DeletedChapters += n
		if err := store.DeleteBook(ctx, b.ID); err != nil {
			return result, fmt.Errorf("smazání duplikátu %q: %w", b.Title, err)
		}
		result.DeletedBooks++
	}
	return result, nil
}

// PlanRepair sestaví plán opravy pro knihovnu tohoto scanneru.
func (s *Scanner) PlanRepair(ctx context.Context) (RepairPlan, error) {
	return PlanRepair(ctx, s.store, s.audioRoot)
}

// Repair provede opravu za běhu serveru a spustí nový průchod knihovnou.
//
// Plán se sestavuje až tady, na serveru – klient si o opravu jen řekne, nikdy
// neposílá seznam knih ke smazání. Po dobu mazání drží ingestMu, takže watcher
// mezitím nic nevloží, a průchod knihovnou kapitoly zase načte.
func (s *Scanner) Repair(ctx context.Context) (RepairResult, error) {
	if s.Status().Running {
		return RepairResult{}, ErrScanRunning
	}

	// Jakmile se začne mazat, nesmí to zrušené spojení přerušit v půli –
	// databáze by zůstala rozpracovaná. Hodnoty z kontextu zůstávají.
	ctx = context.WithoutCancel(ctx)

	result, err := func() (RepairResult, error) {
		s.ingestMu.Lock()
		defer s.ingestMu.Unlock()

		plan, err := PlanRepair(ctx, s.store, s.audioRoot)
		if err != nil {
			return RepairResult{}, err
		}
		if plan.IsEmpty() {
			return RepairResult{Plan: plan}, nil
		}
		return ApplyRepair(ctx, s.store, plan)
	}()
	if err != nil {
		return result, err
	}

	if !result.Plan.IsEmpty() {
		s.log.Info("oprava kapitol provedena",
			"kapitol", result.DeletedChapters, "duplikátů", result.DeletedBooks)
		// Chybu ignorujeme: běžící scan kapitoly načte i tak.
		_ = s.Rescan()
	}
	return result, nil
}

func repairBook(b model.Book) RepairBook {
	return RepairBook{ID: b.ID, Title: b.Title, FilePath: b.FilePath}
}
