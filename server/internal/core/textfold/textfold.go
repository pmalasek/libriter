// Package textfold skládá klíč pro hledání: bez diakritiky, malými písmeny,
// mezery sjednocené. Odpovídá foldName v core-shared/src/format.ts, aby
// hledání na serveru našlo totéž, co dřív hledání v prohlížeči.
package textfold

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Písmena, která nejsou jen základ + diakritika, takže je NFD nerozloží.
var foldedLetters = map[rune]string{'ł': "l", 'ø': "o", 'đ': "d", 'ß': "ss", 'æ': "ae", 'œ': "oe"}

// tableSize pokrývá latinku, řečtinu i cyrilici; pro ně je rozklad
// předpočítaný a Fold nealokuje.
const tableSize = 0x2000

// table[r] je složená podoba znaku r (malé písmeno bez diakritiky).
var table = func() *[tableSize]string {
	var t [tableSize]string
	for r := rune(utf8.RuneSelf); r < tableSize; r++ {
		t[r] = foldRune(r)
	}
	return &t
}()

func foldRune(r rune) string {
	r = unicode.ToLower(r)
	if repl, ok := foldedLetters[r]; ok {
		return repl
	}
	var b strings.Builder
	for _, d := range norm.NFD.String(string(r)) {
		if !unicode.Is(unicode.Mn, d) {
			b.WriteRune(d)
		}
	}
	return b.String()
}

// Fold vrátí klíč pro porovnání textu bez ohledu na diakritiku a velikost
// písmen. SQLite ji volá pro každý řádek prohledávané tabulky, proto se
// vyhýbá alokacím, kde to jde.
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false // předchozí zapsaný znak byla mezera
	write := func(r rune) {
		if unicode.IsSpace(r) {
			if b.Len() > 0 {
				space = true
			}
			return
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}

	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			// ASCII – rychlá cesta bez normalizace.
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			write(rune(c))
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		folded := ""
		if r < tableSize {
			folded = table[r]
		} else {
			folded = foldRune(r)
		}
		for _, x := range folded {
			write(x)
		}
	}
	return b.String()
}

// LikePattern složí vzor pro `fold(sloupec) LIKE ? ESCAPE '\'` – hledaný
// text kdekoli v hodnotě. Zástupné znaky z dotazu se berou doslova.
func LikePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(Fold(q)) + "%"
}
