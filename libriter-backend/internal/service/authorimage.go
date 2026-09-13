package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"libriter/internal/imagestore"
	"libriter/internal/metadata"
	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// ErrImageNotAllowed znamená, že adresa obrázku nepatří žádnému zapnutému
// zdroji metadat.
var ErrImageNotAllowed = errors.New("adresa obrázku nepatří povolenému zdroji")

// AuthorImageService stahuje fotky autorů z povolených zdrojů na disk.
//
// Adresu určuje klient, proto se stahuje jen z hostitelů, které některý
// zapnutý zdroj metadat prohlásí za svoje – jinak by šlo server přimět
// sáhnout kamkoliv (SSRF).
type AuthorImageService struct {
	store    *storage.Store
	chain    *metadata.Chain
	fetcher  *metadata.Fetcher
	root     string
	maxBytes int
}

func NewAuthorImage(store *storage.Store, chain *metadata.Chain, root string) *AuthorImageService {
	return &AuthorImageService{
		store:    store,
		chain:    chain,
		fetcher:  metadata.NewFetcher(0),
		root:     root,
		maxBytes: imagestore.MaxBytes,
	}
}

// SetFromURL stáhne obrázek a uloží ho jako fotku autora.
func (s *AuthorImageService) SetFromURL(ctx context.Context, authorID uuid.UUID, imageURL string) (*model.Author, error) {
	if !s.chain.SupportsImageURL(imageURL) {
		return nil, ErrImageNotAllowed
	}

	author, err := s.store.GetAuthor(ctx, authorID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	data, ext, err := s.download(ctx, imageURL)
	if err != nil {
		return nil, err
	}

	fileName, err := imagestore.Write(s.root, authorID.String(), data, ext)
	if err != nil {
		return nil, err
	}

	// Starý soubor s jinou příponou by jinak zůstal ležet na disku.
	if author.ImagePath != nil && *author.ImagePath != fileName {
		_ = imagestore.Remove(s.root, *author.ImagePath)
	}

	return s.setImagePath(ctx, authorID, &fileName)
}

// Clear smaže fotku autora ze souborů i z databáze.
func (s *AuthorImageService) Clear(ctx context.Context, authorID uuid.UUID) (*model.Author, error) {
	author, err := s.store.GetAuthor(ctx, authorID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if author.ImagePath != nil {
		_ = imagestore.Remove(s.root, *author.ImagePath)
	}
	return s.setImagePath(ctx, authorID, nil)
}

// setImagePath přepíše jen cestu k obrázku; ostatní pole autora zůstanou.
func (s *AuthorImageService) setImagePath(ctx context.Context, authorID uuid.UUID, path *string) (*model.Author, error) {
	author, err := s.store.PatchAuthor(ctx, authorID, func(in *storage.AuthorInput) {
		in.ImagePath = path
	})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return author, err
}

// download stáhne obrázek a ověří, že obrázek skutečně je.
func (s *AuthorImageService) download(ctx context.Context, imageURL string) ([]byte, string, error) {
	body, contentType, err := s.fetcher.GetWithType(ctx, imageURL, "image/*")
	if err != nil {
		return nil, "", fmt.Errorf("stažení obrázku: %w", err)
	}
	defer body.Close()

	ext := imagestore.ExtForContentType(contentType)
	if ext == "" {
		// Některé servery typ neposílají správně – zkusíme příponu z adresy.
		ext = filepath.Ext(imageURL)
		if !imagestore.IsImageExt(ext) {
			return nil, "", fmt.Errorf("adresa nevrátila obrázek (Content-Type %q)", contentType)
		}
	}

	data, err := imagestore.ReadLimited(body)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", errors.New("stažený obrázek je prázdný")
	}
	return data, ext, nil
}
