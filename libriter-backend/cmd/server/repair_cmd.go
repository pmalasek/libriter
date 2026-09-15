package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/scanner"
	"libriter/internal/storage"
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

Totéž umí administrace ve webovém rozhraní (Administrace → Knihovna), tam se
server zastavovat nemusí.
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

	plan, err := scanner.PlanRepair(ctx, store, audioRoot)
	if err != nil {
		return err
	}
	if plan.IsEmpty() {
		fmt.Println("kapitoly odpovídají souborům na disku – není co opravovat")
		return nil
	}

	printRepairPlan(plan)
	if !*apply {
		fmt.Println("\nnic se nezměnilo – opravu provede `libriter repair-chapters --apply`")
		return nil
	}

	result, err := scanner.ApplyRepair(ctx, store, plan)
	if err != nil {
		return err
	}

	fmt.Printf("\nsmazáno kapitol: %d, duplikátů: %d\n", result.DeletedChapters, result.DeletedBooks)
	fmt.Println("spusťte server – počáteční scan kapitoly načte znovu")
	return nil
}

func printRepairPlan(plan scanner.RepairPlan) {
	if len(plan.Rescan) > 0 {
		fmt.Printf("knihy k opětovnému načtení (%d):\n", len(plan.Rescan))
		for _, b := range plan.Rescan {
			fmt.Printf("  %s  [%s]\n", b.Title, b.FilePath)
		}
	}
	if len(plan.Duplicates) > 0 {
		fmt.Printf("duplikáty ke smazání (%d):\n", len(plan.Duplicates))
		for _, b := range plan.Duplicates {
			fmt.Printf("  %s  [%s]\n", b.Title, b.FilePath)
		}
	}
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
