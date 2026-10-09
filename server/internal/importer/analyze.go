package importer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"libriter/internal/imagestore"
	"libriter/internal/model"
	"libriter/internal/scanner"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

// Kódy varování u knihy v náhledu.
const (
	WarnNoTitle    = "no_title"
	WarnNoAuthor   = "no_author"
	WarnNoDuration = "no_duration"
	WarnSimilar    = "similar_exists"
)

// discDir poznává podadresáře s disky jedné knihy (CD1, Disc 2, …).
var discDir = regexp.MustCompile(`(?i)^(cd|disc|disk)[\s_-]*\d+$`)

// numberedDir rozdělí „01 - V kleci“ na pořadí v sérii a název.
var numberedDir = regexp.MustCompile(`^(\d{1,3})\s*[-–—._)]\s*(.+)$`)

// adLine jsou reklamní odstavce na konci popisu v bookinfo.html.
var adLine = regexp.MustCompile(`(?i)^knižní předloha`)

// Analyze uzavře nahrávání a spustí rozpoznání knih na pozadí.
func (s *Service) Analyze(id uuid.UUID) (*Session, error) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	if sess.State != StateUploading {
		s.mu.Unlock()
		return nil, ErrWrongState
	}
	sess.State = StateAnalyzing
	snap := snapshot(sess)
	s.mu.Unlock()

	go s.runAnalyze(s.appCtx, id)
	return snap, nil
}

func (s *Service) runAnalyze(ctx context.Context, id uuid.UUID) {
	books, skipped, err := s.analyze(ctx, id)
	s.update(id, func(sess *Session) {
		sess.Skipped = skipped
		if err != nil {
			sess.State = StateFailed
			sess.Error = err.Error()
			return
		}
		sess.Books = books
		sess.State = StateReady
	})
	if err != nil {
		// Z nepovedené analýzy se nic importovat nedá – soubory jen zabírají místo.
		_ = os.RemoveAll(s.filesDir(id))
		s.log.Warn("analýza importu selhala", "id", id, "err", err)
	}
}

func (s *Service) analyze(ctx context.Context, id uuid.UUID) ([]Book, []string, error) {
	filesDir := s.filesDir(id)

	if err := s.extractZips(filesDir); err != nil {
		return nil, nil, err
	}

	drafts, skipped, err := detectBooks(filesDir)
	if err != nil {
		return nil, skipped, err
	}
	if len(drafts) == 0 {
		return nil, skipped, ErrNoBooks
	}

	total := 0
	for _, d := range drafts {
		total += len(d.audio)
	}
	s.update(id, func(sess *Session) { sess.Progress = Progress{Total: total} })

	books := make([]Book, 0, len(drafts))
	for _, d := range drafts {
		if ctx.Err() != nil {
			return nil, skipped, ctx.Err()
		}
		b := describeBook(filesDir, d, func() {
			s.update(id, func(sess *Session) { sess.Progress.Done++ })
		})
		if s.scanner != nil && b.Title != "" {
			if similar, err := s.scanner.FindSimilarBook(ctx, b.Title); err == nil && similar != nil {
				b.Warnings = append(b.Warnings, WarnSimilar)
				b.SimilarTitle = similar.Title
			}
		}
		books = append(books, b)
	}
	return books, skipped, nil
}

// extractZips rozbalí všechny nahrané archivy. Rozbalený obsah má vlastní
// limit velikosti, stejný jako upload.
func (s *Service) extractZips(filesDir string) error {
	var zips []string
	err := filepath.WalkDir(filesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".zip") {
			rel, _ := filepath.Rel(filesDir, p)
			zips = append(zips, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return err
	}
	sort.Strings(zips)

	var written int64
	for _, z := range zips {
		n, err := extractZip(filesDir, z, s.maxBytes-written)
		written += n
		if err != nil {
			if errors.Is(err, ErrTooLarge) {
				return fmt.Errorf("%s: %w", path.Base(z), ErrTooLarge)
			}
			return err
		}
	}
	return nil
}

// draft je kniha rozpoznaná podle adresářů, ještě bez metadat.
type draft struct {
	key      string   // adresář knihy relativně k filesDir ("." = kořen)
	audio    []string // audio soubory relativně k filesDir
	sidecars []string // obrázky, bookinfo.html, .pls přímo v adresáři knihy
}

// detectBooks rozdělí nahrané soubory do knih: kniha je každý adresář
// s audio soubory; podadresáře s disky (CD1, CD2) patří k nadřazené knize.
func detectBooks(filesDir string) ([]draft, []string, error) {
	byKey := map[string]*draft{}
	var others []string

	err := filepath.WalkDir(filesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(filesDir, p)
		rel = filepath.ToSlash(rel)
		if !scanner.IsAudioFile(rel) {
			others = append(others, rel)
			return nil
		}
		key := bookKey(path.Dir(rel))
		dr := byKey[key]
		if dr == nil {
			dr = &draft{key: key}
			byKey[key] = dr
		}
		dr.audio = append(dr.audio, rel)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	var skipped []string
	for _, rel := range others {
		if dr := byKey[path.Dir(rel)]; dr != nil {
			dr.sidecars = append(dr.sidecars, rel)
		} else {
			skipped = append(skipped, rel)
		}
	}

	drafts := make([]draft, 0, len(byKey))
	for _, dr := range byKey {
		drafts = append(drafts, *dr)
	}
	sort.Slice(drafts, func(i, j int) bool { return scanner.NaturalLess(drafts[i].key, drafts[j].key) })
	if skipped == nil {
		skipped = []string{}
	}
	return drafts, skipped, nil
}

// bookKey vrátí adresář knihy pro adresář s audio soubory.
func bookKey(dir string) string {
	if dir != "." && discDir.MatchString(path.Base(dir)) {
		return path.Dir(dir)
	}
	return dir
}

// fileMeta jsou údaje o jednom audio souboru knihy.
type fileMeta struct {
	rel      string // relativně k adresáři knihy
	tags     *scanner.AudioMeta
	duration int
	size     int64
}

// describeBook sestaví náhled knihy. Priorita zdrojů: bookinfo.html > tagy >
// názvy adresářů.
func describeBook(filesDir string, d draft, tick func()) Book {
	bookAbs := filepath.Join(filesDir, filepath.FromSlash(d.key))
	b := Book{
		Key:      d.key,
		Include:  true,
		Authors:  []string{},
		Warnings: []string{},
	}

	info, playlist := readSidecars(filesDir, d.sidecars)

	files := make([]fileMeta, 0, len(d.audio))
	missingDuration := false
	for _, rel := range d.audio {
		abs := filepath.Join(filesDir, filepath.FromSlash(rel))
		fm := fileMeta{rel: relTo(d.key, rel)}
		if tags, err := scanner.ReadTagMeta(abs); err == nil {
			fm.tags = tags
		} else {
			fm.tags = &scanner.AudioMeta{}
		}
		fm.duration = scanner.ProbeDuration(abs)
		if fm.duration <= 0 {
			missingDuration = true
		}
		if fi, err := os.Stat(abs); err == nil {
			fm.size = fi.Size()
		}
		files = append(files, fm)
		tick()
	}

	listed := playlist
	if info != nil && len(info.Chapters) > 0 {
		listed = info.Chapters
	}
	orderFiles(files, listed)

	titles := map[string]string{}
	for _, ch := range listed {
		if ch.Title != "" {
			titles[fileKey(ch.File)] = ch.Title
		}
	}
	for _, fm := range files {
		title := titles[fileKey(path.Base(fm.rel))]
		if title == "" {
			title = fm.tags.ChapterTitle
		}
		if title == "" {
			base := path.Base(fm.rel)
			title = strings.TrimSuffix(base, path.Ext(base))
		}
		title = strings.Join(strings.Fields(title), " ")
		b.Chapters = append(b.Chapters, Chapter{
			Path: fm.rel, Title: title, DurationSeconds: fm.duration, SizeBytes: fm.size,
		})
		b.DurationSeconds += fm.duration
		b.SizeBytes += fm.size
	}

	// Adresáře: „Autor/01 - Název“ → skupina (autor), pořadí v sérii, název.
	var folderTitle string
	if d.key != "." {
		folderTitle = path.Base(d.key)
		if m := numberedDir.FindStringSubmatch(folderTitle); m != nil {
			pos, _ := strconv.Atoi(m[1])
			b.SeriesPosition = &pos
			folderTitle = strings.TrimSpace(m[2])
		}
		if parent := path.Dir(d.key); parent != "." {
			b.Group = path.Base(parent)
		}
	}

	tagAlbum, tagAuthors, tagNarrator, tagLanguage := commonTags(files)
	b.Language = tagLanguage

	// Název
	switch {
	case info != nil && info.Title != "":
		b.Title = info.Title
	case tagAlbum != "":
		b.Title = scanner.CleanAlbumTitle(tagAlbum)
	default:
		b.Title = folderTitle
	}

	// Autoři
	var authors []model.AuthorName
	switch {
	case info != nil && info.Author != "":
		authors = model.ParseAuthorNames(info.Author)
	case len(tagAuthors) > 0:
		authors = preferFolderForm(tagAuthors, model.ParseAuthorNames(b.Group))
	case b.Group != "":
		authors = model.ParseAuthorNames(b.Group)
	}
	for _, a := range authors {
		b.Authors = append(b.Authors, a.Full())
	}

	// Interpret
	if info != nil && info.Narrator != "" {
		b.Narrator = info.Narrator
	} else {
		b.Narrator = tagNarrator
	}

	if info != nil {
		b.Publisher = info.Publisher
		b.Description = cleanDescription(info.Description)
	}

	b.HasCover = largestImage(filesDir, d.sidecars) != "" ||
		(len(files) > 0 && scanner.HasEmbeddedCover(filepath.Join(bookAbs, filepath.FromSlash(files[0].rel))))

	if b.Title == "" {
		b.Warnings = append(b.Warnings, WarnNoTitle)
	}
	if len(b.Authors) == 0 {
		b.Warnings = append(b.Warnings, WarnNoAuthor)
	}
	if missingDuration {
		b.Warnings = append(b.Warnings, WarnNoDuration)
	}
	return b
}

// readSidecars načte bookinfo.html a playlist.pls z adresáře knihy.
func readSidecars(filesDir string, sidecars []string) (*bookInfo, []infoChapter) {
	var info *bookInfo
	var playlist []infoChapter

	// bookinfo.html má přednost před jinými HTML soubory.
	sorted := append([]string{}, sidecars...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return strings.EqualFold(path.Base(sorted[i]), "bookinfo.html") &&
			!strings.EqualFold(path.Base(sorted[j]), "bookinfo.html")
	})

	for _, rel := range sorted {
		abs := filepath.Join(filesDir, filepath.FromSlash(rel))
		switch strings.ToLower(path.Ext(rel)) {
		case ".html", ".htm":
			if info != nil {
				continue
			}
			if parsed, err := parseBookInfo(abs); err == nil && (parsed.Title != "" || parsed.Author != "") {
				info = parsed
			}
		case ".pls":
			if playlist != nil {
				continue
			}
			if parsed, err := parsePLS(abs); err == nil && len(parsed) > 0 {
				playlist = parsed
			}
		}
	}
	return info, playlist
}

// orderFiles seřadí soubory: podle seznamu z bookinfo/playlistu, když pokrývá
// všechny soubory; jinak podle disku a tracku z tagů, když je mají všechny;
// jinak přirozeně podle cesty.
func orderFiles(files []fileMeta, listed []infoChapter) {
	if pos, ok := listedPositions(files, listed); ok {
		sort.SliceStable(files, func(i, j int) bool {
			return pos[fileKey(path.Base(files[i].rel))] < pos[fileKey(path.Base(files[j].rel))]
		})
		return
	}

	byTags := true
	seen := map[[2]int]bool{}
	for _, f := range files {
		k := [2]int{f.tags.DiscNumber, f.tags.TrackNumber}
		if f.tags.TrackNumber <= 0 || seen[k] {
			byTags = false
			break
		}
		seen[k] = true
	}
	if byTags {
		sort.SliceStable(files, func(i, j int) bool {
			a, b := files[i].tags, files[j].tags
			if a.DiscNumber != b.DiscNumber {
				return a.DiscNumber < b.DiscNumber
			}
			return a.TrackNumber < b.TrackNumber
		})
		return
	}

	sort.SliceStable(files, func(i, j int) bool { return scanner.NaturalLess(files[i].rel, files[j].rel) })
}

// listedPositions vrátí pořadí souborů podle seznamu z bookinfo/playlistu.
// Vrací false, když seznam některý soubor nezná nebo se názvy opakují.
func listedPositions(files []fileMeta, listed []infoChapter) (map[string]int, bool) {
	if len(listed) == 0 {
		return nil, false
	}
	pos := map[string]int{}
	for i, ch := range listed {
		k := fileKey(ch.File)
		if _, dup := pos[k]; dup {
			return nil, false
		}
		pos[k] = i
	}
	seen := map[string]bool{}
	for _, f := range files {
		k := fileKey(path.Base(f.rel))
		if _, ok := pos[k]; !ok || seen[k] {
			return nil, false
		}
		seen[k] = true
	}
	return pos, true
}

func fileKey(name string) string {
	return strings.ToLower(norm.NFC.String(strings.TrimSpace(name)))
}

// commonTags vrátí nejčastější album tag a jazyk a první nalezené autory
// a interpreta.
func commonTags(files []fileMeta) (album string, authors []model.AuthorName, narrator, language string) {
	counts := map[string]int{}
	langCounts := map[string]int{}
	for _, f := range files {
		if f.tags.Language != "" {
			langCounts[f.tags.Language]++
		}
		if f.tags.BookTitle != "" {
			counts[f.tags.BookTitle]++
		}
		if len(authors) == 0 && len(f.tags.Authors) > 0 {
			authors = f.tags.Authors
		}
		if narrator == "" {
			narrator = f.tags.Narrator
		}
	}
	return mostCommon(counts), authors, narrator, mostCommon(langCounts)
}

// mostCommon vrátí nejčastější hodnotu; při shodě abecedně první, ať je
// výsledek stálý.
func mostCommon(counts map[string]int) string {
	var out string
	best := 0
	for v, n := range counts {
		if n > best || (n == best && v < out) {
			out, best = v, n
		}
	}
	return out
}

// cleanDescription odstraní reklamní odstavce („Knižní předloha … k zakoupení“).
func cleanDescription(s string) string {
	paras := strings.Split(s, "\n")
	out := paras[:0]
	for _, p := range paras {
		if adLine.MatchString(strings.TrimSpace(p)) {
			continue
		}
		out = append(out, p)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// largestImage vrátí největší obrázek mezi doprovodnými soubory ("" = žádný).
func largestImage(filesDir string, sidecars []string) string {
	var best string
	var bestSize int64
	for _, rel := range sidecars {
		if !imagestore.IsImageExt(path.Ext(rel)) {
			continue
		}
		fi, err := os.Stat(filepath.Join(filesDir, filepath.FromSlash(rel)))
		if err != nil || fi.Size() == 0 || fi.Size() > imagestore.MaxBytes {
			continue
		}
		if fi.Size() > bestSize {
			best, bestSize = rel, fi.Size()
		}
	}
	return best
}

// relTo vrátí cestu rel relativně k adresáři knihy key.
func relTo(key, rel string) string {
	if key == "." {
		return rel
	}
	return strings.TrimPrefix(rel, key+"/")
}

// preferFolderForm nahradí autora z tagu jménem ze složky, když jde o tatáž
// slova. Tag „Amis Kingsley“ nejde rozdělit na jméno a příjmení, složka
// „Amis, Kingsley“ ano.
func preferFolderForm(tagAuthors, folderAuthors []model.AuthorName) []model.AuthorName {
	if len(folderAuthors) == 0 {
		return tagAuthors
	}
	out := make([]model.AuthorName, len(tagAuthors))
	for i, a := range tagAuthors {
		out[i] = a
		for _, f := range folderAuthors {
			if sameWords(a.Full(), f.Full()) {
				out[i] = f
				break
			}
		}
	}
	return out
}

// sameWords porovná jména bez ohledu na pořadí slov a velikost písmen.
func sameWords(a, b string) bool {
	wa, wb := strings.Fields(strings.ToLower(a)), strings.Fields(strings.ToLower(b))
	if len(wa) != len(wb) {
		return false
	}
	sort.Strings(wa)
	sort.Strings(wb)
	return strings.Join(wa, " ") == strings.Join(wb, " ")
}
