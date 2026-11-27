// Package fsroot hlídá cesty do knihoven na disku (AUDIO_ROOT, VIDEO_ROOT).
//
// Cesta z databáze i soubor nalezený průchodem disku může být symbolický
// odkaz. Odkazy uvnitř knihovny jsou v pořádku (NAS je běžně používá), odkaz
// ven z ní ale nesmí zpřístupnit cizí soubor. Proto se cíl i kořen překládají
// přes EvalSymlinks a porovnávají až skutečné cesty. Kořen sám smí být odkaz
// (svazek Dockeru, přípojný bod).
//
// Číst se smí jen běžný soubor: FIFO by os.Open zablokovalo a zařízení by
// vrátilo nekonečná data. Soubory se proto otevírají s O_NONBLOCK a typ se
// ověřuje na otevřeném deskriptoru, ne podle jména.
//
// os.Root se nepoužívá záměrně: odmítá i absolutní odkazy, které vedou
// dovnitř kořene, a ty na NASu bývají.
package fsroot

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	// ErrOutside: cesta vede mimo kořen (lexikálně nebo přes odkaz).
	ErrOutside = errors.New("cesta vede mimo kořen knihovny")
	// ErrNotRegular: na cestě není běžný soubor (adresář, FIFO, zařízení…).
	ErrNotRegular = errors.New("cesta nevede na běžný soubor")
	// ErrTooLarge: soubor přesahuje povolenou velikost.
	ErrTooLarge = errors.New("soubor je příliš velký")
)

// Join ověří relativní cestu z databáze lexikálně (není absolutní, nemá
// složku „..“) a složí ji pod kořen. Odkazy nepřekládá – na to je Contains.
func Join(root, rel string) (string, error) {
	if root == "" || rel == "" || filepath.IsAbs(rel) {
		return "", ErrOutside
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".." {
			return "", ErrOutside
		}
	}
	// root může být relativní cesta (viz config.env.path).
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(absRoot, filepath.FromSlash(rel))
	if !within(absRoot, abs) || abs == absRoot {
		return "", ErrOutside
	}
	return abs, nil
}

// Contains řekne, jestli path po překladu odkazů leží v kořeni root (nebo
// je to kořen sám). Neexistující konec cesty nevadí – přeloží se nejbližší
// existující předek, takže jde ověřit i cíl, který teprve vznikne.
func Contains(root, path string) bool {
	realRoot, err := resolveExisting(root)
	if err != nil {
		return false
	}
	real, err := resolveExisting(path)
	if err != nil {
		return false
	}
	return within(realRoot, real)
}

// Stat vrátí informace o souboru path (cíli odkazu), pokud po překladu
// odkazů leží v kořeni a je to běžný soubor. Slouží průchodu knihovnou;
// čtení obsahu má jít přes Open/OpenFile, které ověří i otevřený deskriptor.
func Stat(root, path string) (fs.FileInfo, error) {
	if !Contains(root, path) {
		return nil, ErrOutside
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	return info, nil
}

// Resolve složí relativní cestu pod kořen a ověří, že po překladu odkazů
// vede na běžný soubor uvnitř kořene. Vrací cestu pod kořenem (ne cíl
// odkazu), aby se v logu a v ffmpeg objevovalo jméno z knihovny.
func Resolve(root, rel string) (string, error) {
	abs, err := Join(root, rel)
	if err != nil {
		return "", err
	}
	if _, err := Stat(root, abs); err != nil {
		return "", err
	}
	return abs, nil
}

// Open otevře soubor rel uvnitř kořene ke čtení. Odkazy uvnitř kořene
// následuje, ven ne; vrací jen běžný soubor.
func Open(root, rel string) (*os.File, fs.FileInfo, error) {
	abs, err := Join(root, rel)
	if err != nil {
		return nil, nil, err
	}
	return OpenIn(root, abs)
}

// OpenIn otevře soubor s absolutní cestou path, který musí po překladu
// odkazů ležet v kořeni root.
//
// Otevírá se už přeložená cesta s O_NOFOLLOW, takže odkaz podstrčený mezi
// kontrolou a otevřením na místě souboru neprojde.
func OpenIn(root, path string) (*os.File, fs.FileInfo, error) {
	realRoot, err := resolveExisting(root)
	if err != nil {
		return nil, nil, err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, nil, err
	}
	if !within(realRoot, real) {
		return nil, nil, ErrOutside
	}
	return openRegular(real, syscall.O_NOFOLLOW)
}

// OpenFile otevře ke čtení soubor mimo knihovnu (mezipaměť, dočasný soubor)
// tak, aby FIFO nezablokovalo, a vrátí ho jen tehdy, je-li to běžný soubor.
func OpenFile(path string) (*os.File, fs.FileInfo, error) {
	return openRegular(path, 0)
}

// ReadFile načte nejvýš max bajtů běžného souboru; větší soubor je chyba.
func ReadFile(path string, max int64) ([]byte, error) {
	f, info, err := OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLimited(f, info, max)
}

// ReadFileIn načte nejvýš max bajtů souboru path uvnitř kořene root.
func ReadFileIn(root, path string, max int64) ([]byte, error) {
	f, info, err := OpenIn(root, path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readLimited(f, info, max)
}

func readLimited(f *os.File, info fs.FileInfo, max int64) ([]byte, error) {
	if info.Size() > max {
		return nil, fmt.Errorf("%w: %d B", ErrTooLarge, info.Size())
	}
	// Soubor může mezitím narůst – strop platí i pro to, co se skutečně přečte.
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, ErrTooLarge
	}
	return data, nil
}

// openRegular otevře soubor s O_NONBLOCK (FIFO pak open nezablokuje)
// a ověří typ až na otevřeném deskriptoru – mezi kontrolou a otevřením se
// tak soubor nedá vyměnit. U běžného souboru O_NONBLOCK nic nemění.
func openRegular(path string, extra int) (*os.File, fs.FileInfo, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|extra, 0)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, nil, ErrNotRegular
	}
	return f, info, nil
}

// resolveExisting přeloží odkazy v cestě. Když konec cesty neexistuje,
// přeloží nejbližšího existujícího předka a zbytek k němu připojí.
func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var rest []string
	for {
		real, err := filepath.EvalSymlinks(abs)
		if err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				real = filepath.Join(real, rest[i])
			}
			return real, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", err
		}
		rest = append(rest, filepath.Base(abs))
		abs = parent
	}
}

// within řekne, jestli čistá absolutní cesta p leží v root nebo je root.
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
