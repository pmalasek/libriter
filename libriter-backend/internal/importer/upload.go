package importer

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"libriter/internal/imagestore"
	"libriter/internal/scanner"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

// Doprovodné soubory, které se s knihou nahrávají: metadata a playlist.
var sidecarExts = map[string]bool{".html": true, ".htm": true, ".pls": true}

// isJunk pozná systémové soubory, které do knihovny nepatří (macOS metadata
// v zipech, náhledy Windows).
func isJunk(rel string) bool {
	for _, part := range strings.Split(rel, "/") {
		lower := strings.ToLower(part)
		if part == "__MACOSX" || strings.HasPrefix(part, "._") ||
			lower == "thumbs.db" || lower == ".ds_store" || lower == "desktop.ini" {
			return true
		}
	}
	return false
}

// isAccepted vrátí true pro typy souborů, které import přijímá.
func isAccepted(rel string) bool {
	ext := strings.ToLower(path.Ext(rel))
	return scanner.IsAudioFile(rel) || ext == ".zip" || sidecarExts[ext] || imagestore.IsImageExt(ext)
}

// CleanRelPath ověří a normalizuje relativní cestu souboru od klienta:
// oddělovače "/", Unicode NFC (macOS posílá rozložené znaky), bez "..",
// absolutních cest, prázdných a řídicích znaků.
func CleanRelPath(p string) (string, error) {
	p = norm.NFC.String(strings.ReplaceAll(p, "\\", "/"))
	p = strings.TrimLeft(p, "/")
	if p == "" {
		return "", ErrInvalidPath
	}

	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch part {
		case "", ".":
			continue
		case "..":
			return "", ErrInvalidPath
		}
		for _, r := range part {
			if unicode.IsControl(r) {
				return "", ErrInvalidPath
			}
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return "", ErrInvalidPath
	}
	return strings.Join(out, "/"), nil
}

// AddFile uloží jeden nahraný soubor do relace. Systémové soubory tiše
// zahodí (vrátí false), nepodporované typy odmítne.
func (s *Service) AddFile(id uuid.UUID, relPath string, body io.Reader) (bool, error) {
	rel, err := CleanRelPath(relPath)
	if err != nil {
		return false, err
	}
	if isJunk(rel) {
		return false, nil
	}
	if !isAccepted(rel) {
		return false, fmt.Errorf("%w: %s", ErrFileType, path.Base(rel))
	}

	s.mu.Lock()
	sess, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return false, ErrNotFound
	}
	if sess.State != StateUploading {
		s.mu.Unlock()
		return false, ErrWrongState
	}
	remaining := s.maxBytes - sess.UploadedBytes
	s.mu.Unlock()

	dst := filepath.Join(s.filesDir(id), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}

	// Stejná cesta nahraná znovu (opakovaný pokus) soubor přepíše.
	var previous int64
	fi, statErr := os.Stat(dst)
	replaced := statErr == nil
	if replaced {
		previous = fi.Size()
	}

	f, err := os.Create(dst)
	if err != nil {
		return false, err
	}
	// O bajt víc než zbývá, aby šlo poznat překročení limitu.
	n, copyErr := io.Copy(f, io.LimitReader(body, remaining+previous+1))
	closeErr := f.Close()
	if copyErr == nil && n > remaining+previous {
		copyErr = ErrTooLarge
	}
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = os.Remove(dst)
		if replaced {
			s.update(id, func(sess *Session) {
				sess.UploadedFiles--
				sess.UploadedBytes -= previous
			})
		}
		return false, copyErr
	}

	s.update(id, func(sess *Session) {
		if !replaced {
			sess.UploadedFiles++
		}
		sess.UploadedBytes += n - previous
	})
	return true, nil
}

// UploadedFile je soubor, který už v relaci je.
type UploadedFile struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Files vrátí soubory nahrané do relace. Klient podle nich při pokračování
// přerušeného nahrávání přeskočí, co už server má. Nedokončený soubor AddFile
// maže, takže shoda cesty a velikosti znamená kompletní soubor.
func (s *Service) Files(id uuid.UUID) ([]UploadedFile, error) {
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
	s.mu.Unlock()

	root := s.filesDir(id)
	out := []UploadedFile{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out = append(out, UploadedFile{Path: filepath.ToSlash(rel), Size: fi.Size()})
		return nil
	})
	return out, err
}
