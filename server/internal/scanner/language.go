package scanner

import "context"

// FallbackLanguage je jazyk knihy, když ho nezná tag ani nastavení.
const FallbackLanguage = "cs"

// SetDefaultLanguage nastaví zdroj výchozího jazyka nových knih (nastavení
// knihovny v administraci). Volá se při startu serveru; scanner se na zdroj
// ptá při každém založení knihy, takže změna nastavení platí hned.
func (s *Scanner) SetDefaultLanguage(fn func(context.Context) string) {
	s.langMu.Lock()
	defer s.langMu.Unlock()
	s.defaultLanguage = fn
}

// DefaultLanguage vrátí výchozí jazyk nových knih.
func (s *Scanner) DefaultLanguage(ctx context.Context) string {
	s.langMu.Lock()
	fn := s.defaultLanguage
	s.langMu.Unlock()

	if fn != nil {
		if lang := fn(ctx); lang != "" {
			return lang
		}
	}
	return FallbackLanguage
}

// bookLanguage vybere jazyk nové knihy: z tagu, pokud je v číselníku jazyků,
// jinak výchozí. Kód mimo číselník by pak nešel v editaci knihy uložit.
func (s *Scanner) bookLanguage(ctx context.Context, tagLang string) string {
	if tagLang != "" {
		ok, err := s.store.LanguageExists(ctx, tagLang)
		if err != nil {
			s.log.Debug("jazyk z tagu se nepodařilo ověřit", "jazyk", tagLang, "err", err)
		}
		if ok {
			return tagLang
		}
	}
	return s.DefaultLanguage(ctx)
}
