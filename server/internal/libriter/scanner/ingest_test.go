package scanner

import "testing"

// Track čísla se na každém disku vydání opakují od 1, pozice se proto musí lišit.
func TestTagPosition(t *testing.T) {
	tests := []struct {
		name  string
		track int
		disc  int
		want  int
	}{
		{name: "bez tagů", track: 0, disc: 0, want: 0},
		{name: "jen track", track: 7, disc: 0, want: 7},
		{name: "první disk", track: 7, disc: 1, want: 7},
		{name: "druhý disk", track: 1, disc: 2, want: 1001},
		{name: "pátý disk", track: 11, disc: 5, want: 4011},
		{name: "disk bez tracku", track: 0, disc: 3, want: 0},
	}
	for _, tc := range tests {
		got := tagPosition(&AudioMeta{TrackNumber: tc.track, DiscNumber: tc.disc})
		if got != tc.want {
			t.Errorf("%s: tagPosition = %d, chtěno %d", tc.name, got, tc.want)
		}
	}
}

func TestSameBookLocation(t *testing.T) {
	tests := []struct {
		name    string
		bookDir string
		relDir  string
		want    bool
	}{
		{
			name:    "stejný adresář",
			bookDir: "Cole, Daniel/Loutkář",
			relDir:  "Cole, Daniel/Loutkář",
			want:    true,
		},
		{
			name:    "disk v podadresáři knihy",
			bookDir: "Cole, Daniel/Loutkář",
			relDir:  "Cole, Daniel/Loutkář/CD2",
			want:    true,
		},
		{
			name:    "kniha vznikla z prvního disku",
			bookDir: "Cole, Daniel/Loutkář/CD1",
			relDir:  "Cole, Daniel/Loutkář/CD2",
			want:    true,
		},
		{
			name:    "dvě vydání v adresáři autora",
			bookDir: "Doyle, Arthur Conan/Podpis čtyř",
			relDir:  "Doyle, Arthur Conan/Sherlock Holmes 65x/Podpis čtyř",
			want:    false,
		},
		{
			name:    "sousedící knihy u jednoho autora",
			bookDir: "Doyle, Arthur Conan/Podpis čtyř",
			relDir:  "Doyle, Arthur Conan/Podpis čtyř (2012)",
			want:    false,
		},
		{
			name:    "sousedící adresáře v AUDIO_ROOT",
			bookDir: "Podpis čtyř",
			relDir:  "Podpis čtyř – nové vydání",
			want:    false,
		},
	}
	for _, tc := range tests {
		if got := sameBookLocation(tc.bookDir, tc.relDir); got != tc.want {
			t.Errorf("%s: sameBookLocation(%q, %q) = %v, chtěno %v",
				tc.name, tc.bookDir, tc.relDir, got, tc.want)
		}
	}
}
