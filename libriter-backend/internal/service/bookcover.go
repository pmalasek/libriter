package service

import (
	"context"
	"errors"

	"libriter/internal/imagestore"
	"libriter/internal/metadata"
	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// ErrCoverNotAllowed znamená, že adresa obálky nepatří žádnému zapnutému
// zdroji metadat.
var ErrCoverNotAllowed = errors.New("adresa obálky nepatří povolenému zdroji")

// BookCoverService stahuje obálky knih ze zdrojů metadat do COVER_ROOT.
//
// Adresu určuje klient, proto se stahuje jen z hostitelů, které některý
// zapnutý zdroj prohlásí za své (metadata.CoverProvider) – jinak by šlo
// server přimět sáhnout kamkoliv (SSRF).
type BookCoverService struct {
	store   *storage.Store
	chain   ChainSource
	fetcher *metadata.Fetcher
	root    string
}

func NewBookCover(store *storage.Store, chain ChainSource, root string) *BookCoverService {
	return &BookCoverService{
		store:   store,
		chain:   chain,
		fetcher: metadata.NewFetcher(0),
		root:    root,
	}
}

// SetFromURL stáhne obrázek a uloží ho jako obálku knihy. Dosavadní obálku
// nahradí – o tom, jestli se má měnit, rozhoduje klient.
func (s *BookCoverService) SetFromURL(ctx context.Context, bookID uuid.UUID, coverURL string) (*model.Book, error) {
	if !s.chain.Chain().SupportsCoverURL(coverURL) {
		return nil, ErrCoverNotAllowed
	}

	book, err := s.store.GetBook(ctx, bookID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	data, ext, err := downloadImage(ctx, s.fetcher, coverURL)
	if err != nil {
		return nil, err
	}

	// Jméno souboru je stejné jako u obálek, které ukládá scanner
	// (<book_id>.<ext>), takže se obálka z adresáře a ze zdroje nemíchají.
	fileName, err := imagestore.Write(s.root, bookID.String(), data, ext)
	if err != nil {
		return nil, err
	}
	// Starý soubor s jinou příponou by jinak zůstal ležet na disku.
	if book.CoverPath != nil && *book.CoverPath != fileName {
		_ = imagestore.Remove(s.root, *book.CoverPath)
	}

	if err := s.store.UpdateBookCoverPath(ctx, bookID, fileName); err != nil {
		return nil, err
	}
	return s.store.GetBook(ctx, bookID)
}
