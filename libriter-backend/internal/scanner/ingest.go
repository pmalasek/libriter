// internal/scanner/ingest.go
//
// Vkládá audio soubor do databáze jako kapitolu (chapter) v rámci knihy (book).
//
// Pravidlo seskupování (v tomto pořadí priority):
//  1. album tag + author_id  → nejpřesnější identifikace ze samotného souboru
//  2. adresář               → fallback, pokud album tag chybí
//
// books.file_path    = relativní cesta k adresáři od AUDIO_ROOT
// chapters.file_path = relativní cesta k souboru od AUDIO_ROOT
// chapters.position  = track číslo z tagu, nebo pořadové číslo jako fallback

package scanner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"libriter/internal/model"
	"libriter/internal/storage"
)

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

	// Najdi nebo vytvoř autora
	author, err := s.store.GetOrCreateAuthor(ctx, meta.Author)
	if err != nil {
		return fmt.Errorf("get/create author %q: %w", meta.Author, err)
	}

	// Najdi existující knihu: nejdříve podle album tagu, pak podle adresáře
	book, err := s.findOrCreateBook(ctx, relDir, meta, author, duration)
	if err != nil {
		return err
	}

	// Obálka: obrázek v adresáři, jinak obrázek vložený v audio souboru
	s.ensureCover(ctx, book, filepath.Dir(absPath), absPath)

	// Urči pozici kapitoly: z track tagu, nebo jako další v pořadí
	position := meta.TrackNumber
	if position <= 0 {
		count, _, cerr := s.store.GetBookChapterStats(ctx, book.ID)
		if cerr != nil {
			return fmt.Errorf("get chapter stats: %w", cerr)
		}
		position = count + 1
	}

	return s.appendChapter(ctx, relPath, book, meta, position, duration)
}

// findOrCreateBook najde knihu podle album tagu (+ autor) nebo adresáře.
// Pokud neexistuje, vytvoří ji.
func (s *Scanner) findOrCreateBook(
	ctx context.Context,
	relDir string,
	meta *AudioMeta,
	author *model.Author,
	duration int,
) (*model.Book, error) {
	// 1. Pokus: album tag + author_id
	if meta.BookTitle != "" {
		book, err := s.store.GetBookByTitleAndAuthorID(ctx, meta.BookTitle, author.ID)
		if err == nil {
			return book, nil
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return nil, fmt.Errorf("get book by title: %w", err)
		}
	}

	// 2. Pokus: adresář (fallback)
	book, err := s.store.GetBookByDirPath(ctx, relDir)
	if err == nil {
		return book, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, fmt.Errorf("get book by dir: %w", err)
	}

	// Kniha neexistuje – vytvoř ji
	var narratorPtr *string
	if meta.Narrator != "" {
		n := meta.Narrator
		narratorPtr = &n
	}

	title := meta.BookTitle
	if title == "" {
		title = filepath.Base(relDir)
	}

	in := storage.BookInput{
		AuthorID:        author.ID,
		Title:           title,
		Narrator:        narratorPtr,
		DurationSeconds: duration,
		FilePath:        relDir,
		Language:        "cs",
	}

	book, err = s.store.CreateBook(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("create book: %w", err)
	}

	s.log.Info("kniha vytvořena", "title", book.Title, "author", author.Name, "dir", relDir)
	return book, nil
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
