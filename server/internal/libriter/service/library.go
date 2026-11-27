package service

import (
	"context"
	"errors"
	"path"
	"strings"

	"mediater/internal/libriter/audiostore"
	"mediater/internal/libriter/model"
	"mediater/internal/libriter/scanner"
	"mediater/internal/libriter/storage"

	"github.com/google/uuid"
)

// bookRemover je to, co služba potřebuje od scanneru: smazat knihu tak, aby
// jí watcher nestihl soubory ingestovat zpátky. Rozhraní místo konkrétního
// typu drží službu testovatelnou bez souborového systému.
type bookRemover interface {
	DeleteBook(ctx context.Context, id uuid.UUID, deleteFiles bool) (scanner.DeleteResult, error)
	// DefaultLanguage je jazyk nové knihy, když ho klient neposlal.
	DefaultLanguage(ctx context.Context) string
	// AudioRoot je kořen knihovny; file_path nové knihy musí ležet v něm.
	AudioRoot() string
}

type BookService struct {
	store   *storage.Store
	scanner bookRemover
}

func NewBook(store *storage.Store, scanner bookRemover) *BookService {
	return &BookService{store: store, scanner: scanner}
}

func (b *BookService) List(ctx context.Context) ([]model.Book, error) {
	return b.store.ListBooks(ctx)
}

func (b *BookService) GetByID(ctx context.Context, id uuid.UUID) (*model.Book, error) {
	book, err := b.store.GetBook(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return book, err
}

// ErrInvalidFilePath znamená, že file_path nové knihy neleží v AUDIO_ROOT.
var ErrInvalidFilePath = errors.New("file_path musí být relativní cesta uvnitř knihovny")

// Create založí knihu z API. file_path musí být relativní cesta uvnitř
// AUDIO_ROOT (ověří se i přes symbolické odkazy) a cover_path se nepřebírá:
// obálku ukládá jen scanner nebo služba obálek, jinak by editor mohl knihu
// navázat na libovolný soubor a ten by se při mazání knihy smazal.
func (b *BookService) Create(ctx context.Context, in storage.BookInput) (*model.Book, error) {
	clean, ok := b.cleanFilePath(in.FilePath)
	if !ok {
		return nil, ErrInvalidFilePath
	}
	in.FilePath = clean
	in.CoverPath = nil
	return b.store.CreateBook(ctx, in)
}

// cleanFilePath ověří adresář knihy relativní k AUDIO_ROOT a vrátí ho
// v normalizovaném tvaru (lomítka, bez „./“).
func (b *BookService) cleanFilePath(rel string) (string, bool) {
	rel = path.Clean(strings.TrimSpace(rel))
	if rel == "." || rel == "/" {
		return "", false
	}
	if _, ok := audiostore.Resolve(b.scanner.AudioRoot(), rel); !ok {
		return "", false
	}
	return rel, true
}

// Update nahradí pole knihy, která spravuje editor. Interní cesty
// (file_path, cover_path) a album tag zůstanou uložené – spravuje je scanner
// a služba obálek a podle nich se mažou soubory; z API se nepřebírají.
func (b *BookService) Update(ctx context.Context, id uuid.UUID, in storage.BookInput) (*model.Book, error) {
	book, err := b.store.PatchBook(ctx, id, func(current *storage.BookInput) {
		in.FilePath, in.CoverPath, in.AlbumTag = current.FilePath, current.CoverPath, current.AlbumTag
		*current = in
	})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return book, err
}

// Patch upraví jen pole, která apply skutečně nastaví; zbytek zůstane beze změny.
func (b *BookService) Patch(ctx context.Context, id uuid.UUID, apply func(*storage.BookInput)) (*model.Book, error) {
	book, err := b.store.PatchBook(ctx, id, apply)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return book, err
}

// Delete smaže knihu. Při deleteFiles zmizí i její audio soubory a obálka –
// bez toho scanner knihu ze zbylých souborů při dalším průchodu založí znovu.
func (b *BookService) Delete(
	ctx context.Context, id uuid.UUID, deleteFiles bool,
) (scanner.DeleteResult, error) {
	result, err := b.scanner.DeleteBook(ctx, id, deleteFiles)
	if errors.Is(err, storage.ErrNotFound) {
		return result, ErrNotFound
	}
	return result, err
}

// ErrUnknownLanguage znamená, že kód jazyka knihy není v číselníku.
var ErrUnknownLanguage = errors.New("neznámý kód jazyka")

// DefaultLanguage vrátí výchozí jazyk nových knih (nastavení knihovny).
func (b *BookService) DefaultLanguage(ctx context.Context) string {
	return b.scanner.DefaultLanguage(ctx)
}

// Languages vrátí číselník jazyků, ze kterého se jazyk knihy vybírá.
func (b *BookService) Languages(ctx context.Context) ([]model.Language, error) {
	return b.store.ListLanguages(ctx)
}

// CheckLanguage ověří kód jazyka proti číselníku; neznámý vrátí jako
// ErrUnknownLanguage.
func (b *BookService) CheckLanguage(ctx context.Context, code string) error {
	ok, err := b.store.LanguageExists(ctx, code)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUnknownLanguage
	}
	return nil
}

// ErrChapterSetMismatch znamená, že seznam kapitol k seřazení neodpovídá
// kapitolám knihy – typicky scanner mezitím přidal soubor.
var ErrChapterSetMismatch = errors.New("seznam kapitol neodpovídá knize")

// Chapter vrátí jednu kapitolu i s cestou k audio souboru (ta se do JSONu
// neserializuje, slouží streamování).
func (b *BookService) Chapter(ctx context.Context, id uuid.UUID) (*model.Chapter, error) {
	chapter, err := b.store.GetChapterByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return chapter, err
}

// Chapters vrátí kapitoly knihy v pořadí přehrávání.
func (b *BookService) Chapters(ctx context.Context, id uuid.UUID) ([]model.Chapter, error) {
	if _, err := b.store.GetBook(ctx, id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return b.store.GetChaptersByBookID(ctx, id)
}

// ReorderChapters nastaví pořadí kapitol podle seznamu ID (všechny kapitoly
// knihy, každá právě jednou) a vrátí je v novém pořadí.
func (b *BookService) ReorderChapters(ctx context.Context, id uuid.UUID, chapterIDs []uuid.UUID) ([]model.Chapter, error) {
	chapters, err := b.store.ReorderChapters(ctx, id, chapterIDs)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return nil, ErrNotFound
	case errors.Is(err, storage.ErrChapterSetMismatch):
		return nil, ErrChapterSetMismatch
	}
	return chapters, err
}

// --- Authors ---

type AuthorService struct {
	store *storage.Store
}

func NewAuthor(store *storage.Store) *AuthorService {
	return &AuthorService{store: store}
}

func (a *AuthorService) List(ctx context.Context) ([]model.Author, error) {
	return a.store.ListAuthors(ctx)
}

func (a *AuthorService) GetByID(ctx context.Context, id uuid.UUID) (*model.Author, error) {
	author, err := a.store.GetAuthor(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return author, err
}

func (a *AuthorService) Create(ctx context.Context, in storage.AuthorInput) (*model.Author, error) {
	// Fotku nahrává jen AuthorImageService; cesta z API se nepřebírá.
	in.ImagePath = nil
	author, err := a.store.CreateAuthor(ctx, in)
	if errors.Is(err, storage.ErrConflict) {
		return nil, ErrConflict
	}
	return author, err
}

func (a *AuthorService) Update(ctx context.Context, id uuid.UUID, in storage.AuthorInput) (*model.Author, error) {
	// image_path spravuje jen AuthorImageService (a podle něj maže soubory);
	// hodnota z API se nepřebírá.
	author, err := a.store.PatchAuthor(ctx, id, func(current *storage.AuthorInput) {
		in.ImagePath = current.ImagePath
		*current = in
	})
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return nil, ErrNotFound
	case errors.Is(err, storage.ErrConflict):
		return nil, ErrConflict
	}
	return author, err
}

func (a *AuthorService) Delete(ctx context.Context, id uuid.UUID) error {
	err := a.store.DeleteAuthor(ctx, id)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, storage.ErrConflict):
		return ErrConflict
	}
	return err
}

// --- Series ---

type SeriesService struct {
	store *storage.Store
}

func NewSeries(store *storage.Store) *SeriesService {
	return &SeriesService{store: store}
}

func (s *SeriesService) List(ctx context.Context) ([]model.Series, error) {
	return s.store.ListSeries(ctx)
}

func (s *SeriesService) GetByID(ctx context.Context, id uuid.UUID) (*model.Series, error) {
	sr, err := s.store.GetSeries(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return sr, err
}

func (s *SeriesService) Create(ctx context.Context, title string, description *string) (*model.Series, error) {
	return s.store.CreateSeries(ctx, title, description)
}

func (s *SeriesService) Update(ctx context.Context, id uuid.UUID, title string, description *string) (*model.Series, error) {
	sr, err := s.store.UpdateSeries(ctx, id, title, description)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return sr, err
}

func (s *SeriesService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.store.DeleteSeries(ctx, id); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	return nil
}
