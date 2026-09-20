package service

import (
	"context"
	"errors"

	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// bookRemover je to, co služba potřebuje od scanneru: smazat knihu tak, aby
// jí watcher nestihl soubory ingestovat zpátky. Rozhraní místo konkrétního
// typu drží službu testovatelnou bez souborového systému.
type bookRemover interface {
	DeleteBook(ctx context.Context, id uuid.UUID, deleteFiles bool) (scanner.DeleteResult, error)
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

func (b *BookService) Create(ctx context.Context, in storage.BookInput) (*model.Book, error) {
	return b.store.CreateBook(ctx, in)
}

func (b *BookService) Update(ctx context.Context, id uuid.UUID, in storage.BookInput) (*model.Book, error) {
	book, err := b.store.UpdateBook(ctx, id, in)
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
	author, err := a.store.CreateAuthor(ctx, in)
	if errors.Is(err, storage.ErrConflict) {
		return nil, ErrConflict
	}
	return author, err
}

func (a *AuthorService) Update(ctx context.Context, id uuid.UUID, in storage.AuthorInput) (*model.Author, error) {
	author, err := a.store.UpdateAuthor(ctx, id, in)
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
