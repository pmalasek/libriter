package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.mp3", "2.mp3", true},
		{"2.mp3", "10.mp3", true},
		{"10.mp3", "2.mp3", false},
		{"Kapitola 2.mp3", "Kapitola 10.mp3", true},
		{"cd1-02.mp3", "cd2-01.mp3", true},
		{"cd1-10.mp3", "cd1-9.mp3", false},
		{"1.mp3", "01.mp3", true}, // stejná hodnota, kratší zápis první
		{"01.mp3", "1.mp3", false},
		{"a.mp3", "B.mp3", true}, // bez ohledu na velikost písmen
		{"B.mp3", "a.mp3", false},
		{"Část 2.mp3", "Část 10.mp3", true},
		{"stejne.mp3", "stejne.mp3", false},
		{"a.mp3", "a1.mp3", true}, // kratší předpona první
		{"a1.mp3", "a.mp3", false},
	}
	for _, tc := range tests {
		if got := naturalLess(tc.a, tc.b); got != tc.want {
			t.Errorf("naturalLess(%q, %q) = %v, chtěno %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNaturalRank(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"1.mp3", "10.mp3", "2.mp3", "cover.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "cd2.mp3"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	tests := map[string]int{
		"1.mp3":     1,
		"2.mp3":     2,
		"10.mp3":    3,
		"cover.jpg": 0, // není audio
		"cd2.mp3":   0, // adresář, ne soubor
		"chybi.mp3": 0,
	}
	for name, want := range tests {
		if got := naturalRank(dir, name); got != want {
			t.Errorf("naturalRank(%q) = %d, chtěno %d", name, got, want)
		}
	}

	if got := naturalRank(filepath.Join(dir, "neexistuje"), "1.mp3"); got != 0 {
		t.Errorf("naturalRank v neexistujícím adresáři = %d, chtěno 0", got)
	}
}
