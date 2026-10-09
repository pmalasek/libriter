package scanner

import (
	"context"
	"errors"
	"testing"
	"time"

	"libriter/internal/storage"

	"github.com/google/uuid"
)

// Rozdvojená kniha: stejný adresář, stejný název, jiný album tag. Cílem je
// starší kniha, novější se má sloučit do ní.
func TestPlanMergeGroupsSplitBook(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	older := createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "PÍSEÈNÁ BOUØE")
	// created_at má vteřinovou přesnost, jinak není pořadí jednoznačné.
	time.Sleep(1100 * time.Millisecond)
	newer := createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "Písečná bouře")

	addChapter(t, store, older.ID, 1, "rollins/pisecna-boure/01.mp3")
	addChapter(t, store, older.ID, 2, "rollins/pisecna-boure/02.mp3")
	addChapter(t, store, newer.ID, 3, "rollins/pisecna-boure/03.mp3")

	plan, err := PlanMerge(ctx, store)
	if err != nil {
		t.Fatalf("PlanMerge: %v", err)
	}
	if len(plan.Groups) != 1 {
		t.Fatalf("skupin = %d, chtěna 1: %+v", len(plan.Groups), plan.Groups)
	}

	group := plan.Groups[0]
	if group.Target.ID != older.ID {
		t.Errorf("cíl = %s, chtěna starší kniha %s", group.Target.ID, older.ID)
	}
	if group.Target.AlbumTag != "PÍSEÈNÁ BOUØE" || group.Target.ChapterCount != 2 {
		t.Errorf("cíl = tag %q, %d kapitol; chtěno PÍSEÈNÁ BOUØE a 2", group.Target.AlbumTag, group.Target.ChapterCount)
	}
	if len(group.Sources) != 1 || group.Sources[0].ID != newer.ID {
		t.Fatalf("zdroje = %+v, chtěna novější kniha %s", group.Sources, newer.ID)
	}
	if group.Sources[0].ChapterCount != 1 || group.Sources[0].FilePath != "rollins/pisecna-boure" {
		t.Errorf("zdroj = %d kapitol, cesta %q", group.Sources[0].ChapterCount, group.Sources[0].FilePath)
	}
}

// Celá série v jednom adresáři (Letopisy Narnie) jsou různé knihy – názvy se
// liší, sloučit se nesmí.
func TestPlanMergeIgnoresDistinctTitlesInOneDir(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	dir := "lewis/letopisy-narnie"
	for _, title := range []string{"Lev, čarodějnice a skříň", "Princ Kaspian", "Plavba Jitřního poutníka"} {
		createRepairBook(t, store, title, dir, "Letopisy Narnie - "+title)
	}

	plan, err := PlanMerge(ctx, store)
	if err != nil {
		t.Fatalf("PlanMerge: %v", err)
	}
	if !plan.IsEmpty() {
		t.Errorf("plán = %+v, chtěno prázdno", plan.Groups)
	}
}

// Dvě vydání téhož titulu v sousedních adresářích autora jsou dvě knihy.
func TestPlanMergeIgnoresSiblingEditions(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	createRepairBook(t, store, "Hobit", "tolkien/hobit", "Hobit")
	createRepairBook(t, store, "Hobit", "tolkien/hobit-2001", "Hobit")

	plan, err := PlanMerge(ctx, store)
	if err != nil {
		t.Fatalf("PlanMerge: %v", err)
	}
	if !plan.IsEmpty() {
		t.Errorf("plán = %+v, chtěno prázdno (dvě vydání)", plan.Groups)
	}
}

// Názvy se srovnávají bez ohledu na velikost písmen, mezery a diakritiku.
func TestPlanMergeNormalizesTitle(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	dir := "rollins/pisecna-boure"
	createRepairBook(t, store, "Písečná bouře", dir, "tag1")
	createRepairBook(t, store, "  PÍSEČNÁ   BOUŘE", dir, "tag2")
	createRepairBook(t, store, "Pisecna boure", dir, "tag3")

	plan, err := PlanMerge(ctx, store)
	if err != nil {
		t.Fatalf("PlanMerge: %v", err)
	}
	if len(plan.Groups) != 1 || len(plan.Groups[0].Sources) != 2 {
		t.Fatalf("plán = %+v, chtěna 1 skupina se 2 zdroji", plan.Groups)
	}
}

// Kniha rozdělená na disky má soubory v podadresářích – i tak je to jedna kniha.
func TestPlanMergeJoinsDiscSubdirs(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure/CD1", "Pisecna boure CD1")
	createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure/CD2", "Pisecna boure CD2")

	plan, err := PlanMerge(ctx, store)
	if err != nil {
		t.Fatalf("PlanMerge: %v", err)
	}
	if len(plan.Groups) != 1 || len(plan.Groups[0].Sources) != 1 {
		t.Fatalf("plán = %+v, chtěna 1 skupina s 1 zdrojem", plan.Groups)
	}
}

// Sloučení přesune kapitoly do cíle, přepočítá délku a nenechá po sobě nic,
// co by musela řešit oprava kapitol.
func TestApplyMergeMovesChapters(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)

	older := createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "PÍSEÈNÁ BOUØE")
	time.Sleep(1100 * time.Millisecond)
	newer := createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "Písečná bouře")

	writeAudio(t, audioRoot, "rollins/pisecna-boure/01.mp3")
	writeAudio(t, audioRoot, "rollins/pisecna-boure/02.mp3")
	writeAudio(t, audioRoot, "rollins/pisecna-boure/03.mp3")
	addChapter(t, store, older.ID, 1, "rollins/pisecna-boure/01.mp3")
	addChapter(t, store, older.ID, 2, "rollins/pisecna-boure/02.mp3")
	addChapter(t, store, newer.ID, 3, "rollins/pisecna-boure/03.mp3")

	plan, err := PlanMerge(ctx, store)
	if err != nil {
		t.Fatalf("PlanMerge: %v", err)
	}

	result, err := ApplyMerge(ctx, store, plan)
	if err != nil {
		t.Fatalf("ApplyMerge: %v", err)
	}
	if result.MergedBooks != 1 || result.MovedChapters != 1 {
		t.Errorf("sloučeno knih = %d, přesunuto kapitol = %d; chtěno 1 a 1",
			result.MergedBooks, result.MovedChapters)
	}

	chapters, err := store.GetChaptersByBookID(ctx, older.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	if len(chapters) != 3 {
		t.Errorf("kapitol cíle = %d, chtěny 3", len(chapters))
	}
	if _, err := store.GetBook(ctx, newer.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("zdrojová kniha zůstala: %v", err)
	}

	// Po sloučení nezbývá co slučovat ani opravovat.
	if plan, err := PlanMerge(ctx, store); err != nil || !plan.IsEmpty() {
		t.Errorf("plán po sloučení = %+v (err %v), chtěno prázdno", plan.Groups, err)
	}
	repairPlan, err := PlanRepair(ctx, store, audioRoot)
	if err != nil {
		t.Fatalf("PlanRepair: %v", err)
	}
	if !repairPlan.IsEmpty() {
		t.Errorf("oprava kapitol = %+v, chtěno prázdno", repairPlan)
	}
}

// Výběr skupin: admin může sloučit jen některé nálezy. Shoda názvu a umístění
// nepozná knihu rozdělenou scannerem od dvou knih, kterým import metadat dal
// stejný název.
func TestFilterPlanKeepsSelectedGroups(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "PÍSEÈNÁ BOUØE")
	createRepairBook(t, store, "Písečná bouře", "rollins/pisecna-boure", "Písečná bouře")
	createRepairBook(t, store, "Otázka ceny", "sapkowski/posledni-prani/4-otazka-ceny", "")
	createRepairBook(t, store, "Otázka ceny", "sapkowski/posledni-prani/5-konec-sveta", "")

	plan, err := PlanMerge(ctx, store)
	if err != nil {
		t.Fatalf("PlanMerge: %v", err)
	}
	if len(plan.Groups) != 2 {
		t.Fatalf("skupin = %d, chtěny 2", len(plan.Groups))
	}

	// Která z dvojice je cíl, rozhoduje čas vzniku (a při shodě vteřiny ID),
	// proto se bere z plánu.
	var wanted uuid.UUID
	for _, group := range plan.Groups {
		if group.Target.Title == "Písečná bouře" {
			wanted = group.Target.ID
		}
	}

	filtered := FilterPlan(plan, []uuid.UUID{wanted})
	if len(filtered.Groups) != 1 || filtered.Groups[0].Target.ID != wanted {
		t.Errorf("po výběru = %+v, chtěna jen skupina %s", filtered.Groups, wanted)
	}
	// Prázdný výběr nechává plán celý.
	if len(FilterPlan(plan, nil).Groups) != 2 {
		t.Error("prázdný výběr plán osekal")
	}
}

// Za běhu průchodu knihovnou se neslučuje – scanner by mezitím mohl zakládat
// další knihy.
func TestMergeRefusesWhileScanRunning(t *testing.T) {
	store, audioRoot := newRepairEnv(t)

	s := New(audioRoot, t.TempDir(), store)
	if !s.beginScan("test") {
		t.Fatal("beginScan: průchod už běží")
	}

	if _, err := s.Merge(context.Background(), nil); !errors.Is(err, ErrScanRunning) {
		t.Errorf("Merge při běžícím scanu: %v, chtěno ErrScanRunning", err)
	}
}
