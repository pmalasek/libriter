package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
)

// TrashDir je složka koše v každé knihovně. Začíná tečkou, takže ji průchod
// i sledování změn přeskakují (Ignored) a soubor v ní jde ručně vrátit.
const TrashDir = ".trash"

// ErrLastFile znamená, že soubor je jediný soubor filmu – ten se z aplikace
// nemaže, aby nešlo omylem smazat celý film.
var ErrLastFile = errors.New("jediný soubor filmu nelze smazat")

// TrashFile přesune soubor (a jeho titulky) do koše knihovny – podsložky
// .trash se zachovanou cestou – a odebere ho z knihovny. Film nebo díl bez
// souborů zůstane kvůli historii sledování, ve výpisech se nezobrazuje.
// Vrací cestu v koši (relativní k VIDEO_ROOT).
func (s *Scanner) TrashFile(ctx context.Context, mediaID uuid.UUID) (string, error) {
	s.ingest.Lock()
	defer s.ingest.Unlock()

	rec, err := s.store.MediaByID(ctx, mediaID)
	if err != nil {
		return "", err
	}
	if rec.MovieID != nil {
		paths, err := s.store.MovieFilePaths(ctx, *rec.MovieID)
		if err != nil {
			return "", err
		}
		if len(paths) < 2 {
			return "", ErrLastFile
		}
	}
	top, rest, ok := strings.Cut(rec.FilePath, "/")
	if !ok {
		return "", ErrRenumberInvalid
	}
	dest := path.Join(top, TrashDir, rest)
	dest = s.freeName(dest)
	if err := os.MkdirAll(filepath.Dir(s.abs(dest)), 0o755); err != nil {
		return "", fmt.Errorf("koš: %w", err)
	}
	// Disk napřed: kdyby přesun selhal, soubor zůstane v knihovně. Sledování
	// změn starou cestu smaže samo, DeleteMedia je pak bez účinku.
	if err := os.Rename(s.abs(rec.FilePath), s.abs(dest)); err != nil {
		return "", fmt.Errorf("přesun do koše %s: %w", rec.FilePath, err)
	}
	stem := strings.TrimSuffix(path.Base(rec.FilePath), path.Ext(rec.FilePath))
	destStem := strings.TrimSuffix(path.Base(dest), path.Ext(dest))
	for _, sub := range s.subtitleNames(rec.FilePath) {
		from := path.Join(path.Dir(rec.FilePath), sub)
		to := path.Join(path.Dir(dest), destStem+sub[len(stem):])
		if err := os.Rename(s.abs(from), s.abs(to)); err != nil {
			s.log.Warn("přesun titulků do koše", "path", from, "err", err)
		}
	}
	if err := s.store.DeleteMedia(ctx, mediaID); err != nil {
		return "", err
	}
	s.log.Info("soubor přesunut do koše", "z", rec.FilePath, "do", dest)
	return dest, nil
}

// freeName vrátí rel, nebo – když už na disku existuje – variantu
// "název (2).ext", "název (3).ext"…
func (s *Scanner) freeName(rel string) string {
	if _, err := os.Lstat(s.abs(rel)); err != nil {
		return rel
	}
	ext := path.Ext(rel)
	base := strings.TrimSuffix(rel, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Lstat(s.abs(candidate)); err != nil {
			return candidate
		}
	}
}

// ErrTrashPath znamená cestu, která nevede do koše žádné knihovny.
var ErrTrashPath = errors.New("cesta není v koši knihovny")

// TrashItem je video v koši s titulky, které leží vedle něj.
type TrashItem struct {
	// Path je cesta v koši relativní k VIDEO_ROOT ("Seriály/.trash/Show/a.mkv").
	Path string `json:"path"`
	// OriginalPath je cesta, kam se soubor obnoví ("Seriály/Show/a.mkv").
	OriginalPath string    `json:"original_path"`
	SizeBytes    int64     `json:"size_bytes"`
	TrashedAt    time.Time `json:"trashed_at"`
	Subtitles    []string  `json:"subtitles"`
}

// TrashLibrary je koš jedné knihovny. SizeBytes počítá všechny soubory
// v koši, i titulky a soubory, které nejsou video.
type TrashLibrary struct {
	Dir       string      `json:"dir"`
	Name      string      `json:"name"`
	SizeBytes int64       `json:"size_bytes"`
	Items     []TrashItem `json:"items"`
}

// ListTrash vrátí koše všech knihoven (i prázdné), videa v nich seřazená
// od naposledy smazaného.
func (s *Scanner) ListTrash(ctx context.Context) ([]TrashLibrary, error) {
	libs, err := s.store.ListLibraries(ctx, false)
	if err != nil {
		return nil, err
	}
	out := []TrashLibrary{}
	for _, lib := range libs {
		tl := TrashLibrary{Dir: lib.Dir, Name: lib.Name, Items: []TrashItem{}}
		root := filepath.Join(s.root, lib.Dir, TrashDir)
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			tl.SizeBytes += info.Size()
			if !IsVideo(p) {
				return nil
			}
			rel := s.rel(p)
			item := TrashItem{Path: rel, OriginalPath: originalPath(lib.Dir, rel), SizeBytes: info.Size(), TrashedAt: info.ModTime(), Subtitles: []string{}}
			for _, sub := range s.subtitleNames(rel) {
				item.Subtitles = append(item.Subtitles, path.Join(path.Dir(rel), sub))
			}
			tl.Items = append(tl.Items, item)
			return nil
		})
		sort.Slice(tl.Items, func(a, b int) bool { return tl.Items[a].TrashedAt.After(tl.Items[b].TrashedAt) })
		out = append(out, tl)
	}
	return out, nil
}

// originalPath převede cestu v koši knihovny dir zpět na původní.
func originalPath(dir, rel string) string {
	return path.Join(dir, strings.TrimPrefix(rel, dir+"/"+TrashDir+"/"))
}

// trashLibrary ověří, že rel vede na soubor v koši existující knihovny,
// a vrátí knihovnu.
func (s *Scanner) trashLibrary(ctx context.Context, rel string) (*model.Library, error) {
	clean := path.Clean("/" + rel)[1:]
	if clean != rel {
		return nil, ErrTrashPath
	}
	top, rest, ok := strings.Cut(rel, "/")
	if !ok || !strings.HasPrefix(rest, TrashDir+"/") {
		return nil, ErrTrashPath
	}
	libs, err := s.store.ListLibraries(ctx, false)
	if err != nil {
		return nil, err
	}
	for i := range libs {
		if libs[i].Dir == top {
			return &libs[i], nil
		}
	}
	return nil, ErrTrashPath
}

// RestoreTrash vrátí video z koše (i s titulky) na původní místo a hned ho
// založí v knihovně. Když na původním místě už soubor je, vrátí
// ErrRenameConflict.
func (s *Scanner) RestoreTrash(ctx context.Context, rel string) (string, error) {
	lib, err := s.trashLibrary(ctx, rel)
	if err != nil {
		return "", err
	}
	if !IsVideo(rel) {
		return "", ErrTrashPath
	}
	if _, err := os.Stat(s.abs(rel)); err != nil {
		return "", storage.ErrNotFound
	}
	dest := originalPath(lib.Dir, rel)
	subs := s.subtitleNames(rel)
	for _, name := range append([]string{path.Base(rel)}, subs...) {
		if _, err := os.Lstat(s.abs(path.Join(path.Dir(dest), name))); err == nil {
			return "", fmt.Errorf("%w: %s", ErrRenameConflict, path.Join(path.Dir(dest), name))
		}
	}
	if err := os.MkdirAll(filepath.Dir(s.abs(dest)), 0o755); err != nil {
		return "", fmt.Errorf("obnovení: %w", err)
	}
	for _, sub := range subs {
		if err := os.Rename(s.abs(path.Join(path.Dir(rel), sub)), s.abs(path.Join(path.Dir(dest), sub))); err != nil {
			s.log.Warn("obnovení titulků", "path", sub, "err", err)
		}
	}
	if err := os.Rename(s.abs(rel), s.abs(dest)); err != nil {
		return "", fmt.Errorf("obnovení %s: %w", rel, err)
	}
	if lib.Enabled {
		if _, err := s.processIn(ctx, lib, s.abs(dest)); err != nil {
			return "", err
		}
	}
	s.log.Info("soubor obnoven z koše", "z", rel, "do", dest)
	return dest, nil
}

// DeleteTrash trvale smaže video z koše i s titulky vedle něj.
func (s *Scanner) DeleteTrash(ctx context.Context, rel string) error {
	if _, err := s.trashLibrary(ctx, rel); err != nil {
		return err
	}
	if !IsVideo(rel) {
		return ErrTrashPath
	}
	for _, sub := range s.subtitleNames(rel) {
		if err := os.Remove(s.abs(path.Join(path.Dir(rel), sub))); err != nil {
			s.log.Warn("mazání titulků z koše", "path", sub, "err", err)
		}
	}
	if err := os.Remove(s.abs(rel)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storage.ErrNotFound
		}
		return fmt.Errorf("mazání z koše %s: %w", rel, err)
	}
	s.removeEmptyDirs(path.Dir(rel))
	s.log.Info("soubor trvale smazán z koše", "path", rel)
	return nil
}

// EmptyTrash trvale smaže celý koš knihovny dir. Vrací uvolněné bajty.
func (s *Scanner) EmptyTrash(ctx context.Context, dir string) (int64, error) {
	libs, err := s.ListTrash(ctx)
	if err != nil {
		return 0, err
	}
	for _, l := range libs {
		if l.Dir != dir {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.root, dir, TrashDir)); err != nil {
			return 0, fmt.Errorf("vysypání koše %s: %w", dir, err)
		}
		s.log.Info("koš vysypán", "knihovna", dir, "bajtů", l.SizeBytes)
		return l.SizeBytes, nil
	}
	return 0, storage.ErrNotFound
}

// removeEmptyDirs smaže prázdné složky od dir nahoru, nejvýš po kořen koše.
func (s *Scanner) removeEmptyDirs(dir string) {
	for strings.Count(dir, "/") >= 2 && path.Base(dir) != TrashDir {
		if err := os.Remove(s.abs(dir)); err != nil {
			return
		}
		dir = path.Dir(dir)
	}
}
