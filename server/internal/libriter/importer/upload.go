package importer

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"mediater/internal/core/imagestore"
	"mediater/internal/libriter/scanner"

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

// uploadFileMode jsou práva nahraných souborů. Po importu se přesunou do
// AUDIO_ROOT, a kontejner běží s UMASK=0000 – os.Create by dal 0666 a soubor
// by mohl přepsat kdokoli s přístupem ke sdílené knihovně.
const uploadFileMode os.FileMode = 0o644

// createUploadFile otevře cílový soubor pro zápis (existující zkrátí).
func createUploadFile(dst string) (*os.File, error) {
	return os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, uploadFileMode)
}

// AddFile uloží jeden nahraný soubor do relace. Systémové soubory tiše
// zahodí (vrátí false), nepodporované typy odmítne.
//
// size je deklarovaná velikost těla (Content-Length), -1 když není známá.
// Místo v limitu relace se rezervuje pod zámkem ještě před zápisem: známá
// velikost se rezervuje přesně, neznámá zabere celý zbytek limitu. Souběžné
// nahrávání tak limit relace nepřekročí – každý zápis smí jen to, co si
// rezervoval.
func (s *Service) AddFile(id uuid.UUID, relPath string, body io.Reader, size int64) (bool, error) {
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

	dst := filepath.Join(s.filesDir(id), filepath.FromSlash(rel))
	res, err := s.reserveUpload(id, rel, dst, size)
	if err != nil {
		return false, err
	}

	n, err := writeUpload(dst, body, res.bytes)
	if err != nil {
		_ = os.Remove(dst)
	}
	s.finishUpload(id, rel, res, n, err)
	if err != nil {
		return false, err
	}
	return true, nil
}

// uploadReservation je místo v limitu relace zabrané jedním zápisem.
type uploadReservation struct {
	bytes    int64 // kolik bajtů smí zápis nejvýš zapsat
	previous int64 // velikost přepisovaného souboru (0 = nový soubor)
	replaced bool
}

// reserveUpload pod zámkem ověří fázi relace, zamkne cestu proti souběžnému
// zápisu a rezervuje místo v limitu. Přepisovaný soubor (opakovaný pokus)
// se zápisem zkrátí, jeho bajty se proto hned uvolní.
func (s *Service) reserveUpload(id uuid.UUID, rel, dst string, size int64) (uploadReservation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return uploadReservation{}, ErrNotFound
	}
	if sess.State != StateUploading {
		return uploadReservation{}, ErrWrongState
	}
	if _, busy := sess.writing[rel]; busy {
		return uploadReservation{}, ErrUploadBusy
	}

	var res uploadReservation
	if fi, err := os.Stat(dst); err == nil {
		res.previous = fi.Size()
		res.replaced = true
	}

	available := s.maxBytes - sess.UploadedBytes - sess.reserved + res.previous
	switch {
	case size > available:
		return uploadReservation{}, ErrTooLarge
	case size >= 0:
		res.bytes = size
	default:
		res.bytes = max(available, 0)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return uploadReservation{}, err
	}

	if sess.writing == nil {
		sess.writing = make(map[string]struct{})
	}
	sess.writing[rel] = struct{}{}
	sess.reserved += res.bytes
	if res.replaced {
		sess.UploadedFiles--
		sess.UploadedBytes -= res.previous
	}
	return res, nil
}

// writeUpload zapíše tělo do dst, nejvýš limit bajtů (víc = ErrTooLarge).
func writeUpload(dst string, body io.Reader, limit int64) (int64, error) {
	f, err := createUploadFile(dst)
	if err != nil {
		return 0, err
	}
	// O bajt víc než je rezervováno, aby šlo poznat překročení limitu.
	n, err := io.Copy(f, io.LimitReader(body, limit+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > limit {
		err = ErrTooLarge
	}
	return n, err
}

// finishUpload uvolní rezervaci a započítá zapsaný soubor. Při chybě je
// soubor smazaný; přepisovaný soubor zmizel také (zápis ho zkrátil), takže
// zůstává odečtený.
func (s *Service) finishUpload(id uuid.UUID, rel string, res uploadReservation, n int64, writeErr error) {
	s.mu.Lock()
	sess, ok := s.sessions[id]
	if !ok {
		// Relaci mezitím někdo smazal; MkdirAll ji mohl na disku znovu
		// založit, tak po sobě uklidit.
		s.mu.Unlock()
		_ = os.RemoveAll(s.sessionDir(id))
		return
	}
	defer s.mu.Unlock()

	delete(sess.writing, rel)
	sess.reserved -= res.bytes
	if writeErr == nil {
		sess.UploadedFiles++
		sess.UploadedBytes += n
	}
	sess.UpdatedAt = time.Now().UTC()
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
