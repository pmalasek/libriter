package metadata

import "testing"

func TestYearFromDate(t *testing.T) {
	tests := map[string]int{
		"1890":           1890,
		"1890-01-09":     1890,
		"9.1. 1890":      1890,
		"9 January 1890": 1890,
		"":               0,
		"neznámo":        0,
		// Nesmyslné roky se zahazují.
		"875":  0,
		"3000": 0,
	}
	for in, want := range tests {
		if got := YearFromDate(in); got != want {
			t.Errorf("YearFromDate(%q) = %d, chtěno %d", in, got, want)
		}
	}
}

func TestYearsFromText(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		birth, death int
	}{
		{"rozmezí s pomlčkou", "Karel Čapek (1890–1938) byl spisovatel.", 1890, 1938},
		{"rozmezí se spojovníkem", "Karel Čapek (1890-1938)", 1890, 1938},
		{"rozmezí s mezerami", "žil 1890 – 1938", 1890, 1938},
		// Bez rozmezí se bere jen první rok; spojovat dva roky z volného
		// textu by hádalo (v životopisu jich bývá spousta).
		{"jen narození", "Narodil se 9.1. 1890 v Malých Svatoňovicích.", 1890, 0},
		{"roky v textu bez rozmezí", "Vydal Krakatit 1924 a Válku s mloky 1936.", 1924, 0},
		{"bez roku", "O autorovi není nic známo.", 0, 0},
		{"obrácené rozmezí se ignoruje", "1938–1890", 1938, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			birth, death := YearsFromText(tt.text)
			if birth != tt.birth || death != tt.death {
				t.Errorf("YearsFromText(%q) = (%d, %d), chtěno (%d, %d)",
					tt.text, birth, death, tt.birth, tt.death)
			}
		})
	}
}
