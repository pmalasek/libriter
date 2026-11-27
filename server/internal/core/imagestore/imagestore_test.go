package imagestore

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtForContentType(t *testing.T) {
	cases := map[string]string{
		"image/jpeg":              ".jpg",
		"image/png":               ".png",
		"IMAGE/WEBP":              ".webp",
		"image/jpeg; charset=bin": ".jpg",
		" image/gif ":             ".gif",
		// Typ, který obrázek není, se nesmí uložit.
		"text/html":                ".",
		"application/octet-stream": ".",
	}
	for in, want := range cases {
		got := ExtForContentType(in)
		if want == "." {
			if got != "" {
				t.Errorf("ExtForContentType(%q) = %q, chtěno prázdno", in, got)
			}
			if IsImageContentType(in) {
				t.Errorf("IsImageContentType(%q) = true", in)
			}
			continue
		}
		if got != want {
			t.Errorf("ExtForContentType(%q) = %q, chtěno %q", in, got, want)
		}
	}
}

// tinyPNG vrátí skutečný obrázek 1×1 – SniffExt ho musí poznat.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWrite(t *testing.T) {
	root := filepath.Join(t.TempDir(), "obrazky") // ještě neexistuje – musí se vytvořit
	img := tinyPNG(t)

	name, err := Write(root, "kniha", img)
	if err != nil {
		t.Fatal(err)
	}
	// Přípona podle obsahu, ne podle toho, co tvrdí zdroj.
	if name != "kniha.png" {
		t.Fatalf("název = %q", name)
	}

	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil || !bytes.Equal(data, img) {
		t.Fatalf("obsah se liší, err %v", err)
	}
	// Zápis jde přes .tmp + rename, po sobě nesmí nic zůstat.
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatalf("zůstal .tmp soubor: %v", entries)
	}

	if _, err := Write("", "x", img); err == nil {
		t.Error("zápis bez nastaveného adresáře má selhat")
	}
}

// HTML vydávající se za obrázek se nesmí uložit – soubory obálek vydává
// endpoint bez přihlášení.
func TestWriteRejectsNonImage(t *testing.T) {
	root := t.TempDir()
	for _, data := range [][]byte{
		[]byte("<!doctype html><script>alert(1)</script>"),
		[]byte("obrazek"),
		nil,
	} {
		if _, err := Write(root, "x", data); !errors.Is(err, ErrNotImage) {
			t.Errorf("Write(%q) = %v, chtěno ErrNotImage", data, err)
		}
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("na disku něco zůstalo: %v", entries)
	}
}

func TestExistsAndRemove(t *testing.T) {
	root := t.TempDir()

	if Exists(root, "") || Exists(root, "chybi.jpg") {
		t.Error("neexistující soubor se hlásí jako existující")
	}

	name := "ok.jpg"
	if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Exists(root, name) {
		t.Error("existující obrázek nebyl rozpoznán")
	}

	if err := Remove(root, name); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if Exists(root, name) {
		t.Error("soubor po Remove zůstal")
	}
	// Mazání neexistujícího souboru není chyba.
	if err := Remove(root, name); err != nil {
		t.Errorf("Remove neexistujícího: %v", err)
	}
}

func TestResolveBlocksTraversal(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name     string
		root     string
		fileName string
		want     bool
	}{
		{"holý název souboru", root, "abc.jpg", true},
		{"prázdný root", "", "abc.jpg", false},
		{"prázdný název", root, "", false},
		{"tečka", root, ".", false},
		{"dvě tečky", root, "..", false},
		{"únik nahoru", root, "../secret.txt", false},
		{"podadresář", root, "sub/x.jpg", false},
		{"absolutní cesta", root, "/etc/passwd", false},
		{"zpětné lomítko", root, `..\x.jpg`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			abs, ok := Resolve(tt.root, tt.fileName)
			if ok != tt.want {
				t.Fatalf("Resolve(%q, %q) ok = %v, chtěno %v", tt.root, tt.fileName, ok, tt.want)
			}
			if ok && !strings.HasPrefix(abs, tt.root) {
				t.Errorf("cesta %q vede mimo %q", abs, tt.root)
			}
		})
	}
}

func TestReadLimited(t *testing.T) {
	data, err := ReadLimited(strings.NewReader("obrazek"))
	if err != nil || string(data) != "obrazek" {
		t.Fatalf("data = %q, err = %v", data, err)
	}

	// Cizí server nesmí zaplnit disk.
	huge := strings.NewReader(strings.Repeat("x", MaxBytes+1))
	if _, err := ReadLimited(huge); err == nil {
		t.Error("příliš velký obrázek měl skončit chybou")
	}
}
