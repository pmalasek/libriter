// internal/scanner/duplicates.go
//
// Podezření na duplicitu: tatáž kniha zavedená dvakrát ve dvou nesouvisejících
// adresářích. Scanner tomu nebrání – identita kapitoly je cesta k souboru,
// takže kopie v jiné složce je z jeho pohledu nový obsah. Tady se jen pozná
// a nahlásí; co s tím, rozhoduje vždycky člověk.
//
// Proč to ostatní kontroly nechytí: oprava kapitol hledá duplikát jen ve
// stejném adresáři (stejný album tag + stejná cesta) a sloučení rozdělených
// knih vyžaduje SameBookLocation. Obojí je záměr – řeší jinou vadu. Kopie
// v nesouvisejícím adresáři propadne oběma.

package scanner

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// DuplicateBook je jedna kopie knihy v hlášení.
type DuplicateBook struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	FilePath        string    `json:"file_path"`
	AlbumTag        string    `json:"album_tag"` // prázdné = kniha bez tagu
	ChapterCount    int       `json:"chapter_count"`
	DurationSeconds int       `json:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

// DuplicateGroup je jedna kniha, která je v knihovně víckrát. Match říká,
// podle čeho se kopie poznaly – podle album tagu je shoda spolehlivější než
// podle názvu, který mohl přepsat import metadat.
type DuplicateGroup struct {
	// Key identifikuje skupinu napříč voláními; podle něj se odmítá
	// („není to duplicita“) i vrací zpátky mezi nálezy.
	Key   string          `json:"key"`
	Match string          `json:"match"` // "album" | "title"
	Books []DuplicateBook `json:"books"`
}

// DuplicateReport je výsledek kontroly. Nemá žádné „apply“: smazat kopii
// znamená sáhnout na soubory a to je rozhodnutí, které server neudělá sám.
//
// Dismissed drží skupiny, které někdo označil za planý poplach. Nemíchají se
// mezi nálezy, ale vracejí se v odpovědi, aby šlo rozhodnutí vzít zpět.
type DuplicateReport struct {
	Groups    []DuplicateGroup `json:"groups"`
	Dismissed []DuplicateGroup `json:"dismissed"`
}

func (r DuplicateReport) IsEmpty() bool { return len(r.Groups) == 0 }

// PlanDuplicates najde knihy, které vypadají jako tentýž titul zavedený
// vícekrát na nesouvisejících místech knihovny.
//
// Dvě podmínky musí platit zároveň:
//   - shoda album tagu, jinak shoda názvu zbaveného diakritiky a velikosti
//     písmen (mergeKey) – název se totiž importem metadat mění,
//   - různé umístění (!sameBookLocation) – knihy na stejném místě jsou buď
//     jedna rozdělená kniha (sloučení), nebo série v jednom adresáři.
//
// Falešný nález je možný: dvě vydání téhož titulu s jiným vypravěčem jsou
// legitimní. Proto je to hlášení, ne plán.
func PlanDuplicates(ctx context.Context, store *storage.Store) (DuplicateReport, error) {
	report := DuplicateReport{Groups: []DuplicateGroup{}, Dismissed: []DuplicateGroup{}}

	books, err := store.ListBooks(ctx)
	if err != nil {
		return report, fmt.Errorf("seznam knih: %w", err)
	}

	dismissedRows, err := store.ListDismissedDuplicates(ctx)
	if err != nil {
		return report, fmt.Errorf("odmítnuté duplicity: %w", err)
	}
	dismissed := make(map[string]bool, len(dismissedRows))
	for _, row := range dismissedRows {
		// Klíč musí sedět celý. Když se část knih mezitím smazala, kaskáda
		// odmítnutí zkrátila a na žádnou skupinu už nepasuje – nález se tím
		// správně objeví znovu.
		if storage.DuplicateGroupKey(row.BookIDs) == row.GroupKey {
			dismissed[row.GroupKey] = true
		}
	}

	// Nejstarší kniha je v každé skupině první – nese dřív importovaná
	// metadata a je to obvykle ta, kterou si uživatel chce nechat.
	sort.Slice(books, func(i, j int) bool {
		if !books[i].CreatedAt.Equal(books[j].CreatedAt) {
			return books[i].CreatedAt.Before(books[j].CreatedAt)
		}
		return books[i].ID.String() < books[j].ID.String()
	})

	// Podle album tagu se páruje přednostně; co se chytne tam, se už podle
	// názvu nehlásí podruhé.
	claimed := map[uuid.UUID]bool{}
	byAlbum := map[string][]model.Book{}
	byTitle := map[string][]model.Book{}
	albumKeys := []string{}
	titleKeys := []string{}

	for _, b := range books {
		if b.AlbumTag != nil && *b.AlbumTag != "" {
			key := mergeKey(*b.AlbumTag)
			if _, seen := byAlbum[key]; !seen {
				albumKeys = append(albumKeys, key)
			}
			byAlbum[key] = append(byAlbum[key], b)
		}
		key := mergeKey(b.Title)
		if _, seen := byTitle[key]; !seen {
			titleKeys = append(titleKeys, key)
		}
		byTitle[key] = append(byTitle[key], b)
	}

	add := func(match string, group []model.Book) {
		out := duplicateGroup(match, group)
		if dismissed[out.Key] {
			report.Dismissed = append(report.Dismissed, out)
			return
		}
		report.Groups = append(report.Groups, out)
	}

	for _, key := range albumKeys {
		for _, group := range splitByLocation(byAlbum[key]) {
			add("album", group)
			for _, b := range group {
				claimed[b.ID] = true
			}
		}
	}
	for _, key := range titleKeys {
		for _, group := range splitByLocation(byTitle[key]) {
			if anyClaimed(group, claimed) {
				continue
			}
			add("title", group)
		}
	}

	byTitleAsc := func(groups []DuplicateGroup) {
		sort.Slice(groups, func(i, j int) bool {
			return groups[i].Books[0].Title < groups[j].Books[0].Title
		})
	}
	byTitleAsc(report.Groups)
	byTitleAsc(report.Dismissed)
	return report, nil
}

// splitByLocation nechá ze skupiny stejně pojmenovaných knih jen ty, které
// leží na různých místech. Knihy na jednom místě řeší sloučení rozdělených
// knih, ne tahle kontrola.
func splitByLocation(books []model.Book) [][]model.Book {
	if len(books) < 2 {
		return nil
	}

	// Shluky podle umístění; kniha sedící k víc shlukům je spojí, aby výsledek
	// nezávisel na pořadí na vstupu (stejný princip jako u sloučení).
	locations := [][]model.Book{}
	for _, book := range books {
		locations = addToLocationGroup(locations, book)
	}
	if len(locations) < 2 {
		return nil
	}

	// Z každého místa jde do hlášení nejstarší kniha – zbytek téhož místa je
	// rozdělená kniha, což je jiná vada.
	group := make([]model.Book, 0, len(locations))
	for _, loc := range locations {
		group = append(group, loc[0])
	}
	return [][]model.Book{group}
}

func anyClaimed(books []model.Book, claimed map[uuid.UUID]bool) bool {
	for _, b := range books {
		if claimed[b.ID] {
			return true
		}
	}
	return false
}

func duplicateGroup(match string, books []model.Book) DuplicateGroup {
	ids := make([]uuid.UUID, 0, len(books))
	out := make([]DuplicateBook, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID)
		albumTag := ""
		if b.AlbumTag != nil {
			albumTag = *b.AlbumTag
		}
		out = append(out, DuplicateBook{
			ID:              b.ID,
			Title:           b.Title,
			FilePath:        b.FilePath,
			AlbumTag:        albumTag,
			ChapterCount:    b.ChapterCount,
			DurationSeconds: b.DurationSeconds,
			CreatedAt:       b.CreatedAt,
		})
	}
	return DuplicateGroup{Key: storage.DuplicateGroupKey(ids), Match: match, Books: out}
}

// PlanDuplicates sestaví hlášení pro knihovnu tohoto scanneru.
func (s *Scanner) PlanDuplicates(ctx context.Context) (DuplicateReport, error) {
	return PlanDuplicates(ctx, s.store)
}

// findSimilar hledá knihu, která vypadá jako ta právě zakládaná, ale leží
// jinde v knihovně. Volá se z createBook – v tu chvíli je jasné, že scanner
// zakládá nový záznam, a je to jediný okamžik, kdy se dá varovat.
func (s *Scanner) findSimilar(
	ctx context.Context, title, albumTag, relDir string,
) (*model.Book, error) {
	if albumTag != "" {
		candidates, err := s.store.GetBooksByAlbumTag(ctx, albumTag)
		if err != nil {
			return nil, err
		}
		for i := range candidates {
			if !sameBookLocation(candidates[i].FilePath, relDir) {
				return &candidates[i], nil
			}
		}
	}

	// Bez album tagu (nebo když podle něj nic nesedí) zbývá název. Musí se
	// srovnat diakritika a velikost písmen – tagy je mívají různě.
	books, err := s.store.ListBooks(ctx)
	if err != nil {
		return nil, err
	}
	key := mergeKey(title)
	for i := range books {
		if mergeKey(books[i].Title) == key && !sameBookLocation(books[i].FilePath, relDir) {
			return &books[i], nil
		}
	}
	return nil, nil
}

// ErrDuplicateGroupUnknown znamená, že skupina v aktuálním hlášení není –
// knihovna se mezitím změnila a rozhodnutí by padlo na něco jiného, než
// uživatel viděl.
var ErrDuplicateGroupUnknown = errors.New("skupina v hlášení není")

// DismissDuplicate označí skupinu za planý poplach. Klíč se ověřuje proti
// čerstvě sestavenému hlášení, takže odmítnout jde jen to, co server sám
// právě našel – klient si nemůže vymyslet vlastní skupinu.
func (s *Scanner) DismissDuplicate(
	ctx context.Context, groupKey string, actorID *uuid.UUID,
) (DuplicateGroup, error) {
	report, err := PlanDuplicates(ctx, s.store)
	if err != nil {
		return DuplicateGroup{}, err
	}

	for _, group := range report.Groups {
		if group.Key != groupKey {
			continue
		}
		ids := make([]uuid.UUID, 0, len(group.Books))
		for _, b := range group.Books {
			ids = append(ids, b.ID)
		}
		if err := s.store.DismissDuplicate(ctx, groupKey, ids, actorID); err != nil {
			return DuplicateGroup{}, err
		}
		s.log.Info("duplicita odmítnuta jako planý poplach",
			"title", group.Books[0].Title, "knih", len(group.Books))
		return group, nil
	}
	return DuplicateGroup{}, ErrDuplicateGroupUnknown
}

// RestoreDuplicate vrátí odmítnutou skupinu zpátky mezi nálezy.
func (s *Scanner) RestoreDuplicate(ctx context.Context, groupKey string) error {
	return s.store.RestoreDuplicate(ctx, groupKey)
}
