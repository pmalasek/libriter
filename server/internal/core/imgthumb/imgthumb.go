// Package imgthumb zmenšuje obrázky (obálky, fotky, plakáty) pro mřížky a
// seznamy. Dlaždice široká pár set pixelů nepotřebuje originál o několika
// megabajtech; zmenšenina se vytvoří při prvním požadavku a dál se čte
// z cache na disku.
package imgthumb

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	_ "image/gif" // registrace dekodérů pro image.Decode

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Widths jsou šířky, které se generují. Požadovaná šířka se zaokrouhlí
// nahoru na nejbližší z nich, aby cache nebobtnala libovolnými rozměry.
var Widths = []int{160, 320, 640}

// maxPixels chrání před dekompresní bombou – obálka větší než 50 Mpx není.
const maxPixels = 50_000_000

// jpegQuality stačí na zmenšeninu; rozdíl proti 90 na dlaždici není vidět.
const jpegQuality = 80

// Cache vytváří a drží zmenšeniny v adresáři root.
type Cache struct {
	root string
	// sem omezuje souběžné dekódování – každý obrázek v paměti zabere
	// šířka × výška × 4 B a mřížka jich chce desítky naráz.
	sem chan struct{}

	mu       sync.Mutex
	inflight map[string]*call
}

type call struct {
	done chan struct{}
	err  error
}

// New vrátí cache zmenšenin v adresáři root (vytvoří se při prvním zápisu).
func New(root string) *Cache {
	return &Cache{
		root:     root,
		sem:      make(chan struct{}, max(runtime.NumCPU(), 2)),
		inflight: map[string]*call{},
	}
}

// SnapWidth zaokrouhlí šířku nahoru na podporovanou; 0 = originál.
func SnapWidth(w int) int {
	if w <= 0 {
		return 0
	}
	for _, allowed := range Widths {
		if w <= allowed {
			return allowed
		}
	}
	return Widths[len(Widths)-1]
}

// Thumb vrátí cestu ke zmenšenině souboru src o šířce width (už zaokrouhlené
// přes SnapWidth). Zmenšenina se znovu vytvoří, když je originál novější.
func (c *Cache) Thumb(src string, width int) (string, error) {
	info, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	sum := sha1.Sum([]byte(src))
	name := hex.EncodeToString(sum[:12]) + "-" + strconv.Itoa(width)
	base := filepath.Join(c.root, name[:2], name)

	if p, ok := fresh(base, info); ok {
		return p, nil
	}

	// Stejnou zmenšeninu chce často víc požadavků naráz (mřížka po
	// načtení stránky); vytvoří ji jen první, ostatní počkají.
	c.mu.Lock()
	if cl, ok := c.inflight[base]; ok {
		c.mu.Unlock()
		<-cl.done
		if cl.err != nil {
			return "", cl.err
		}
		if p, ok := fresh(base, info); ok {
			return p, nil
		}
		return "", errors.New("zmenšenina zmizela")
	}
	cl := &call{done: make(chan struct{})}
	c.inflight[base] = cl
	c.mu.Unlock()

	var path string
	path, cl.err = c.generate(src, base, width)

	c.mu.Lock()
	delete(c.inflight, base)
	c.mu.Unlock()
	close(cl.done)
	return path, cl.err
}

// fresh najde existující zmenšeninu (JPEG nebo PNG), která není starší než originál.
func fresh(base string, src os.FileInfo) (string, bool) {
	for _, ext := range []string{".jpg", ".png"} {
		if t, err := os.Stat(base + ext); err == nil && !t.ModTime().Before(src.ModTime()) {
			return base + ext, true
		}
	}
	return "", false
}

func (c *Cache) generate(src, base string, width int) (string, error) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return "", fmt.Errorf("imgthumb: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return "", fmt.Errorf("imgthumb: nepodporovaný rozměr %dx%d", cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("imgthumb: %w", err)
	}

	// Nezvětšuje se – menší originál se jen přeuloží (pořád bývá menší
	// než PNG nebo JPEG v nejvyšší kvalitě).
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > width {
		h = max(h*width/w, 1)
		w = width
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)

	// Průhlednost (PNG loga, výřezy) by JPEG zničil na černé pozadí.
	ext := ".jpg"
	if !dst.Opaque() {
		ext = ".png"
	}

	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(base), ".thumb-*")
	if err != nil {
		return "", err
	}
	if ext == ".png" {
		err = png.Encode(tmp, dst)
	} else {
		err = jpeg.Encode(tmp, dst, &jpeg.Options{Quality: jpegQuality})
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("imgthumb: zápis: %w", err)
	}
	if err := os.Rename(tmp.Name(), base+ext); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return base + ext, nil
}

// ServeFile pošle obrázek abs: s ?w= jeho zmenšeninu, jinak originál. Když
// zmenšenina nejde vytvořit (poškozený nebo neznámý formát), pošle originál –
// prohlížeč si s ním možná poradí.
func (c *Cache) ServeFile(w http.ResponseWriter, r *http.Request, abs string) {
	width, _ := strconv.Atoi(r.URL.Query().Get("w"))
	if width = SnapWidth(width); width > 0 && c != nil {
		if p, err := c.Thumb(abs, width); err == nil {
			http.ServeFile(w, r, p)
			return
		}
	}
	http.ServeFile(w, r, abs)
}
