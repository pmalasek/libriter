package service

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

// ErrEmptySession znamená, že by session neměla co přehrávat – prázdná série
// nebo seznam bez jediné existující knihy.
var ErrEmptySession = errors.New("session nemá žádné knihy")

// ErrBookNotInSession znamená, že pozice míří na knihu, která do session nepatří.
var ErrBookNotInSession = errors.New("kniha není v session")

// ErrStalePosition znamená, že zápis nesl starší razítko než pozice uložená
// na serveru. Není to chyba volajícího – jen se nic nepřepsalo.
var ErrStalePosition = errors.New("pozici mezitím přepsal novější zápis")

// Rozsah rychlosti přehrávání; musí odpovídat CHECK v migraci 010.
const (
	minPlaybackSpeed = 0.5
	maxPlaybackSpeed = 3.0
)

// Strop přírůstku deníku poslechu na jeden zápis. Běžně jde o 10 sekund
// poslechu, při trojnásobné rychlosti o 30. Prohlížeč na pozadí škrtí
// časovače na zhruba jeden tik za minutu, což dává 180; víc už je chyba
// klienta a do statistik nemá co dělat.
const maxListenedSecondsPerSave = 600

type PlaySessionService struct {
	store *storage.Store
}

func NewPlaySession(store *storage.Store) *PlaySessionService {
	return &PlaySessionService{store: store}
}

// List vrátí poslechové session uživatele.
func (p *PlaySessionService) List(ctx context.Context, userID uuid.UUID) ([]model.PlaySession, error) {
	return p.store.ListPlaySessions(ctx, userID)
}

// Get vrátí jednu session uživatele.
func (p *PlaySessionService) Get(ctx context.Context, userID, id uuid.UUID) (*model.PlaySession, error) {
	session, err := p.store.GetPlaySession(ctx, userID, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return session, err
}

// StartBook otevře session pro knihu. Pokud ji nějaká rozposlouchaná session
// už obsahuje (samostatně nebo jako díl série), pokračuje se v ní – druhý
// návrat ke knize tak nezaloží duplicitní session s nulovou pozicí.
// created=false znamená, že se pokračuje v existující.
func (p *PlaySessionService) StartBook(ctx context.Context, userID, bookID uuid.UUID) (*model.PlaySession, bool, error) {
	if _, err := p.store.GetBook(ctx, bookID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, false, ErrNotFound
		}
		return nil, false, err
	}

	session, err := p.store.FindPlaySessionWithBook(ctx, userID, bookID)
	switch {
	case err == nil:
		// Poslech pokračuje knihou, na kterou uživatel klikl.
		if session.CurrentBookID != nil && *session.CurrentBookID == bookID {
			return session, false, nil
		}
		updated, err := p.switchToBook(ctx, userID, session, bookID)
		return updated, false, err
	case !errors.Is(err, storage.ErrNotFound):
		return nil, false, err
	}

	created, err := p.store.CreatePlaySession(ctx, storage.PlaySessionInput{
		UserID:   userID,
		Kind:     model.PlaySessionBook,
		SourceID: &bookID,
		BookIDs:  []uuid.UUID{bookID},
	})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, false, ErrNotFound
	}
	return created, true, err
}

// StartSeries otevře session pro celou sérii v pořadí dílů. Rozposlouchaná
// session téže série se místo zakládání nové vrátí k pokračování.
func (p *PlaySessionService) StartSeries(ctx context.Context, userID, seriesID uuid.UUID) (*model.PlaySession, bool, error) {
	if _, err := p.store.GetSeries(ctx, seriesID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, false, ErrNotFound
		}
		return nil, false, err
	}

	session, err := p.store.FindPlaySessionBySource(ctx, userID, model.PlaySessionSeries, seriesID)
	if err == nil {
		return session, false, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return nil, false, err
	}

	bookIDs, err := p.seriesBookIDs(ctx, seriesID)
	if err != nil {
		return nil, false, err
	}
	if len(bookIDs) == 0 {
		return nil, false, ErrEmptySession
	}

	created, err := p.store.CreatePlaySession(ctx, storage.PlaySessionInput{
		UserID:   userID,
		Kind:     model.PlaySessionSeries,
		SourceID: &seriesID,
		BookIDs:  bookIDs,
	})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, false, ErrNotFound
	}
	return created, true, err
}

// StartList založí session z ručně vybraných knih a sérií. Série se rozbalí na
// své díly, duplicitní knihy se vynechají. Seznam vzniká vždy nový – je to
// jednorázový výběr, ne odkaz na něco v knihovně.
func (p *PlaySessionService) StartList(ctx context.Context, userID uuid.UUID, title string, bookIDs, seriesIDs []uuid.UUID) (*model.PlaySession, error) {
	ids, err := p.expandBooks(ctx, bookIDs, seriesIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, ErrEmptySession
	}

	if title == "" {
		title = "Seznam " + time.Now().Format("2. 1. 2006")
	}
	session, err := p.store.CreatePlaySession(ctx, storage.PlaySessionInput{
		UserID:  userID,
		Kind:    model.PlaySessionList,
		Title:   &title,
		BookIDs: ids,
	})
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return session, err
}

// AddItems přidá knihy a série na konec session.
func (p *PlaySessionService) AddItems(ctx context.Context, userID, id uuid.UUID, bookIDs, seriesIDs []uuid.UUID) (*model.PlaySession, error) {
	ids, err := p.expandBooks(ctx, bookIDs, seriesIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, ErrEmptySession
	}

	session, err := p.store.AppendPlaySessionItems(ctx, userID, id, ids)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return session, err
}

// SavePosition uloží rozposlouchané místo session. Volá se každých pár sekund
// poslechu i při každé změně, takže musí být levná; kromě pozice připisuje
// jen odposlouchané sekundy do deníku a stav knihy.
func (p *PlaySessionService) SavePosition(ctx context.Context, userID, id uuid.UUID, in storage.PlaySessionPosition) (*model.PlaySession, error) {
	if in.PositionSeconds < 0 {
		return nil, fmt.Errorf("%w: záporná pozice", ErrInvalidSetting)
	}
	// Nesmyslný přírůstek se ořízne, ne odmítne – pozice se uložit musí.
	if in.ListenedSeconds < 0 {
		in.ListenedSeconds = 0
	}
	if in.ListenedSeconds > maxListenedSecondsPerSave {
		in.ListenedSeconds = maxListenedSecondsPerSave
	}
	if in.PlaybackSpeed < minPlaybackSpeed || in.PlaybackSpeed > maxPlaybackSpeed {
		return nil, fmt.Errorf("%w: rychlost mimo rozsah", ErrInvalidSetting)
	}
	if in.ChapterID != nil {
		chapter, err := p.store.GetChapterByID(ctx, *in.ChapterID)
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		// Kapitola cizí knihy by po přepnutí knihy ukazovala do prázdna.
		if chapter.BookID != in.BookID {
			return nil, fmt.Errorf("%w: kapitola nepatří ke knize", ErrInvalidSetting)
		}
	}

	session, err := p.store.UpdatePlaySessionPosition(ctx, userID, id, in)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return nil, ErrNotFound
	case errors.Is(err, storage.ErrBookNotInSession):
		return nil, ErrBookNotInSession
	case errors.Is(err, storage.ErrStalePosition):
		// Pozici mezitím přepsalo novější místo z jiného zařízení. Poslech se
		// do deníku připsal, volající dostane aktuální stav session.
		return session, ErrStalePosition
	}
	return session, err
}

// Delete smaže session uživatele.
func (p *PlaySessionService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	if err := p.store.DeletePlaySession(ctx, userID, id); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return nil
}

// switchToBook přepne aktuální knihu session, aniž by zahodil její pozici.
// ListenedSeconds ani BookFinished se nenastavují – přepnutí knihy není
// poslech, jen se knize připíše, že je rozposlouchaná.
func (p *PlaySessionService) switchToBook(ctx context.Context, userID uuid.UUID, session *model.PlaySession, bookID uuid.UUID) (*model.PlaySession, error) {
	// PreserveRecordedAt: zapisuje se pozice, která v databázi už je. Čerstvé
	// razítko by z ní udělalo "nejnovější poslech" a dávka čekající v telefonu
	// by celá propadla jako zastaralá.
	in := storage.PlaySessionPosition{
		BookID:             bookID,
		PlaybackSpeed:      session.PlaybackSpeed,
		PreserveRecordedAt: true,
	}
	for _, item := range session.Items {
		if item.BookID == bookID {
			in.ChapterID = item.ChapterID
			in.PositionSeconds = item.PositionSeconds
			break
		}
	}

	updated, err := p.store.UpdatePlaySessionPosition(ctx, userID, session.ID, in)
	switch {
	case errors.Is(err, storage.ErrNotFound), errors.Is(err, storage.ErrBookNotInSession):
		return nil, ErrNotFound
	}
	return updated, err
}

// expandBooks složí seznam knih z přímo vybraných knih a z dílů vybraných
// sérií. Pořadí zadání zůstává, díly série jdou za sebou, duplicity padají.
func (p *PlaySessionService) expandBooks(ctx context.Context, bookIDs, seriesIDs []uuid.UUID) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(bookIDs))
	seen := make(map[uuid.UUID]bool, len(bookIDs))

	add := func(id uuid.UUID) {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	for _, id := range bookIDs {
		if _, err := p.store.GetBook(ctx, id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		add(id)
	}
	for _, id := range seriesIDs {
		if _, err := p.store.GetSeries(ctx, id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		seriesBooks, err := p.seriesBookIDs(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, bookID := range seriesBooks {
			add(bookID)
		}
	}
	return ids, nil
}

// seriesBookIDs vrátí knihy série v pořadí dílů; díl bez pořadí jde na konec.
func (p *PlaySessionService) seriesBookIDs(ctx context.Context, seriesID uuid.UUID) ([]uuid.UUID, error) {
	books, err := p.store.ListBooks(ctx)
	if err != nil {
		return nil, err
	}

	inSeries := make([]model.Book, 0, 8)
	for _, book := range books {
		if book.SeriesID != nil && *book.SeriesID == seriesID {
			inSeries = append(inSeries, book)
		}
	}
	sort.SliceStable(inSeries, func(i, j int) bool {
		return seriesOrder(inSeries[i]) < seriesOrder(inSeries[j])
	})

	ids := make([]uuid.UUID, 0, len(inSeries))
	for _, book := range inSeries {
		ids = append(ids, book.ID)
	}
	return ids, nil
}

func seriesOrder(b model.Book) int {
	if b.SeriesPosition == nil {
		return int(^uint(0) >> 1) // bez pořadí až za poslední díl
	}
	return int(*b.SeriesPosition)
}
