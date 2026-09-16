// internal/scanner/ingest.go
//
// Vkládá audio soubor do databáze jako kapitolu (chapter) v rámci knihy (book).
//
// Pravidlo seskupování (v tomto pořadí priority):
//  1. album tag + umístění v knihovně → identifikace ze samotného souboru
//  2. kniha z dřívějších scanů v témž adresáři → doplní se jí album tag
//  3. adresář                  → fallback, pokud album tag chybí
//
// books.file_path    = relativní cesta k adresáři od AUDIO_ROOT
// chapters.file_path = relativní cesta k souboru od AUDIO_ROOT
// chapters.position  = ruční pořadí od editora, jinak číslo disku + track
//                      z tagu, jinak přirozené pořadí názvu souboru v adresáři,
//                      nakonec další v řadě; kolize se posouvají na první volnou

package scanner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// discPositionStride je rozsah pozic vyhrazený jednomu disku. Vyšší než počet
// kapitol na disku, aby si disky nezasahovaly do pozic.
const discPositionStride = 1000

// ingest je volán z processFile (uvnitř ingestMu zámku).
func (s *Scanner) ingest(ctx context.Context, absPath, relPath string) error {
	relDir := filepath.Dir(relPath)
	if relDir == "." {
		s.log.Warn("soubor je přímo v AUDIO_ROOT – přesuňte ho do podadresáře",
			"path", relPath)
		return nil
	}

	dirName := filepath.Base(relDir)
	meta, err := extractMeta(absPath, dirName)
	if err != nil {
		return fmt.Errorf("extrakce metadat: %w", err)
	}

	duration := meta.DurationSeconds
	if duration <= 0 {
		duration = 1
		s.log.Warn("délka souboru neznámá – nainstalujte ffprobe", "path", relPath)
	}

	// Najdi nebo vytvoř autory (kniha jich může mít víc)
	authors, err := s.store.GetOrCreateAuthors(ctx, meta.Authors)
	if err != nil {
		return fmt.Errorf("get/create authors %q: %w", model.JoinAuthorNames(meta.Authors), err)
	}

	// Najdi existující knihu: nejdříve podle album tagu, pak podle adresáře
	book, err := s.findOrCreateBook(ctx, relDir, meta, authors, duration)
	if err != nil {
		return err
	}

	// Obálka: obrázek v adresáři, jinak obrázek vložený v audio souboru
	s.ensureCover(ctx, book, filepath.Dir(absPath), absPath)

	position, err := s.desiredPosition(ctx, book.ID, absPath, relPath, meta)
	if err != nil {
		return err
	}

	// Pozici už může držet jiný soubor (chybějící nebo opakující se track tagy).
	// Přepsat ji nelze – přepsaná kapitola by z DB zmizela a scanner by její
	// soubor ingestoval při každém startu znovu.
	position, err = s.store.NextFreeChapterPosition(ctx, book.ID, position, relPath)
	if err != nil {
		return err
	}

	return s.appendChapter(ctx, relPath, book, meta, position, duration)
}

// findOrCreateBook najde knihu podle album tagu (+ hlavní autor) nebo adresáře.
// Pokud neexistuje, vytvoří ji.
func (s *Scanner) findOrCreateBook(
	ctx context.Context,
	relDir string,
	meta *AudioMeta,
	authors []model.Author,
	duration int,
) (*model.Book, error) {
	// 1. Pokus: album tag + umístění v knihovně.
	// Páruje se podle album tagu, ne podle názvu knihy – ten se importem
	// metadat nebo ruční editací mění a soubory by ke knize přestaly pasovat.
	if meta.BookTitle != "" {
		candidates, err := s.store.GetBooksByAlbumTag(ctx, meta.BookTitle)
		if err != nil {
			return nil, fmt.Errorf("get books by album tag: %w", err)
		}
		for i := range candidates {
			if sameBookLocation(candidates[i].FilePath, relDir) {
				return &candidates[i], nil
			}
		}

		// 2. Pokus: kniha z dřívějších scanů ve stejném adresáři ještě album tag
		// nemá – je to tatáž kniha, jen dosud párovaná podle názvu. Tag se jí
		// doplní, aby se dál poznala i po přejmenování.
		book, err := s.store.GetBookWithoutAlbumTagInDir(ctx, relDir)
		if err == nil {
			if err := s.store.SetBookAlbumTag(ctx, book.ID, meta.BookTitle); err != nil {
				return nil, err
			}
			book.AlbumTag = &meta.BookTitle
			s.log.Info("kniha napojena na album tag", "title", book.Title,
				"album", meta.BookTitle, "dir", relDir)
			return book, nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("get book without album tag: %w", err)
		}

		// Žádná odpovídající kniha. Na adresář se dopadnout nesmí: jeden adresář
		// může obsahovat víc knih (celá série v jedné složce) a všechny by
		// splynuly do té první.
		return s.createBook(ctx, relDir, meta, authors, duration)
	}

	// 3. Pokus: adresář (fallback bez album tagu)
	book, err := s.store.GetBookByDirPath(ctx, relDir)
	if err == nil {
		return book, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("get book by dir: %w", err)
	}

	return s.createBook(ctx, relDir, meta, authors, duration)
}

// createBook vloží novou knihu podle metadat souboru.
func (s *Scanner) createBook(
	ctx context.Context,
	relDir string,
	meta *AudioMeta,
	authors []model.Author,
	duration int,
) (*model.Book, error) {
	var narratorPtr *string
	if meta.Narrator != "" {
		n := meta.Narrator
		narratorPtr = &n
	}

	title := meta.BookTitle
	if title == "" {
		title = filepath.Base(relDir)
	}

	// Album tag se knize uloží jako párovací klíč pro další scany.
	var albumTagPtr *string
	if meta.BookTitle != "" {
		albumTagPtr = &meta.BookTitle
	}

	in := storage.BookInput{
		AuthorIDs:       authorIDs(authors),
		Title:           title,
		Narrator:        narratorPtr,
		DurationSeconds: duration,
		FilePath:        relDir,
		AlbumTag:        albumTagPtr,
		Language:        "cs",
	}

	book, err := s.store.CreateBook(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("create book: %w", err)
	}

	s.log.Info("kniha vytvořena", "title", book.Title,
		"autoři", authorLabel(authors), "dir", relDir)
	return book, nil
}

// desiredPosition určí pozici kapitoly, v tomto pořadí: ruční pořadí nastavené
// editorem (přežívá opravu kapitol), disk a track z tagů, přirozené pořadí
// názvu mezi audio soubory adresáře ("2" před "10"), nakonec další v řadě.
func (s *Scanner) desiredPosition(
	ctx context.Context,
	bookID uuid.UUID,
	absPath, relPath string,
	meta *AudioMeta,
) (int, error) {
	position, ok, err := s.store.ChapterOrderOverride(ctx, bookID, relPath)
	if err != nil {
		return 0, err
	}
	if ok {
		return position, nil
	}

	if position := tagPosition(meta); position > 0 {
		return position, nil
	}

	if rank := naturalRank(filepath.Dir(absPath), filepath.Base(absPath)); rank > 0 {
		return rank, nil
	}

	count, _, err := s.store.GetBookChapterStats(ctx, bookID)
	if err != nil {
		return 0, fmt.Errorf("get chapter stats: %w", err)
	}
	return count + 1, nil
}

// tagPosition spočítá pozici kapitoly z tagů. U multi-disk vydání začínají
// track čísla na každém disku znovu od 1, proto se každému disku vyhradí
// vlastní rozsah pozic – jinak by si kapitoly disků navzájem sahaly na pozice.
// Mezery v číslování ničemu nevadí, kapitoly se řadí a offsety počítají podle
// relativního pořadí.
func tagPosition(meta *AudioMeta) int {
	if meta.TrackNumber <= 0 {
		return 0
	}
	if meta.DiscNumber > 1 {
		return (meta.DiscNumber-1)*discPositionStride + meta.TrackNumber
	}
	return meta.TrackNumber
}

// SameBookLocation je sameBookLocation pro volající mimo scanner (kontrola dat).
func SameBookLocation(bookDir, fileDir string) bool {
	return sameBookLocation(bookDir, fileDir)
}

// sameBookLocation rozhodne, zda kniha uložená v bookDir a soubor v relDir
// patří k sobě. Shoda názvu z album tagu sama nestačí: dvě vydání téhož titulu
// leží ve dvou samostatných adresářích a nesmí splynout do jedné knihy.
// Naopak kniha rozdělená na CD1/CD2 má disky v podadresářích adresáře knihy.
func sameBookLocation(bookDir, relDir string) bool {
	if bookDir == relDir {
		return true
	}
	// Jeden adresář je podadresářem druhého (kniha + její podsložka s diskem).
	if isSubdir(bookDir, relDir) || isSubdir(relDir, bookDir) {
		return true
	}
	// Sourozenecké adresáře (CD1 + CD2): společný rodič musí být adresář knihy.
	// Rodič v prvních dvou úrovních je adresář autora nebo přímo AUDIO_ROOT –
	// tam jsou sousedící adresáře různé knihy, ne disky jedné.
	parent := filepath.Dir(bookDir)
	sep := string(filepath.Separator)
	return parent == filepath.Dir(relDir) && strings.Contains(parent, sep)
}

// isSubdir vrátí true, pokud child leží uvnitř parent.
func isSubdir(parent, child string) bool {
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}

// authorIDs vrátí ID autorů v pořadí, v jakém byly načteny z tagu.
func authorIDs(authors []model.Author) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(authors))
	for _, a := range authors {
		ids = append(ids, a.ID)
	}
	return ids
}

// authorLabel vrátí jména autorů oddělená čárkou (pro log).
func authorLabel(authors []model.Author) string {
	names := make([]string, 0, len(authors))
	for _, a := range authors {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

// appendChapter vloží nebo aktualizuje kapitolu a přepočítá celkovou délku knihy.
func (s *Scanner) appendChapter(
	ctx context.Context,
	relPath string,
	book *model.Book,
	meta *AudioMeta,
	position int,
	duration int,
) error {
	if _, err := s.store.UpsertChapter(ctx, storage.ChapterInput{
		BookID:          book.ID,
		Position:        position,
		Title:           meta.ChapterTitle,
		FilePath:        relPath,
		DurationSeconds: duration,
	}); err != nil {
		return fmt.Errorf("upsert chapter: %w", err)
	}

	// Přepočítej celkovou délku knihy ze součtu všech kapitol
	_, total, err := s.store.GetBookChapterStats(ctx, book.ID)
	if err != nil {
		return fmt.Errorf("get chapter stats: %w", err)
	}
	if err = s.store.UpdateBookDuration(ctx, book.ID, total); err != nil {
		return fmt.Errorf("update book duration: %w", err)
	}

	s.log.Info("kapitola přidána",
		"book", book.Title,
		"position", position,
		"chapter", meta.ChapterTitle,
		"book_total_s", total,
	)
	return nil
}
