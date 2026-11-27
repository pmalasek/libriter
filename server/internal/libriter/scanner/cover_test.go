package scanner

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"mediater/internal/libriter/model"

	"github.com/google/uuid"
)

func TestLargestImageInDir(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, n int) {
		if err := os.WriteFile(filepath.Join(dir, name), make([]byte, n), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("thumb.jpg", 100)
	write("cover.png", 5000)
	write("notes.txt", 90000)
	write("empty.jpg", 0)

	got := largestImageInDir(dir, dir)
	if filepath.Base(got) != "cover.png" {
		t.Fatalf("chtěl cover.png, dostal %q", got)
	}

	empty := t.TempDir()
	if got := largestImageInDir(empty, empty); got != "" {
		t.Fatalf("prázdný adresář má vrátit \"\", dostal %q", got)
	}
}

// tinyJPEG vrátí skutečný obrázek – obálka se ukládá jen podle obsahu.
func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Odkaz cover.jpg mimo knihovnu by zkopíroval cizí soubor do COVER_ROOT,
// odkud ho vydává endpoint bez přihlášení. FIFO by zablokovalo čtení.
func TestCoverIgnoresSymlinkOutsideAndFIFO(t *testing.T) {
	base := t.TempDir()
	audioRoot := filepath.Join(base, "audio")
	bookDir := filepath.Join(audioRoot, "Autor", "Kniha")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Cizí „obrázek“ je větší než cokoli v knihovně – kdyby ho výběr bral,
	// vyhrál by.
	secret := filepath.Join(base, "secret.jpg")
	if err := os.WriteFile(secret, append(tinyJPEG(t), make([]byte, 4096)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(bookDir, "cover.jpg")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(bookDir, "folder.jpg"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	coverRoot := filepath.Join(base, "covers")
	s := &Scanner{audioRoot: audioRoot, coverRoot: coverRoot}

	done := make(chan struct{})
	var rel string
	var err error
	go func() {
		defer close(done)
		rel, _, err = s.resolveCover(uuid.New(), bookDir, filepath.Join(bookDir, "01.mp3"))
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hledání obálky se zablokovalo na FIFO")
	}
	if err != nil || rel != "" {
		t.Fatalf("obálka z odkazu ven: rel %q err %v", rel, err)
	}
	if entries, _ := os.ReadDir(coverRoot); len(entries) != 0 {
		t.Fatalf("do COVER_ROOT se něco zkopírovalo: %v", entries)
	}

	// Odkaz uvnitř knihovny dál funguje.
	inside := filepath.Join(audioRoot, "Autor", "spolecna.jpg")
	if err := os.WriteFile(inside, tinyJPEG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(bookDir, "cover.jpg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, filepath.Join(bookDir, "cover.jpg")); err != nil {
		t.Fatal(err)
	}
	rel, _, err = s.resolveCover(uuid.New(), bookDir, filepath.Join(bookDir, "01.mp3"))
	if err != nil || filepath.Ext(rel) != ".jpg" {
		t.Fatalf("obálka přes odkaz uvnitř: rel %q err %v", rel, err)
	}
}

// HTML s příponou .jpg se do COVER_ROOT neuloží.
func TestCoverRejectsNonImage(t *testing.T) {
	base := t.TempDir()
	bookDir := filepath.Join(base, "audio", "Kniha")
	if err := os.MkdirAll(bookDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bookDir, "cover.jpg"), []byte("<html><script>x</script></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	coverRoot := filepath.Join(base, "covers")
	s := &Scanner{audioRoot: filepath.Join(base, "audio"), coverRoot: coverRoot}
	if rel, _, err := s.resolveCover(uuid.New(), bookDir, ""); err == nil || rel != "" {
		t.Fatalf("HTML jako obálka: rel %q err %v", rel, err)
	}
	if entries, _ := os.ReadDir(coverRoot); len(entries) != 0 {
		t.Fatalf("do COVER_ROOT se něco uložilo: %v", entries)
	}
}

func TestHasCover(t *testing.T) {
	root := t.TempDir()
	s := &Scanner{coverRoot: root}

	if s.hasCover(&model.Book{}) {
		t.Error("kniha bez cover_path nemá mít obálku")
	}

	missing := "chybi.jpg"
	if s.hasCover(&model.Book{CoverPath: &missing}) {
		t.Error("cover_path na neexistující soubor se má brát jako chybějící")
	}

	name := "ok.jpg"
	if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.hasCover(&model.Book{CoverPath: &name}) {
		t.Error("existující obálka nebyla rozpoznána")
	}
}
