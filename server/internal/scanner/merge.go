package scanner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// MergeBook je kniha v plánu sloučení. Album tag se v běžném API nevrací
// (patří scanneru), tady je ale potřeba – podle něj se pozná, proč se kniha
// rozdělila.
type MergeBook struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	FilePath     string    `json:"file_path"`
	AlbumTag     string    `json:"album_tag"` // prázdné = kniha z dřívějších scanů bez tagu
	ChapterCount int       `json:"chapter_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// MergeGroup je jedna rozdělená kniha: cíl, do kterého se zdroje slijí.
type MergeGroup struct {
	Target  MergeBook   `json:"target"`  // nejstarší z knih – nese dřív importovaná metadata
	Sources []MergeBook `json:"sources"` // knihy, které se do cíle sloučí a zaniknou
}

// MergePlan popisuje, co by sloučení udělalo.
type MergePlan struct {
	Groups []MergeGroup `json:"groups"`
}

func (p MergePlan) IsEmpty() bool { return len(p.Groups) == 0 }

// MergeResult shrnuje provedené sloučení.
type MergeResult struct {
	Plan          MergePlan `json:"plan"`
	MergedBooks   int       `json:"merged_books"`   // kolik knih zaniklo
	MovedChapters int       `json:"moved_chapters"` // kolik kapitol změnilo knihu
}

// PlanMerge najde knihy, které scanner založil vícekrát, přestože jde o jednu
// knihu: leží na stejném místě v knihovně a mají stejný název.
//
// Vzniká to odlišným album tagem v části souborů (typicky rozbité kódování
// diakritiky). Scanner páruje soubory ke knize podle album tagu a umístění a na
// adresář záměrně nepadá – jeden adresář může nést celou sérii –, takže soubory
// s odlišným tagem založí druhou knihu. Shoda názvu je proto podmínka navíc:
// série v jednom adresáři má u každé knihy jiný název a do plánu se nedostane.
func PlanMerge(ctx context.Context, store *storage.Store) (MergePlan, error) {
	plan := MergePlan{Groups: []MergeGroup{}}

	books, err := store.ListBooks(ctx)
	if err != nil {
		return plan, fmt.Errorf("seznam knih: %w", err)
	}

	// Nejstarší kniha je v každé skupině první, takže se stane cílem. Při shodě
	// vteřiny (created_at nemá jemnější rozlišení) rozhoduje ID, ať je plán
	// stabilní mezi voláními.
	sort.Slice(books, func(i, j int) bool {
		if !books[i].CreatedAt.Equal(books[j].CreatedAt) {
			return books[i].CreatedAt.Before(books[j].CreatedAt)
		}
		return books[i].ID.String() < books[j].ID.String()
	})

	// Kbelík podle názvu, uvnitř shluky podle umístění.
	byTitle := map[string][][]model.Book{}
	keys := []string{}
	for _, book := range books {
		key := mergeKey(book.Title)
		if _, seen := byTitle[key]; !seen {
			keys = append(keys, key)
		}
		byTitle[key] = addToLocationGroup(byTitle[key], book)
	}

	for _, key := range keys {
		for _, group := range byTitle[key] {
			if len(group) < 2 {
				continue
			}
			sources := make([]MergeBook, 0, len(group)-1)
			for _, b := range group[1:] {
				sources = append(sources, mergeBook(b))
			}
			plan.Groups = append(plan.Groups, MergeGroup{Target: mergeBook(group[0]), Sources: sources})
		}
	}

	sort.Slice(plan.Groups, func(i, j int) bool {
		return plan.Groups[i].Target.Title < plan.Groups[j].Target.Title
	})
	return plan, nil
}

// addToLocationGroup zařadí knihu ke knihám na stejném místě v knihovně.
// Když sedí k více shlukům najednou, spojí je – jinak by výsledek závisel na
// pořadí knih na vstupu.
func addToLocationGroup(groups [][]model.Book, book model.Book) [][]model.Book {
	matched := []int{}
	for i, group := range groups {
		for _, member := range group {
			if SameBookLocation(member.FilePath, book.FilePath) {
				matched = append(matched, i)
				break
			}
		}
	}
	if len(matched) == 0 {
		return append(groups, []model.Book{book})
	}

	first := matched[0]
	groups[first] = append(groups[first], book)
	for _, i := range matched[1:] {
		groups[first] = append(groups[first], groups[i]...)
		groups[i] = nil
	}

	kept := make([][]model.Book, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			kept = append(kept, group)
		}
	}
	return kept
}

// ApplyMerge sloučí knihy podle plánu. Kapitoly se přesouvají, ne mažou –
// scanner je tedy nenačítá znovu a nemá šanci knihu podle album tagu zase
// rozdělit. Album tag zdrojů zaniká: pokud se kapitoly sloučené knihy někdy
// smažou (oprava kapitol), soubory s odlišným tagem se oddělí znovu. Trvale to
// řeší až přetagování souborů.
func ApplyMerge(ctx context.Context, store *storage.Store, plan MergePlan) (MergeResult, error) {
	result := MergeResult{Plan: plan}

	for _, group := range plan.Groups {
		sourceIDs := make([]uuid.UUID, 0, len(group.Sources))
		for _, s := range group.Sources {
			sourceIDs = append(sourceIDs, s.ID)
		}

		moved, err := store.MergeBooks(ctx, group.Target.ID, sourceIDs)
		if err != nil {
			return result, fmt.Errorf("sloučení knihy %q: %w", group.Target.Title, err)
		}
		result.MovedChapters += moved
		result.MergedBooks += len(sourceIDs)
	}
	return result, nil
}

// PlanMerge sestaví plán sloučení pro knihovnu tohoto scanneru.
func (s *Scanner) PlanMerge(ctx context.Context) (MergePlan, error) {
	return PlanMerge(ctx, s.store)
}

// FilterPlan nechá v plánu jen skupiny s uvedeným cílem. Prázdný seznam nechá
// plán beze změny.
//
// Shoda názvu a umístění nedokáže rozlišit skutečně rozdělenou knihu od dvou
// knih, kterým import metadat dal stejný název (dvě povídky jednoho vydání
// v sousedních adresářích). Poslední slovo má proto admin – vybere skupiny,
// které sloučit chce, ale co se s knihami stane, počítá pořád server.
func FilterPlan(plan MergePlan, targets []uuid.UUID) MergePlan {
	if len(targets) == 0 {
		return plan
	}

	wanted := make(map[uuid.UUID]bool, len(targets))
	for _, id := range targets {
		wanted[id] = true
	}

	filtered := MergePlan{Groups: []MergeGroup{}}
	for _, group := range plan.Groups {
		if wanted[group.Target.ID] {
			filtered.Groups = append(filtered.Groups, group)
		}
	}
	return filtered
}

// Merge sloučí rozdělené knihy za běhu serveru. Prázdný targets slučuje vše,
// jinak jen skupiny s uvedeným cílem.
//
// Plán se stejně jako u opravy kapitol sestavuje až tady, na serveru – klient
// posílá nanejvýš výběr skupin, nikdy seznam knih ke smazání. Po dobu zápisu
// drží ingestMu, aby watcher mezitím nezaložil další knihu ve stejném adresáři.
// Nový průchod knihovnou se nespouští: nic se nemazalo, každý soubor má svou
// kapitolu dál.
func (s *Scanner) Merge(ctx context.Context, targets []uuid.UUID) (MergeResult, error) {
	if s.Status().Running {
		return MergeResult{}, ErrScanRunning
	}

	// Zrušené spojení nesmí přerušit zápis v půli.
	ctx = context.WithoutCancel(ctx)

	s.ingestMu.Lock()
	defer s.ingestMu.Unlock()

	plan, err := PlanMerge(ctx, s.store)
	if err != nil {
		return MergeResult{}, err
	}
	plan = FilterPlan(plan, targets)
	if plan.IsEmpty() {
		return MergeResult{Plan: plan}, nil
	}

	result, err := ApplyMerge(ctx, s.store, plan)
	if err != nil {
		return result, err
	}

	s.log.Info("rozdělené knihy sloučeny",
		"knih", result.MergedBooks, "kapitol", result.MovedChapters)
	return result, nil
}

// mergeKey je název knihy zbavený rozdílů, které vznikají přepisem tagů:
// velikosti písmen, přebytečných mezer a diakritiky (soubory bez diakritiky
// v tagu jsou běžné). Rozbité kódování („PÍSEÈNÁ BOUØE“) tím srovnat nejde –
// takové knihy musí admin nejdřív pojmenovat stejně.
func mergeKey(title string) string {
	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		title,
	)
	if err != nil {
		folded = title
	}
	return strings.Join(strings.Fields(strings.ToLower(folded)), " ")
}

func mergeBook(b model.Book) MergeBook {
	albumTag := ""
	if b.AlbumTag != nil {
		albumTag = *b.AlbumTag
	}
	return MergeBook{
		ID:           b.ID,
		Title:        b.Title,
		FilePath:     b.FilePath,
		AlbumTag:     albumTag,
		ChapterCount: b.ChapterCount,
		CreatedAt:    b.CreatedAt,
	}
}
