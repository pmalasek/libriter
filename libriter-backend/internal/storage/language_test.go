package storage

import (
	"context"
	"testing"
)

func TestLanguagesSeeded(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	list, err := store.ListLanguages(ctx)
	if err != nil {
		t.Fatalf("ListLanguages: %v", err)
	}
	// ISO 639-1 má 183 platných kódů.
	if len(list) != 183 {
		t.Errorf("počet jazyků = %d, chtěno 183", len(list))
	}

	for _, code := range []string{"cs", "sk", "en", "de", "zh"} {
		ok, err := store.LanguageExists(ctx, code)
		if err != nil {
			t.Fatalf("LanguageExists(%q): %v", code, err)
		}
		if !ok {
			t.Errorf("jazyk %q v číselníku chybí", code)
		}
	}

	for _, code := range []string{"xx", "CS", "", "cze"} {
		ok, err := store.LanguageExists(ctx, code)
		if err != nil {
			t.Fatalf("LanguageExists(%q): %v", code, err)
		}
		if ok {
			t.Errorf("LanguageExists(%q) = true, chtěno false", code)
		}
	}
}
