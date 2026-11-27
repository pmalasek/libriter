package fsroot

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// library vytvoří knihovnu s běžným souborem, odkazy dovnitř i ven, FIFO
// a souborem mimo kořen. Vrací kořen knihovny a cestu k cizímu souboru.
func library(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "lib")
	if err := os.MkdirAll(filepath.Join(root, "autor", "kniha"), 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(base, "secret.txt")
	mustWrite(t, secret, "tajne")
	mustWrite(t, filepath.Join(root, "autor", "kniha", "01.mp3"), "audio")

	mustLink(t, "kniha/01.mp3", filepath.Join(root, "autor", "rel.mp3"))
	mustLink(t, filepath.Join(root, "autor", "kniha", "01.mp3"), filepath.Join(root, "autor", "abs.mp3"))
	mustLink(t, secret, filepath.Join(root, "autor", "out.mp3"))
	mustLink(t, "../../secret.txt", filepath.Join(root, "autor", "outrel.mp3"))
	mustLink(t, base, filepath.Join(root, "ven"))
	if err := syscall.Mkfifo(filepath.Join(root, "autor", "fifo.mp3"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	return root, secret
}

func mustWrite(t *testing.T, p, data string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// withTimeout selže, když f do pár sekund nedoběhne (zablokované FIFO).
func withTimeout(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("operace se zablokovala")
	}
}

func TestJoin(t *testing.T) {
	for _, rel := range []string{"", "/etc/passwd", "../x", "a/../../x", "a/.."} {
		if _, err := Join("/data", rel); err == nil {
			t.Errorf("Join(%q) prošel", rel)
		}
	}
	if p, err := Join("/data", "a/b c.mp3"); err != nil || p != "/data/a/b c.mp3" {
		t.Errorf("Join = %q %v", p, err)
	}
	if _, err := Join("", "a"); err == nil {
		t.Error("prázdný kořen prošel")
	}
}

func TestResolveAndOpen(t *testing.T) {
	root, _ := library(t)

	for _, rel := range []string{"autor/kniha/01.mp3", "autor/rel.mp3", "autor/abs.mp3"} {
		if _, err := Resolve(root, rel); err != nil {
			t.Errorf("Resolve(%q): %v", rel, err)
		}
		f, info, err := Open(root, rel)
		if err != nil {
			t.Errorf("Open(%q): %v", rel, err)
			continue
		}
		if info.Size() != int64(len("audio")) {
			t.Errorf("Open(%q) velikost %d", rel, info.Size())
		}
		f.Close()
	}

	for _, rel := range []string{"autor/out.mp3", "autor/outrel.mp3", "ven/secret.txt"} {
		if _, err := Resolve(root, rel); !errors.Is(err, ErrOutside) {
			t.Errorf("Resolve(%q) = %v, chtěno ErrOutside", rel, err)
		}
		if _, _, err := Open(root, rel); !errors.Is(err, ErrOutside) {
			t.Errorf("Open(%q) = %v, chtěno ErrOutside", rel, err)
		}
	}

	if _, err := Resolve(root, "autor/kniha"); !errors.Is(err, ErrNotRegular) {
		t.Errorf("adresář: %v", err)
	}
	if _, err := Resolve(root, "autor/chybi.mp3"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("chybějící soubor: %v", err)
	}
}

func TestFIFORejectedWithoutBlocking(t *testing.T) {
	root, _ := library(t)
	fifo := filepath.Join(root, "autor", "fifo.mp3")
	withTimeout(t, func() {
		if _, err := Resolve(root, "autor/fifo.mp3"); !errors.Is(err, ErrNotRegular) {
			t.Errorf("Resolve FIFO: %v", err)
		}
		if _, _, err := Open(root, "autor/fifo.mp3"); !errors.Is(err, ErrNotRegular) {
			t.Errorf("Open FIFO: %v", err)
		}
		if _, err := ReadFile(fifo, 100); !errors.Is(err, ErrNotRegular) {
			t.Errorf("ReadFile FIFO: %v", err)
		}
		if _, err := Stat(root, fifo); !errors.Is(err, ErrNotRegular) {
			t.Errorf("Stat FIFO: %v", err)
		}
	})
}

func TestRootBehindSymlink(t *testing.T) {
	root, _ := library(t)
	alias := filepath.Join(t.TempDir(), "alias")
	mustLink(t, root, alias)

	// Kořen zadaný odkazem: absolutní odkaz uvnitř vede na skutečnou cestu
	// kořene, i tak musí projít.
	for _, rel := range []string{"autor/kniha/01.mp3", "autor/abs.mp3", "autor/rel.mp3"} {
		if _, _, err := Open(alias, rel); err != nil {
			t.Errorf("Open přes odkaz na kořen (%q): %v", rel, err)
		}
	}
	if _, _, err := Open(alias, "autor/out.mp3"); !errors.Is(err, ErrOutside) {
		t.Errorf("únik přes odkaz na kořen: %v", err)
	}
}

func TestContainsMissingTail(t *testing.T) {
	root, _ := library(t)
	if !Contains(root, filepath.Join(root, "novy", "autor", "kniha.mp3")) {
		t.Error("neexistující cesta v kořeni má projít")
	}
	if Contains(root, filepath.Join(root, "ven", "novy", "x.mp3")) {
		t.Error("neexistující cesta za odkazem ven nesmí projít")
	}
	if !Contains("/neexistuje/knihovna", "/neexistuje/knihovna/a.mp3") {
		t.Error("neexistující kořen: lexikálně vnořená cesta má projít")
	}
}

func TestReadFileLimit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "big")
	mustWrite(t, p, "0123456789")
	if _, err := ReadFile(p, 5); !errors.Is(err, ErrTooLarge) {
		t.Errorf("strop velikosti: %v", err)
	}
	data, err := ReadFile(p, 10)
	if err != nil || string(data) != "0123456789" {
		t.Errorf("ReadFile = %q %v", data, err)
	}
}

func TestReadFileIn(t *testing.T) {
	root, secret := library(t)
	if _, err := ReadFileIn(root, filepath.Join(root, "autor", "out.mp3"), 100); !errors.Is(err, ErrOutside) {
		t.Errorf("odkaz ven: %v", err)
	}
	if _, err := ReadFileIn(root, secret, 100); !errors.Is(err, ErrOutside) {
		t.Errorf("soubor mimo kořen: %v", err)
	}
	if data, err := ReadFileIn(root, filepath.Join(root, "autor", "rel.mp3"), 100); err != nil || string(data) != "audio" {
		t.Errorf("odkaz dovnitř: %q %v", data, err)
	}
}
