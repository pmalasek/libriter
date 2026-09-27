package scanner

import (
	"context"
	"testing"
)

func TestTagLanguage(t *testing.T) {
	tests := []struct {
		raw  map[string]interface{}
		want string
	}{
		{map[string]interface{}{"TLAN": "cze"}, "cs"},              // ID3v2.3/2.4
		{map[string]interface{}{"TLA": "eng"}, "en"},               // ID3v2.2
		{map[string]interface{}{"language": "German"}, "de"},       // Vorbis/FLAC
		{map[string]interface{}{"LANGUAGE": []string{"sk"}}, "sk"}, // MP4
		{map[string]interface{}{"TLAN": "xxx", "language": "fr"}, "fr"},
		{map[string]interface{}{"TIT2": "Název"}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		if got := tagLanguage(tt.raw); got != tt.want {
			t.Errorf("tagLanguage(%v) = %q, chtěno %q", tt.raw, got, tt.want)
		}
	}
}

func TestBookLanguage(t *testing.T) {
	store, root := newRepairEnv(t)
	s := New(root, "", store)
	ctx := context.Background()

	if got := s.bookLanguage(ctx, "en"); got != "en" {
		t.Errorf("jazyk z tagu = %q, chtěno en", got)
	}
	// Kód mimo číselník ani prázdný tag nesmí projít – nastoupí výchozí.
	if got := s.bookLanguage(ctx, "xx"); got != FallbackLanguage {
		t.Errorf("neznámý jazyk = %q, chtěno %s", got, FallbackLanguage)
	}

	s.SetDefaultLanguage(func(context.Context) string { return "de" })
	if got := s.bookLanguage(ctx, ""); got != "de" {
		t.Errorf("bez tagu = %q, chtěno de (nastavení)", got)
	}
	s.SetDefaultLanguage(func(context.Context) string { return "" })
	if got := s.DefaultLanguage(ctx); got != FallbackLanguage {
		t.Errorf("prázdné nastavení = %q, chtěno %s", got, FallbackLanguage)
	}
}
