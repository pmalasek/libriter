package model

import "strings"

// iso6392 převádí třípísmenné kódy ISO 639-2 na ISO 639-1. ID3 tag TLAN
// podle normy nese právě ISO 639-2 – a to v bibliografické (cze, ger)
// i terminologické (ces, deu) podobě, proto jsou v mapě obě.
var iso6392 = map[string]string{
	"cze": "cs", "ces": "cs",
	"slo": "sk", "slk": "sk",
	"eng": "en",
	"ger": "de", "deu": "de",
	"fre": "fr", "fra": "fr",
	"spa": "es",
	"ita": "it",
	"pol": "pl",
	"rus": "ru",
	"ukr": "uk",
	"dut": "nl", "nld": "nl",
	"swe": "sv",
	"dan": "da",
	"nor": "no", "nob": "nb", "nno": "nn",
	"fin": "fi",
	"hun": "hu",
	"por": "pt",
	"tur": "tr",
	"jpn": "ja",
	"chi": "zh", "zho": "zh",
	"hrv": "hr",
	"slv": "sl",
	"srp": "sr",
	"bul": "bg",
	"rum": "ro", "ron": "ro",
	"gre": "el", "ell": "el",
	"lat": "la",
}

// englishLanguageNames převádí anglické názvy jazyků, jak je uvádějí
// zahraniční zdroje metadat („english“, „German“), na ISO 639-1.
var englishLanguageNames = map[string]string{
	"english":          "en",
	"german":           "de",
	"french":           "fr",
	"spanish":          "es",
	"italian":          "it",
	"czech":            "cs",
	"slovak":           "sk",
	"polish":           "pl",
	"dutch":            "nl",
	"swedish":          "sv",
	"danish":           "da",
	"norwegian":        "no",
	"finnish":          "fi",
	"portuguese":       "pt",
	"russian":          "ru",
	"ukrainian":        "uk",
	"hungarian":        "hu",
	"turkish":          "tr",
	"japanese":         "ja",
	"chinese":          "zh",
	"mandarin_chinese": "zh",
}

// LanguageFromEnglishName vrátí kód ISO 639-1 pro anglický název jazyka;
// neznámý název vrátí prázdný řetězec.
func LanguageFromEnglishName(name string) string {
	return englishLanguageNames[strings.ToLower(strings.TrimSpace(name))]
}

// LanguageFromTag převede jazyk z audio tagu na kód ISO 639-1. Tagy ho
// zapisují různě – "cze", "ces", "cs", "cs-CZ", "Czech" – a někdy víc jazyků
// najednou ("eng/ger"); bere se první. Nerozpoznaný zápis vrátí prázdný
// řetězec. Jestli kód opravdu je v číselníku jazyků, musí ověřit volající.
func LanguageFromTag(value string) string {
	first := strings.FieldsFunc(value, func(r rune) bool {
		return r == '/' || r == ';' || r == ',' || r == '\x00'
	})
	if len(first) == 0 {
		return ""
	}
	v := strings.ToLower(strings.TrimSpace(first[0]))
	// Značka jazyka s regionem ("cs-CZ", "en_US").
	if i := strings.IndexAny(v, "-_"); i > 0 {
		v = v[:i]
	}

	switch {
	case len(v) == 2 && isLetters(v):
		return v
	case len(v) == 3:
		return iso6392[v]
	default:
		return LanguageFromEnglishName(v)
	}
}

func isLetters(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}
