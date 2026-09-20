package scanner

import (
	"context"
	"errors"
	"testing"
)

// Táž kniha ve dvou nesouvisejících adresářích – přesně to, co uživatel
// způsobí dvojím nakopírováním. Ani oprava kapitol, ani sloučení to nechytí.
func TestPlanDuplicatesFindsCopyInOtherDir(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	first := createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	second := createRepairBook(t, store, "Osamělý mrtvý muž", "nove/osamely-2", "Osamělý mrtvý muž")

	report, err := PlanDuplicates(ctx, store)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if len(report.Groups) != 1 {
		t.Fatalf("skupin = %d, chtěna 1: %+v", len(report.Groups), report.Groups)
	}
	group := report.Groups[0]
	if group.Match != "album" {
		t.Errorf("shoda = %q, chtěno album", group.Match)
	}
	if len(group.Books) != 2 {
		t.Fatalf("knih ve skupině = %d, chtěny 2", len(group.Books))
	}

	// Obě kopie musí být v hlášení i s adresářem – podle něj uživatel pozná,
	// kterou smazat. Na pořadí se nespoléhá: created_at má rozlišení na
	// sekundy, takže knihy založené v jedné vteřině řadí až ID.
	paths := map[string]bool{}
	for _, b := range group.Books {
		paths[b.FilePath] = true
	}
	if !paths["smolik/osamely"] || !paths["nove/osamely-2"] {
		t.Errorf("adresáře = %v, chtěny oba", paths)
	}
	if group.Books[0].ID != first.ID && group.Books[0].ID != second.ID {
		t.Errorf("ve skupině je cizí kniha: %s", group.Books[0].ID)
	}
}

// Rozbitá diakritika v tagu nesmí duplicitu schovat: názvy se srovnávají
// stejně jako u sloučení rozdělených knih.
func TestPlanDuplicatesFoldsTitles(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	// Různé album tagy, takže se musí chytit shoda názvu.
	createRepairBook(t, store, "Ze života hmyzu", "capek/hmyz", "Ze zivota hmyzu")
	createRepairBook(t, store, "ZE ZIVOTA HMYZU", "zaloha/hmyz", "Jiny tag")

	report, err := PlanDuplicates(ctx, store)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if len(report.Groups) != 1 || report.Groups[0].Match != "title" {
		t.Fatalf("skupiny = %+v, chtěna 1 se shodou podle názvu", report.Groups)
	}
}

// Dva disky jedné knihy leží v podadresářích jejího adresáře. To není
// duplicita, ale jedna kniha – řeší ji sloučení rozdělených knih.
func TestPlanDuplicatesIgnoresSameLocation(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely/CD2", "Osamělý mrtvý muž")

	report, err := PlanDuplicates(ctx, store)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if !report.IsEmpty() {
		t.Errorf("hlášení = %+v, chtěno prázdno", report.Groups)
	}
}

// Série různých titulů v jednom adresáři se nesmí hlásit jako duplicita.
func TestPlanDuplicatesIgnoresSeriesInOneDir(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	createRepairBook(t, store, "Povídky z jedné kapsy", "capek/povidky", "Z jedné kapsy")
	createRepairBook(t, store, "Povídky z druhé kapsy", "capek/povidky", "Z druhé kapsy")

	report, err := PlanDuplicates(ctx, store)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if !report.IsEmpty() {
		t.Errorf("hlášení = %+v, chtěno prázdno", report.Groups)
	}
}

// Jedna kniha se nehlásí sama proti sobě.
func TestPlanDuplicatesIgnoresSingleBook(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	createRepairBook(t, store, "Krakatit", "capek/krakatit", "Krakatit")

	report, err := PlanDuplicates(ctx, store)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if !report.IsEmpty() {
		t.Errorf("hlášení = %+v, chtěno prázdno", report.Groups)
	}
}

// Skupina se nesmí objevit dvakrát – jednou podle album tagu a podruhé
// podle názvu, když sedí obojí.
func TestPlanDuplicatesReportsGroupOnce(t *testing.T) {
	ctx := context.Background()
	store, _ := newRepairEnv(t)

	createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	createRepairBook(t, store, "Osamělý mrtvý muž", "nove/osamely-2", "Osamělý mrtvý muž")

	report, err := PlanDuplicates(ctx, store)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if len(report.Groups) != 1 {
		t.Errorf("skupin = %d, chtěna 1: %+v", len(report.Groups), report.Groups)
	}
}

// findSimilar musí najít knihu jinde v knihovně, ne tu na stejném místě –
// jinak by scanner varoval u každého dalšího souboru téže knihy.
func TestFindSimilarIgnoresSameLocation(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")

	same, err := s.findSimilar(ctx, "Osamělý mrtvý muž", "Osamělý mrtvý muž", "smolik/osamely")
	if err != nil {
		t.Fatalf("findSimilar: %v", err)
	}
	if same != nil {
		t.Errorf("našel knihu na stejném místě: %+v", same)
	}

	other, err := s.findSimilar(ctx, "Osamělý mrtvý muž", "Osamělý mrtvý muž", "nove/osamely-2")
	if err != nil {
		t.Fatalf("findSimilar: %v", err)
	}
	if other == nil {
		t.Error("kopii v jiném adresáři nenašel")
	}
}

// Falešný nález (dvě vydání téhož titulu) jde odmítnout a hlášení ho pak
// nepřipomíná. Bez toho by v administraci visel napořád.
func TestDismissDuplicateHidesGroup(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	createRepairBook(t, store, "Osamělý mrtvý muž", "jine/vydani", "Osamělý mrtvý muž")

	before, err := s.PlanDuplicates(ctx)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if len(before.Groups) != 1 {
		t.Fatalf("skupin = %d, chtěna 1", len(before.Groups))
	}
	key := before.Groups[0].Key

	if _, err := s.DismissDuplicate(ctx, key, nil); err != nil {
		t.Fatalf("DismissDuplicate: %v", err)
	}

	after, err := s.PlanDuplicates(ctx)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if len(after.Groups) != 0 {
		t.Errorf("nálezy = %+v, chtěno prázdno", after.Groups)
	}
	if len(after.Dismissed) != 1 || after.Dismissed[0].Key != key {
		t.Fatalf("odmítnuté = %+v, chtěna skupina %s", after.Dismissed, key)
	}

	// Rozhodnutí musí jít vzít zpět.
	if err := s.RestoreDuplicate(ctx, key); err != nil {
		t.Fatalf("RestoreDuplicate: %v", err)
	}
	restored, err := s.PlanDuplicates(ctx)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if len(restored.Groups) != 1 || len(restored.Dismissed) != 0 {
		t.Errorf("po obnovení: nálezy = %+v, odmítnuté = %+v", restored.Groups, restored.Dismissed)
	}
}

// Přibude-li třetí kopie, je to nový nález – odmítnutí dvojice ho nesmí
// schovat, protože o té třetí knize nikdo nerozhodoval.
func TestDismissDuplicateReappearsWithNewCopy(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	createRepairBook(t, store, "Osamělý mrtvý muž", "jine/vydani", "Osamělý mrtvý muž")

	before, err := s.PlanDuplicates(ctx)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if _, err := s.DismissDuplicate(ctx, before.Groups[0].Key, nil); err != nil {
		t.Fatalf("DismissDuplicate: %v", err)
	}

	createRepairBook(t, store, "Osamělý mrtvý muž", "treti/kopie", "Osamělý mrtvý muž")

	after, err := s.PlanDuplicates(ctx)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if len(after.Groups) != 1 || len(after.Groups[0].Books) != 3 {
		t.Fatalf("nálezy = %+v, chtěna jedna skupina o 3 knihách", after.Groups)
	}
	if len(after.Dismissed) != 0 {
		t.Errorf("odmítnuté = %+v, chtěno prázdno (klíč už nesedí)", after.Dismissed)
	}
}

// Odmítnout jde jen skupinu, kterou server právě sám našel.
func TestDismissDuplicateRefusesUnknownKey(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	createRepairBook(t, store, "Krakatit", "capek/krakatit", "Krakatit")

	if _, err := s.DismissDuplicate(ctx, "vymysleny-klic", nil); !errors.Is(err, ErrDuplicateGroupUnknown) {
		t.Errorf("DismissDuplicate = %v, chtěno ErrDuplicateGroupUnknown", err)
	}
}

// Smazání jedné knihy ze skupiny odnese kaskádou i odmítnutí – zbylý klíč
// už na nic nesedí a nic se neschovává omylem.
func TestDismissDuplicateClearedWhenBookDeleted(t *testing.T) {
	ctx := context.Background()
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	first := createRepairBook(t, store, "Osamělý mrtvý muž", "smolik/osamely", "Osamělý mrtvý muž")
	createRepairBook(t, store, "Osamělý mrtvý muž", "jine/vydani", "Osamělý mrtvý muž")

	before, err := s.PlanDuplicates(ctx)
	if err != nil {
		t.Fatalf("PlanDuplicates: %v", err)
	}
	if _, err := s.DismissDuplicate(ctx, before.Groups[0].Key, nil); err != nil {
		t.Fatalf("DismissDuplicate: %v", err)
	}

	if _, err := s.DeleteBook(ctx, first.ID, false); err != nil {
		t.Fatalf("DeleteBook: %v", err)
	}

	rows, err := store.ListDismissedDuplicates(ctx)
	if err != nil {
		t.Fatalf("ListDismissedDuplicates: %v", err)
	}
	for _, row := range rows {
		if len(row.BookIDs) > 1 {
			t.Errorf("odmítnutí přežilo smazání knihy v plném rozsahu: %+v", row)
		}
	}
}
