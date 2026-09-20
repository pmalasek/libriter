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

const repairUsage = `Použití: libriter repair-chapters [--apply] [--force-missing]

Zkontroluje, že kapitoly v databázi odpovídají audio souborům v AUDIO_ROOT,
a najde knihy, které je potřeba načíst znovu:

  - knihy, jimž chybí soubory ležící na disku (dřívější verze scanneru si
    kapitoly na stejné pozici navzájem přepisovaly, např. dvoudiskové vydání)
  - knihy s kapitolami z cizího adresáře (dvě vydání slepená do jedné knihy)
  - dvě knihy se stejným album tagem v jednom adresáři (duplikát)
  - knihy, jejichž soubory na disku už nejsou (smazané z disku)

Bez --apply jen vypíše, co by udělal. S --apply smaže kapitoly dotčených knih,
nadbytečné duplikáty a záznamy bez souborů; kapitoly jsou odvozená data a
scanner je při dalším spuštění načte znovu se správnými pozicemi. Audio soubory
se nemažou nikdy. Spouštějte při zastaveném serveru.

Chybí-li příliš mnoho souborů, sepne pojistka proti nepřipojenému disku:
chybějící soubory se jen vypíšou a nesmažou. Teprve --force-missing je smaže
i tak – použijte, až ověříte, že je disk připojený.

Totéž umí administrace ve webovém rozhraní (Administrace → Knihovna), tam se
server zastavovat nemusí.
`

// runRepairChapters obsluhuje `libriter repair-chapters`.
func runRepairChapters(args []string) error {
	fs := flag.NewFlagSet("repair-chapters", flag.ContinueOnError)
	fs.Usage = func() { fmt.Print(repairUsage) }
	apply := fs.Bool("apply", false, "provést opravu (bez tohoto přepínače jen vypíše plán)")
	force := fs.Bool("force-missing", false, "smazat chybějící soubory i při sepnuté pojistce")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	store, roots, closeDB, err := openRepairStore(ctx)
	if err != nil {
		return err
	}
	defer closeDB()

	plan, err := scanner.PlanRepair(ctx, store, roots.audio)
	if err != nil {
		return err
	}
	if plan.IsEmpty() {
		fmt.Println("kapitoly odpovídají souborům na disku – není co opravovat")
		return nil
	}

	printRepairPlan(plan, *force)
	if !*apply {
		fmt.Println("\nnic se nezměnilo – opravu provede `libriter repair-chapters --apply`")
		return nil
	}

	result, err := scanner.ApplyRepair(ctx, store, roots.cover, plan, *force)
	if err != nil {
		return err
	}

	fmt.Printf("\nsmazáno kapitol: %d, knih: %d (z toho bez souborů na disku: %d)\n",
		result.DeletedChapters, result.DeletedBooks, result.DeletedOrphans)
	if result.Skipped {
		fmt.Println("chybějící soubory se kvůli pojistce přeskočily – viz --force-missing")
	}
	fmt.Println("spusťte server – počáteční scan kapitoly načte znovu")
	return nil
}

func printRepairPlan(plan scanner.RepairPlan, force bool) {
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
	if len(plan.Orphans) > 0 {
		fmt.Printf("knihy bez souborů na disku – smažou se celé (%d):\n", len(plan.Orphans))
		for _, b := range plan.Orphans {
			fmt.Printf("  %s  [%s]  %d kapitol\n", b.Title, b.FilePath, b.Total)
		}
	}
	if len(plan.Missing) > 0 {
		fmt.Printf("knihy s chybějícími kapitolami (%d):\n", len(plan.Missing))
		for _, b := range plan.Missing {
			fmt.Printf("  %s  [%s]  chybí %d z %d:\n", b.Title, b.FilePath, len(b.Chapters), b.Total)
			for _, c := range b.Chapters {
				fmt.Printf("      %s\n", c.FilePath)
			}
		}
	}
	if len(plan.Unresolvable) > 0 {
		fmt.Printf("kapitoly s nepoužitelnou cestou – nahlášeny, nemažou se (%d):\n",
			len(plan.Unresolvable))
		for _, c := range plan.Unresolvable {
			fmt.Printf("  %s  [%s]\n", c.Title, c.FilePath)
		}
	}
	if plan.Guard.Tripped {
		fmt.Printf("\nPOZOR: %s\n", plan.Guard.Reason)
		if force {
			fmt.Println("--force-missing je zapnutý – chybějící soubory se smažou i tak")
		} else {
			fmt.Println("chybějící soubory se proto přeskočí; vynutíte je přes --force-missing")
		}
	}
}

// dataRoots jsou adresáře, do kterých oprava sahá.
type dataRoots struct {
	audio string
	cover string
}

// openRepairStore otevře databázi (včetně migrací) a vrátí store i cesty k datům.
// Stejně jako správa uživatelů používá LoadCLI – nevyžaduje JWT_SECRET.
func openRepairStore(ctx context.Context) (*storage.Store, dataRoots, func(), error) {
	cfg, err := config.LoadCLI()
	if err != nil {
		return nil, dataRoots{}, nil, fmt.Errorf("konfigurace: %w", err)
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
		return nil, dataRoots{}, nil, fmt.Errorf("databáze: %w", err)
	}
	roots := dataRoots{audio: cfg.Storage.AudioRoot, cover: cfg.Storage.CoverRoot}
	return storage.New(sqlDB), roots, func() { _ = sqlDB.Close() }, nil
}
