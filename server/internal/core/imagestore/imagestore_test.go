package imagestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeExt(t *testing.T) {
	cases := map[string]string{
		".JPEG": ".jpeg", "png": ".png", ".xyz": ".jpg", "": ".jpg", ".webp": ".webp",
	}
	for in, want := range cases {
		if got := NormalizeExt(in); got != want {
			t.Errorf("NormalizeExt(%q) = %q, chtěno %q", in, got, want)
		}
	}
}

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

func TestWriteAndCopyFile(t *testing.T) {
	root := filepath.Join(t.TempDir(), "obrazky") // ještě neexistuje – musí se vytvořit

	src := filepath.Join(t.TempDir(), "folder.JPG")
	if err := os.WriteFile(src, []byte("obrazek"), 0o644); err != nil {
		t.Fatal(err)
	}

	name, err := CopyFile(root, "kniha", src)
	if err != nil {
		t.Fatal(err)
	}
	if name != "kniha.jpg" {
		t.Fatalf("název = %q", name)
	}

	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil || string(data) != "obrazek" {
		t.Fatalf("obsah %q err %v", data, err)
	}
	// Zápis jde přes .tmp + rename, po sobě nesmí nic zůstat.
	if entries, _ := os.ReadDir(root); len(entries) != 1 {
		t.Fatalf("zůstal .tmp soubor: %v", entries)
	}

	if _, err := Write("", "x", []byte("a"), ".jpg"); err == nil {
		t.Error("zápis bez nastaveného adresáře má selhat")
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
