package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// BookEdit jsou úpravy knihy z náhledu, se kterými se importuje.
type BookEdit struct {
	Key            string   `json:"key"`
	Include        bool     `json:"include"`
	Title          string   `json:"title"`
	Authors        []string `json:"authors"`
	Narrator       string   `json:"narrator"`
	Description    string   `json:"description"`
	SeriesTitle    string   `json:"series_title"`
	SeriesPosition *int     `json:"series_position"`
	Language       string   `json:"language"`
}

// maxSeriesPosition odpovídá books.series_position (int16).
const maxSeriesPosition = 32767

// Commit uloží úpravy z náhledu a spustí import vybraných knih na pozadí.
func (s *Service) Commit(id uuid.UUID, edits []BookEdit) (*Session, error) {
	byKey := make(map[string]BookEdit, len(edits))
	for _, e := range edits {
		byKey[e.Key] = e
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	if sess.State != StateReady {
		return nil, ErrWrongState
	}

	books := make([]Book, len(sess.Books))
	selected := 0
	for i, b := range sess.Books {
		if e, ok := byKey[b.Key]; ok {
			if err := applyEdit(&b, e); err != nil {
				return nil, err
			}
		}
		if b.Include {
			selected++
		}
		books[i] = b
	}
	if selected == 0 {
		return nil, ErrNothingChose
	}

	sess.Books = books
	sess.State = StateImporting
	sess.Progress = Progress{Total: selected}
	sess.Results = []Result{}
	snap := snapshot(sess)

	go s.runCommit(s.appCtx, id)
	return snap, nil
}

// applyEdit ověří a přenese úpravy do knihy.
func applyEdit(b *Book, e BookEdit) error {
	b.Include = e.Include
	if !e.Include {
		return nil
	}

	title := strings.TrimSpace(e.Title)
	if title == "" {
		return fmt.Errorf("%w: chybí název (%s)", ErrInvalidBook, b.Key)
	}
	if e.SeriesPosition != nil && (*e.SeriesPosition < 0 || *e.SeriesPosition > maxSeriesPosition) {
		return fmt.Errorf("%w: neplatné pořadí v sérii (%s)", ErrInvalidBook, b.Key)
	}

	b.Title = title
	b.Authors = b.Authors[:0]
	for _, a := range e.Authors {
		if a = strings.TrimSpace(a); a != "" {
			b.Authors = append(b.Authors, a)
		}
	}
	b.Narrator = strings.TrimSpace(e.Narrator)
	b.Description = strings.TrimSpace(e.Description)
	b.SeriesTitle = strings.TrimSpace(e.SeriesTitle)
	b.SeriesPosition = e.SeriesPosition
	b.Language = strings.ToLower(strings.TrimSpace(e.Language))
	return nil
}

func (s *Service) runCommit(ctx context.Context, id uuid.UUID) {
	sess, err := s.Get(id)
	if err != nil {
		return
	}

	series := map[string]uuid.UUID{} // série založené nebo nalezené v tomto importu
	for _, b := range sess.Books {
		if !b.Include {
			continue
		}
		res := Result{Key: b.Key, Title: b.Title}
		book, err := s.importBook(ctx, id, b, series)
		if err != nil {
			res.Error = err.Error()
			s.log.Warn("import knihy selhal", "title", b.Title, "err", err)
		} else {
			res.BookID = &book.ID
		}
		s.update(id, func(sess *Session) {
			sess.Results = append(sess.Results, res)
			sess.Progress.Done++
		})
	}

	s.update(id, func(sess *Session) { sess.State = StateDone })
	// Výsledky zůstávají v relaci; soubory už nejsou potřeba (co se
	// nepřesunulo, patřilo nevybraným nebo nepovedeným knihám).
	_ = os.RemoveAll(s.filesDir(id))
}

// bookLanguage ověří jazyk z náhledu proti číselníku. Prázdný jazyk (tagy ho
// neuvádějí a klient žádný neposlal) dostane výchozí jazyk knihovny;
// kód mimo číselník je chyba – knihu by pak nešlo uložit v editaci.
func (s *Service) bookLanguage(ctx context.Context, language string) (string, error) {
	if language == "" {
		return s.scanner.DefaultLanguage(ctx), nil
	}
	ok, err := s.store.LanguageExists(ctx, language)
	if err != nil {
		return "", fmt.Errorf("jazyk: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("%w: neznámý kód jazyka %q", ErrInvalidBook, language)
	}
	return language, nil
}

func (s *Service) importBook(ctx context.Context, id uuid.UUID, b Book, series map[string]uuid.UUID) (*model.Book, error) {
	names := make([]model.AuthorName, 0, len(b.Authors))
	for _, a := range b.Authors {
		if n := model.ParseAuthorName(a); !n.IsEmpty() {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		names = []model.AuthorName{model.UnknownAuthor}
	}
	authors, err := s.store.GetOrCreateAuthors(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("autoři: %w", err)
	}

	language, err := s.bookLanguage(ctx, b.Language)
	if err != nil {
		return nil, err
	}
	in := storage.BookInput{
		Title:    b.Title,
		Language: language,
	}
	for _, a := range authors {
		in.AuthorIDs = append(in.AuthorIDs, a.ID)
	}
	if b.Narrator != "" {
		n := b.Narrator
		in.Narrator = &n
	}
	if b.Description != "" {
		d := b.Description
		in.Description = &d
	}
	if b.SeriesTitle != "" {
		sid, err := s.seriesID(ctx, b.SeriesTitle, series)
		if err != nil {
			return nil, fmt.Errorf("série: %w", err)
		}
		in.SeriesID = &sid
		if b.SeriesPosition != nil {
			p := int16(*b.SeriesPosition)
			in.SeriesPosition = &p
		}
	}

	target, err := s.targetDir(ctx, names[0], b.Title)
	if err != nil {
		return nil, err
	}
	in.FilePath = target

	bookAbs := filepath.Join(s.filesDir(id), filepath.FromSlash(b.Key))
	req := scanner.ImportBook{Book: in}
	for _, ch := range b.Chapters {
		req.Chapters = append(req.Chapters, scanner.ImportChapter{
			ImportFile: scanner.ImportFile{
				SrcPath: filepath.Join(bookAbs, filepath.FromSlash(ch.Path)),
				RelPath: path.Join(target, ch.Path),
			},
			Title:           ch.Title,
			DurationSeconds: ch.DurationSeconds,
			SizeBytes:       ch.SizeBytes,
		})
	}
	if entries, err := os.ReadDir(bookAbs); err == nil {
		for _, e := range entries {
			if e.IsDir() || scanner.IsAudioFile(e.Name()) || !isAccepted(e.Name()) {
				continue
			}
			req.Extras = append(req.Extras, scanner.ImportFile{
				SrcPath: filepath.Join(bookAbs, e.Name()),
				RelPath: path.Join(target, e.Name()),
			})
		}
	}

	return s.scanner.ImportBook(ctx, req)
}

// seriesID najde sérii podle názvu (bez ohledu na velikost písmen), nebo ji založí.
func (s *Service) seriesID(ctx context.Context, title string, cache map[string]uuid.UUID) (uuid.UUID, error) {
	key := strings.ToLower(title)
	if id, ok := cache[key]; ok {
		return id, nil
	}
	list, err := s.store.ListSeries(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	for _, sr := range list {
		if strings.EqualFold(sr.Title, title) {
			cache[key] = sr.ID
			return sr.ID, nil
		}
	}
	sr, err := s.store.CreateSeries(ctx, title, nil)
	if err != nil {
		return uuid.Nil, err
	}
	cache[key] = sr.ID
	return sr.ID, nil
}

// targetDir vrátí volný adresář knihy „Příjmení, Jméno/Název“ v AUDIO_ROOT.
func (s *Service) targetDir(ctx context.Context, author model.AuthorName, title string) (string, error) {
	authorDir := safeName(author.Last)
	if first := strings.TrimSpace(author.First + " " + author.Middle); first != "" {
		authorDir = safeName(author.Last + ", " + first)
	}
	if authorDir == "" {
		authorDir = "Neznámý autor"
	}
	titleDir := safeName(title)
	if titleDir == "" {
		titleDir = "Kniha"
	}

	root := s.scanner.AudioRoot()
	for n := 1; n < 1000; n++ {
		name := titleDir
		if n > 1 {
			name = fmt.Sprintf("%s (%d)", titleDir, n)
		}
		rel := path.Join(authorDir, name)
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		exists, err := s.store.BookExistsByFilePath(ctx, rel)
		if err != nil {
			return "", err
		}
		if !exists {
			return rel, nil
		}
	}
	return "", errors.New("nelze najít volný název adresáře knihy")
}

// safeName upraví text na název adresáře: bez oddělovačů cest a znaků, které
// Windows/Samba nesnesou, bez teček a mezer na krajích.
func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsControl(r):
			return -1
		case strings.ContainsRune(`/\:*?"<>|`, r):
			return '_'
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, ". ")
	if len(s) > 150 {
		s = strings.TrimSpace(string([]rune(s)[:100]))
	}
	return s
}

// CoverFile vrátí obálku knihy z náhledu: cestu k obrázku ve stagingu, nebo
// data obálky vložené v prvním audio souboru.
func (s *Service) CoverFile(id uuid.UUID, key string) (filePath string, data []byte, mime string, err error) {
	sess, err := s.Get(id)
	if err != nil {
		return "", nil, "", err
	}
	var book *Book
	for i := range sess.Books {
		if sess.Books[i].Key == key {
			book = &sess.Books[i]
			break
		}
	}
	if book == nil || sess.State == StateDone {
		return "", nil, "", ErrNotFound
	}

	filesDir := s.filesDir(id)
	bookAbs := filepath.Join(filesDir, filepath.FromSlash(key))
	var sidecars []string
	if entries, err := os.ReadDir(bookAbs); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				sidecars = append(sidecars, path.Join(key, e.Name()))
			}
		}
	}
	if img := largestImage(filesDir, sidecars); img != "" {
		return filepath.Join(filesDir, filepath.FromSlash(img)), nil, "", nil
	}
	if len(book.Chapters) > 0 {
		if data, mime, ok := scanner.EmbeddedCover(filepath.Join(bookAbs, filepath.FromSlash(book.Chapters[0].Path))); ok {
			return "", data, mime, nil
		}
	}
	return "", nil, "", ErrNotFound
}
