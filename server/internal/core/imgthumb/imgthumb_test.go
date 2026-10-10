package imgthumb

import (
	"image"
	"image/color"
	"image/jpeg"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

func decodeSize(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestSnapWidth(t *testing.T) {
	for in, want := range map[int]int{0: 0, -1: 0, 1: 160, 160: 160, 161: 320, 640: 640, 5000: 640} {
		if got := SnapWidth(in); got != want {
			t.Errorf("SnapWidth(%d) = %d, chci %d", in, got, want)
		}
	}
}

func TestThumb(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "cover.jpg")
	writeJPEG(t, src, 800, 1200)
	c := New(filepath.Join(dir, "thumbs"))

	// Souběžné požadavky na tutéž zmenšeninu skončí stejnou cestou.
	var wg sync.WaitGroup
	paths := make([]string, 8)
	for i := range paths {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := c.Thumb(src, 320)
			if err != nil {
				t.Error(err)
			}
			paths[i] = p
		}()
	}
	wg.Wait()
	for _, p := range paths {
		if p != paths[0] {
			t.Fatalf("různé cesty: %v", paths)
		}
	}
	if w, h := decodeSize(t, paths[0]); w != 320 || h != 480 {
		t.Errorf("rozměr %dx%d", w, h)
	}

	// Malý originál se nezvětšuje.
	small := filepath.Join(dir, "small.jpg")
	writeJPEG(t, small, 100, 50)
	p, err := c.Thumb(small, 640)
	if err != nil {
		t.Fatal(err)
	}
	if w, h := decodeSize(t, p); w != 100 || h != 50 {
		t.Errorf("malý originál %dx%d", w, h)
	}

	// Novější originál zmenšeninu obnoví.
	writeJPEG(t, src, 400, 400)
	future := time.Now().Add(time.Minute)
	os.Chtimes(src, future, future)
	p, _ = c.Thumb(src, 320)
	if w, h := decodeSize(t, p); w != 320 || h != 320 {
		t.Errorf("po změně originálu %dx%d", w, h)
	}
}

func TestServeFileFallsBackToOriginal(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "broken.jpg")
	os.WriteFile(src, []byte("not an image"), 0o644)
	c := New(filepath.Join(dir, "thumbs"))

	rec := httptest.NewRecorder()
	c.ServeFile(rec, httptest.NewRequest("GET", "/x?w=160", nil), src)
	if rec.Code != 200 || rec.Body.String() != "not an image" {
		t.Errorf("fallback: %d %q", rec.Code, rec.Body.String())
	}
}
