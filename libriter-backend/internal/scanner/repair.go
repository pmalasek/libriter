package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"libriter/internal/audiostore"
	"libriter/internal/imagestore"
	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// RepairBook je kniha v plánu opravy – zúžená na to, co administrace potřebuje
// zobrazit. Podle adresáře se pozná, o které vydání jde.
type RepairBook struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	FilePath string    `json:"file_path"`
}

// RepairChapter je kapitola, jejíž soubor na disku není.
type RepairChapter struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	FilePath string    `json:"file_path"`
}

// RepairMissing je kniha, které na disku chybí soubory. Chapters drží jen ty
// chybějící, Total počet všech kapitol knihy – z poměru je v rozhraní vidět,
// jestli zmizela celá kniha, nebo jen pár souborů.
type RepairMissing struct {
	ID       uuid.UUID       `json:"id"`
	Title    string          `json:"title"`
	FilePath string          `json:"file_path"`
	Total    int             `json:"total"`
	Chapters []RepairChapter `json:"chapters"`
}

// RepairGuard hlídá, aby odpojený disk knihovnu nevymazal. Tripped znamená, že
// se chybějící soubory do opravy nepromítnou – jen se vypíšou.
type RepairGuard struct {
	Tripped    bool    `json:"tripped"`
	Reason     string  `json:"reason"` // česky, rovnou k zobrazení
	Missing    int     `json:"missing"`
	Total      int     `json:"total"`
	Share      float64 `json:"share"`
	Limit      float64 `json:"limit"`
	Unreadable int     `json:"unreadable"` // soubory a adresáře, na které nešlo sáhnout
	Books      int     `json:"books"`      // knihy, které by se smazaly celé
	BookLimit  int     `json:"book_limit"`
}

// RepairPlan popisuje, co je potřeba smazat, aby scanner načetl data znovu.
type RepairPlan struct {
	Rescan     []RepairBook `json:"rescan"`     // knihy, jejichž kapitoly se smažou a načtou znovu
	Duplicates []RepairBook `json:"duplicates"` // nadbytečné knihy, které se smažou celé

	// Orphans jsou knihy, kterým na disku nezbyl ani jeden soubor – smažou se
	// celé. Missing jsou knihy, kterým chybí jen část; smažou se jim jen ty
	// kapitoly a kniha zůstane.
	Orphans []RepairMissing `json:"orphans"`
	Missing []RepairMissing `json:"missing"`

	// Unresolvable jsou kapitoly s cestou, kterou nejde bezpečně přeložit na
	// soubor (absolutní cesta, `..`). Jen se hlásí, automaticky se nemažou.
	Unresolvable []RepairChapter `json:"unresolvable"`

	Guard RepairGuard `json:"guard"`
}

// IsEmpty říká, jestli se má co dělat. Guard se do něj záměrně nepočítá –
// sepnutá pojistka je stav „nic se nemaže, ale tohle je špatně“ a rozhraní ho
// musí umět zobrazit i u prázdného plánu.
func (p RepairPlan) IsEmpty() bool {
	return len(p.Rescan) == 0 && len(p.Duplicates) == 0 &&
		len(p.Orphans) == 0 && len(p.Missing) == 0 && len(p.Unresolvable) == 0
}

// RepairResult shrnuje provedenou opravu.
type RepairResult struct {
	Plan            RepairPlan `json:"plan"`
	DeletedChapters int        `json:"deleted_chapters"` // součet za všechny kategorie
	DeletedBooks    int        `json:"deleted_books"`    // součet za všechny kategorie

	DeletedOrphans         int `json:"deleted_orphans"`
	DeletedMissingChapters int `json:"deleted_missing_chapters"`

	// Skipped je true, když pojistka sepnula a chybějící soubory se proto
	// přeskočily. Ostatní kategorie se opravily normálně.
	Skipped bool `json:"skipped"`
}

// Meze pojistky proti odpojenému disku. Chybí-li toho příliš, je pravděpodobnější,
// že knihovna není připojená, než že uživatel tolik souborů smazal.
const (
	missingShareLimit = 0.25 // podíl chybějících kapitol, nad který se pojistka sepne
	missingMinFiles   = 5    // pod tímhle počtem chybějících se podíl neřeší
	missingBookLimit  = 20   // víc knih naráz se bez potvrzení nesmaže
)

// ErrAudioRootUnavailable znamená, že AUDIO_ROOT nejde přečíst. Plán se pak
// nesestavuje vůbec – prázdný přípojný bod by vypadal jako prázdná knihovna
// a oprava by smazala všechno.
var ErrAudioRootUnavailable = errors.New("adresář s audiem není dostupný")

// scanFiles projde audioRoot a vrátí relativní cesty všech audio souborů.
// walkErrors počítá adresáře, na které nešlo sáhnout – WalkDir je mlčky
// přeskakuje a jejich soubory by jinak vypadaly jako smazané.
func scanFiles(audioRoot string) (onDisk map[string]bool, walkErrors int, err error) {
	fi, err := os.Stat(audioRoot)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s: %v", ErrAudioRootUnavailable, audioRoot, err)
	}
	if !fi.IsDir() {
		return nil, 0, fmt.Errorf("%w: %s není adresář", ErrAudioRootUnavailable, audioRoot)
	}

	onDisk = map[string]bool{}
	walkErr := filepath.WalkDir(audioRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			walkErrors++
			return nil
		}
		if d.IsDir() || !IsAudioFile(path) {
			return nil
		}
		if rel, err := filepath.Rel(audioRoot, path); err == nil {
			onDisk[rel] = true
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErrors, fmt.Errorf("průchod %s: %w", audioRoot, walkErr)
	}
	return onDisk, walkErrors, nil
}

// PlanRepair porovná kapitoly v DB se soubory v audioRoot a sestaví plán:
//
//   - knihy, jimž chybí soubory ležící na disku (dřívější verze scanneru si
//     kapitoly na stejné pozici navzájem přepisovaly)
//   - knihy s kapitolami z cizího adresáře (dvě vydání slepená do jedné knihy)
//   - dvě knihy se stejným album tagem v jednom adresáři (duplikát)
//   - knihy, jejichž soubory na disku už nejsou (celé i jen zčásti)
//
// Porovnává se oběma směry. Disk → DB doplní, co scanner ještě nenačetl;
// DB → disk najde záznamy, které po smazání souborů osiřely.
func PlanRepair(ctx context.Context, store *storage.Store, audioRoot string) (RepairPlan, error) {
	plan := RepairPlan{
		Rescan: []RepairBook{}, Duplicates: []RepairBook{},
		Orphans: []RepairMissing{}, Missing: []RepairMissing{},
		Unresolvable: []RepairChapter{},
	}

	books, err := store.ListBooks(ctx)
	if err != nil {
		return plan, fmt.Errorf("seznam knih: %w", err)
	}
	chapters, err := store.ListChapterFiles(ctx)
	if err != nil {
		return plan, fmt.Errorf("seznam kapitol: %w", err)
	}

	onDisk, walkErrors, err := scanFiles(audioRoot)
	if err != nil {
		return plan, err
	}
	// Prázdný adresář u neprázdné databáze je typicky nepřipojený disk, ne
	// vyprázdněná knihovna. Plán by v takovou chvíli navrhl smazat všechno.
	if len(onDisk) == 0 && len(chapters) > 0 {
		return plan, fmt.Errorf("%w: v %s není ani jeden audio soubor, ale databáze jich zná %d",
			ErrAudioRootUnavailable, audioRoot, len(chapters))
	}

	known := make(map[string]bool, len(chapters))
	booksInDir := make(map[string]map[uuid.UUID]bool)
	broken := make(map[uuid.UUID]bool)
	total := make(map[uuid.UUID]int, len(books))
	gone := make(map[uuid.UUID][]RepairChapter)
	// Nečitelné adresáře se počítají k nečitelným souborům: obojí znamená, že
	// se na část knihovny nešlo podívat, a obojí musí pojistku sepnout.
	unreadable := walkErrors

	for _, c := range chapters {
		known[c.FilePath] = true
		total[c.BookID]++
		dir := filepath.Dir(c.FilePath)
		if booksInDir[dir] == nil {
			booksInDir[dir] = make(map[uuid.UUID]bool)
		}
		booksInDir[dir][c.BookID] = true

		// Kapitola z adresáře, který s adresářem knihy nesouvisí.
		if !SameBookLocation(c.BookFilePath, dir) {
			broken[c.BookID] = true
		}

		if onDisk[c.FilePath] {
			continue
		}
		switch state := chapterFileState(audioRoot, c.FilePath); state {
		case fileGone:
			gone[c.BookID] = append(gone[c.BookID], repairChapter(c))
		case fileUnresolvable:
			plan.Unresolvable = append(plan.Unresolvable, repairChapter(c))
		case fileUnreadable:
			unreadable++
		case filePresent:
			// Průchod ho přeskočil (nečitelný podadresář), soubor ale je.
		}
	}

	// Soubory na disku, které v DB chybí; jejich knihy je třeba načíst znovu.
	for rel := range onDisk {
		if known[rel] {
			continue
		}
		for bookID := range booksInDir[filepath.Dir(rel)] {
			broken[bookID] = true
		}
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

	// Každá kniha padne právě do jednoho kbelíku. Duplikát má přednost (mizí
	// celý), po něm přenačtení – to spraví chybějící kapitoly zároveň s tím,
	// co na disku přebývá, takže je zbytečné mazat je zvlášť.
	missingChapters := 0
	for _, b := range books {
		missing := gone[b.ID]
		missingChapters += len(missing)

		switch {
		case duplicateIDs[b.ID]:
		case broken[b.ID]:
			plan.Rescan = append(plan.Rescan, repairBook(b))
		case len(missing) == 0:
		case total[b.ID] > 0 && len(missing) == total[b.ID]:
			// Kniha bez jediného souboru na disku. Podmínka total > 0 je
			// nutná: kniha, která ještě žádnou kapitolu nemá (pád mezi
			// založením knihy a vložením první kapitoly), osiřelá není.
			plan.Orphans = append(plan.Orphans, repairMissing(b, total[b.ID], missing))
		default:
			plan.Missing = append(plan.Missing, repairMissing(b, total[b.ID], missing))
		}
	}

	plan.Guard = evalGuard(missingChapters, len(chapters), len(plan.Orphans), unreadable)
	return plan, nil
}

// evalGuard rozhodne, jestli se chybějící soubory smí promítnout do opravy.
func evalGuard(missing, total, orphanBooks, unreadable int) RepairGuard {
	guard := RepairGuard{
		Missing:    missing,
		Total:      total,
		Limit:      missingShareLimit,
		Unreadable: unreadable,
		Books:      orphanBooks,
		BookLimit:  missingBookLimit,
	}
	if total > 0 {
		guard.Share = float64(missing) / float64(total)
	}

	switch {
	case unreadable > 0:
		guard.Tripped = true
		guard.Reason = fmt.Sprintf(
			"na %d souborů nebo adresářů nešlo sáhnout – část knihovny může být nedostupná, "+
				"ne smazaná", unreadable)
	case missing >= missingMinFiles && guard.Share > missingShareLimit:
		guard.Tripped = true
		guard.Reason = fmt.Sprintf(
			"chybí %d z %d kapitol (%.0f %%) – to vypadá spíš na nepřipojený disk než na mazání",
			missing, total, guard.Share*100)
	case orphanBooks > missingBookLimit:
		guard.Tripped = true
		guard.Reason = fmt.Sprintf(
			"smazalo by se %d knih naráz (strop je %d)", orphanBooks, missingBookLimit)
	}
	return guard
}

// Stav souboru kapitoly na disku.
type fileState int

const (
	filePresent fileState = iota
	fileGone
	fileUnreadable   // stat skončil jinou chybou než „neexistuje“
	fileUnresolvable // cestu z DB nejde bezpečně přeložit
)

// chapterFileState ověří kapitolu, kterou průchod nenašel. Rozlišit „soubor
// smazán“ od „nešlo se na něj podívat“ je celé jádro pojistky: bez toho by
// odpojený disk vypadal jako prázdná knihovna.
func chapterFileState(audioRoot, relPath string) fileState {
	abs, ok := audiostore.Resolve(audioRoot, relPath)
	if !ok {
		return fileUnresolvable
	}
	switch _, err := os.Stat(abs); {
	case err == nil:
		return filePresent
	case errors.Is(err, fs.ErrNotExist):
		return fileGone
	default:
		return fileUnreadable
	}
}

// ApplyRepair smaže kapitoly dotčených knih, nadbytečné duplikáty a záznamy,
// jejichž soubory na disku už nejsou. Kapitoly jsou odvozená data – scanner je
// při dalším průchodu načte znovu se správnými pozicemi. Ruční pořadí kapitol
// se drží zvlášť (chapter_order_overrides) a při novém načtení se obnoví.
//
// force přebíjí pojistku proti odpojenému disku; bez něj se při sepnuté
// pojistce chybějící soubory přeskočí a zbytek plánu se provede normálně.
// Soubory na disku se nemažou nikdy – oprava sahá jen do databáze.
func ApplyRepair(
	ctx context.Context, store *storage.Store, coverRoot string, plan RepairPlan, force bool,
) (RepairResult, error) {
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
		if err := deleteBookRecord(ctx, store, coverRoot, b.ID); err != nil {
			return result, fmt.Errorf("smazání duplikátu %q: %w", b.Title, err)
		}
		result.DeletedBooks++
	}

	if plan.Guard.Tripped && !force {
		result.Skipped = len(plan.Orphans) > 0 || len(plan.Missing) > 0
		return result, nil
	}

	// Knihy, kterým chybí jen část souborů: zmizí ty kapitoly, kniha zůstane.
	for _, b := range plan.Missing {
		ids := make([]uuid.UUID, 0, len(b.Chapters))
		for _, c := range b.Chapters {
			ids = append(ids, c.ID)
		}
		n, err := store.DeleteChaptersByIDs(ctx, ids)
		if err != nil {
			return result, fmt.Errorf("smazání chybějících kapitol knihy %q: %w", b.Title, err)
		}
		result.DeletedChapters += n
		result.DeletedMissingChapters += n

		if err := refreshBookDuration(ctx, store, b.ID); err != nil {
			return result, fmt.Errorf("přepočet délky knihy %q: %w", b.Title, err)
		}
	}

	// Knihy bez jediného souboru na disku: zmizí celé.
	for _, b := range plan.Orphans {
		n, err := store.DeleteChaptersByBookID(ctx, b.ID)
		if err != nil {
			return result, fmt.Errorf("smazání kapitol knihy %q: %w", b.Title, err)
		}
		result.DeletedChapters += n
		if err := deleteBookRecord(ctx, store, coverRoot, b.ID); err != nil {
			return result, fmt.Errorf("smazání knihy %q: %w", b.Title, err)
		}
		result.DeletedBooks++
		result.DeletedOrphans++
	}
	return result, nil
}

// refreshBookDuration srovná délku knihy se zbylými kapitolami.
//
// books.duration_seconds má CHECK (> 0), takže knize bez kapitol se délka
// nechává tak, jak byla – nula by zápis shodila.
func refreshBookDuration(ctx context.Context, store *storage.Store, bookID uuid.UUID) error {
	_, duration, err := store.GetBookChapterStats(ctx, bookID)
	if err != nil {
		return err
	}
	if duration <= 0 {
		return nil
	}
	return store.UpdateBookDuration(ctx, bookID, duration)
}

// deleteBookRecord smaže knihu i s obálkou, kterou jí scanner zkopíroval do
// COVER_ROOT. Bez toho by v něm po každé opravě zůstával osamocený obrázek.
// Audio soubory zůstávají – ty maže jen výslovný příkaz uživatele.
func deleteBookRecord(
	ctx context.Context, store *storage.Store, coverRoot string, bookID uuid.UUID,
) error {
	cover := ""
	if b, err := store.GetBook(ctx, bookID); err == nil && b.CoverPath != nil {
		cover = *b.CoverPath
	}
	if err := store.DeleteBook(ctx, bookID); err != nil {
		return err
	}
	if coverRoot != "" && cover != "" {
		// Kniha už je pryč; nepovedený úklid obrázku nesmí opravu shodit.
		_ = imagestore.Remove(coverRoot, cover)
	}
	return nil
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
func (s *Scanner) Repair(ctx context.Context, force bool) (RepairResult, error) {
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
		return ApplyRepair(ctx, s.store, s.coverRoot, plan, force)
	}()
	if err != nil {
		return result, err
	}

	if result.Plan.Guard.Tripped {
		s.log.Warn("pojistka opravy knihovny", "důvod", result.Plan.Guard.Reason,
			"vynuceno", force)
	}
	if !result.Plan.IsEmpty() {
		s.log.Info("oprava kapitol provedena",
			"kapitol", result.DeletedChapters, "knih", result.DeletedBooks,
			"osiřelých", result.DeletedOrphans)
		// Chybu ignorujeme: běžící scan kapitoly načte i tak.
		_ = s.Rescan()
	}
	return result, nil
}

func repairBook(b model.Book) RepairBook {
	return RepairBook{ID: b.ID, Title: b.Title, FilePath: b.FilePath}
}

func repairChapter(c storage.ChapterFile) RepairChapter {
	return RepairChapter{ID: c.ID, Title: c.Title, FilePath: c.FilePath}
}

func repairMissing(b model.Book, total int, chapters []RepairChapter) RepairMissing {
	return RepairMissing{
		ID: b.ID, Title: b.Title, FilePath: b.FilePath,
		Total: total, Chapters: chapters,
	}
}
