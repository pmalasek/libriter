package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

const repairUsage = `Použití: libriter repair-chapters [--apply]

Zkontroluje, že kapitoly v databázi odpovídají audio souborům v AUDIO_ROOT,
a najde knihy, které je potřeba načíst znovu:

  - knihy, jimž chybí soubory ležící na disku (dřívější verze scanneru si
    kapitoly na stejné pozici navzájem přepisovaly, např. dvoudiskové vydání)
  - knihy s kapitolami z cizího adresáře (dvě vydání slepená do jedné knihy)
  - dvě knihy se stejným album tagem v jednom adresáři (duplikát)

Bez --apply jen vypíše, co by udělal. S --apply smaže kapitoly dotčených knih
a nadbytečné duplikáty; kapitoly jsou odvozená data a scanner je při dalším
spuštění načte znovu se správnými pozicemi. Spouštějte při zastaveném serveru.
`

// runRepairChapters obsluhuje `libriter repair-chapters`.
func runRepairChapters(args []string) error {
	fs := flag.NewFlagSet("repair-chapters", flag.ContinueOnError)
	fs.Usage = func() { fmt.Print(repairUsage) }
	apply := fs.Bool("apply", false, "provést opravu (bez tohoto přepínače jen vypíše plán)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	store, audioRoot, closeDB, err := openRepairStore(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	plan, err := planRepair(ctx, store, audioRoot)
	if err != nil {
		return err
	}
	if len(plan.rescan) == 0 && len(plan.duplicates) == 0 {
		fmt.Println("kapitoly odpovídají souborům na disku – není co opravovat")
		return nil
	}

	printRepairPlan(plan)
	if !*apply {
		fmt.Println("\nnic se nezměnilo – opravu provede `libriter repair-chapters --apply`")
		return nil
	}
	return applyRepair(ctx, store, plan)
}

// repairPlan popisuje, co je potřeba smazat, aby scanner načetl data znovu.
type repairPlan struct {
	rescan     []model.Book // knihy, jejichž kapitoly se smažou a načtou znovu
	duplicates []model.Book // nadbytečné knihy, které se smažou celé
}

// planRepair porovná kapitoly v DB se soubory v AUDIO_ROOT a sestaví plán.
func planRepair(ctx context.Context, store *storage.Store, audioRoot string) (repairPlan, error) {
	var plan repairPlan

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
		if !scanner.SameBookLocation(c.BookFilePath, dir) {
			broken[c.BookID] = true
		}
	}

	// Soubory na disku, které v DB chybí; jejich knihy je třeba načíst znovu.
	walkErr := filepath.WalkDir(audioRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !scanner.IsAudioFile(path) {
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
		plan.duplicates = append(plan.duplicates, group[1:]...)
	}

	duplicateIDs := make(map[uuid.UUID]bool, len(plan.duplicates))
	for _, b := range plan.duplicates {
		duplicateIDs[b.ID] = true
	}
	for _, b := range books {
		if broken[b.ID] && !duplicateIDs[b.ID] {
			plan.rescan = append(plan.rescan, b)
		}
	}
	return plan, nil
}

func printRepairPlan(plan repairPlan) {
	if len(plan.rescan) > 0 {
		fmt.Printf("knihy k opětovnému načtení (%d):\n", len(plan.rescan))
		for _, b := range plan.rescan {
			fmt.Printf("  %s  [%s]\n", b.Title, b.FilePath)
		}
	}
	if len(plan.duplicates) > 0 {
		fmt.Printf("duplikáty ke smazání (%d):\n", len(plan.duplicates))
		for _, b := range plan.duplicates {
			fmt.Printf("  %s  [%s]\n", b.Title, b.FilePath)
		}
	}
}

func applyRepair(ctx context.Context, store *storage.Store, plan repairPlan) error {
	deleted := 0
	for _, b := range plan.rescan {
		n, err := store.DeleteChaptersByBookID(ctx, b.ID)
		if err != nil {
			return fmt.Errorf("smazání kapitol knihy %q: %w", b.Title, err)
		}
		deleted += n
	}
	for _, b := range plan.duplicates {
		n, err := store.DeleteChaptersByBookID(ctx, b.ID)
		if err != nil {
			return fmt.Errorf("smazání kapitol duplikátu %q: %w", b.Title, err)
		}
		deleted += n
		if err := store.DeleteBook(ctx, b.ID); err != nil {
			return fmt.Errorf("smazání duplikátu %q: %w", b.Title, err)
		}
	}

	fmt.Printf("\nsmazáno kapitol: %d, duplikátů: %d\n", deleted, len(plan.duplicates))
	fmt.Println("spusťte server – počáteční scan kapitoly načte znovu")
	return nil
}

// openRepairStore otevře databázi (včetně migrací) a vrátí store i AUDIO_ROOT.
// Stejně jako správa uživatelů používá LoadCLI – nevyžaduje JWT_SECRET.
func openRepairStore(ctx context.Context) (*storage.Store, string, func(), error) {
	cfg, err := config.LoadCLI()
	if err != nil {
		return nil, "", nil, fmt.Errorf("konfigurace: %w", err)
	}

	// Vypsat, s čím se pracuje – jinak je snadné omylem opravit jinou databázi.
	abs, err := filepath.Abs(cfg.DB.Path)
	if err != nil {
		abs = cfg.DB.Path
	}
	fmt.Fprintf(os.Stderr, "databáze:   %s\n", abs)
	fmt.Fprintf(os.Stderr, "audio root: %s\n", cfg.Storage.AudioRoot)

	sqlDB, err := db.Open(ctx, cfg.DB)
	if err != nil {
		return nil, "", nil, fmt.Errorf("databáze: %w", err)
	}
	return storage.New(sqlDB), cfg.Storage.AudioRoot, func() { _ = sqlDB.Close() }, nil
}
