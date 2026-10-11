package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"mediater/internal/core/imagestore"
	"mediater/internal/sceniter/storage"
)

// ErrNotImage: nahraná data nejsou podporovaný obrázek.
var ErrNotImage = errors.New("soubor není podporovaný obrázek")

// providerExts jsou přípony, pod kterými zdroj metadat ukládá obrázky
// (imagestore.ExtForContentType) – pro hledání původního souboru na disku.
var providerExts = []string{".jpg", ".png", ".webp", ".gif", ".bmp"}

// TitleImages spravuje plakát a pozadí nahrané editorem. Ruční obrázek leží
// pod vlastním názvem (poster-manual-movie-<id>.jpg), takže obrázek ze zdroje
// (poster-movie-<id>.jpg) zůstane na disku a po odebrání ručního se vrátí.
type TitleImages struct {
	store *storage.Store
	root  string
}

func NewTitleImages(store *storage.Store, imageRoot string) *TitleImages {
	return &TitleImages{store: store, root: imageRoot}
}

func manualName(image storage.ImageKind, kind string, id uuid.UUID) string {
	return fmt.Sprintf("%s-manual-%s-%s", image, kind, id)
}

func providerName(image storage.ImageKind, kind string, id uuid.UUID) string {
	return fmt.Sprintf("%s-%s-%s", image, kind, id)
}

// Upload uloží nahraný obrázek a označí ho jako ruční. Typ se pozná podle
// obsahu, ne podle hlaviček klienta.
func (t *TitleImages) Upload(ctx context.Context, kind string, id uuid.UUID, image storage.ImageKind, data []byte) error {
	ext := imagestore.SniffExt(data)
	if ext == "" {
		return ErrNotImage
	}
	poster, backdrop, err := t.store.GetTitleImages(ctx, kind, id)
	if err != nil {
		return err
	}
	current := poster
	if image == storage.ImageBackdrop {
		current = backdrop
	}

	fileName, err := imagestore.Write(t.root, manualName(image, kind, id), data, ext)
	if err != nil {
		return err
	}
	// Dřívější ruční soubor s jinou příponou by jinak zůstal ležet na disku.
	if current.Source == "manual" && current.Path != nil && *current.Path != fileName {
		_ = imagestore.Remove(t.root, *current.Path)
	}
	return t.store.SetTitleImage(ctx, kind, id, image, &fileName, true)
}

// Remove smaže ruční obrázek a vrátí obrázek ze zdroje metadat, pokud na
// disku zůstal; jinak titul zůstane bez obrázku do příští obnovy metadat.
func (t *TitleImages) Remove(ctx context.Context, kind string, id uuid.UUID, image storage.ImageKind) error {
	poster, backdrop, err := t.store.GetTitleImages(ctx, kind, id)
	if err != nil {
		return err
	}
	current := poster
	if image == storage.ImageBackdrop {
		current = backdrop
	}
	if current.Source != "manual" {
		return nil
	}
	if current.Path != nil {
		_ = imagestore.Remove(t.root, *current.Path)
	}

	for _, ext := range providerExts {
		name := providerName(image, kind, id) + ext
		if imagestore.Exists(t.root, name) {
			return t.store.SetTitleImage(ctx, kind, id, image, &name, false)
		}
	}
	return t.store.SetTitleImage(ctx, kind, id, image, nil, false)
}
