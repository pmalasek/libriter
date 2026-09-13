package service

import (
	"context"
	"errors"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

type BookService struct {
	store *storage.Store
}

func NewBook(store *storage.Store) *BookService {
	return &BookService{store: store}
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

func (b *BookService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := b.store.DeleteBook(ctx, id); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	return nil
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
