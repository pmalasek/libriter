package model

import "testing"

func TestLanguageFromTag(t *testing.T) {
	tests := map[string]string{
		"cze":      "cs",
		"ces":      "cs",
		"CS":       "cs",
		"cs-CZ":    "cs",
		"en_US":    "en",
		"ger":      "de",
		"eng/ger":  "en",
		"Czech":    "cs",
		" German ": "de",
		"xyz":      "",
		"":         "",
		"čeština":  "",
		"12":       "",
		"eng\x00":  "en",
	}
	for in, want := range tests {
		if got := LanguageFromTag(in); got != want {
			t.Errorf("LanguageFromTag(%q) = %q, chtěno %q", in, got, want)
		}
	}
}
