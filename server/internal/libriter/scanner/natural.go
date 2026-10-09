package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// naturalLess porovnává názvy souborů tak, jak je čte člověk: běhy číslic
// jako čísla ("2" < "10"), zbytek bez ohledu na velikost písmen. Lexikální
// řazení by "Kapitola 10" zařadilo před "Kapitola 2".
//
// Přípona se porovnává až nakonec, aby do porovnání názvů nemluvila: bez toho
// by se "a.mp3" porovnávalo jako "a.mp" a skončilo až za "a1.mp3".
func naturalLess(a, b string) bool {
	sa, sb := stem(a), stem(b)
	if sa == sb {
		return a < b
	}
	return naturalLessRaw(sa, sb)
}

// stem vrátí název souboru bez přípony.
func stem(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func naturalLessRaw(a, b string) bool {
	for a != "" && b != "" {
		ca, ra := chunk(a)
		cb, rb := chunk(b)

		if isDigits(ca) && isDigits(cb) {
			ta, tb := strings.TrimLeft(ca, "0"), strings.TrimLeft(cb, "0")
			if len(ta) != len(tb) {
				return len(ta) < len(tb)
			}
			if ta != tb {
				return ta < tb
			}
			// Stejná hodnota, jiný počet nul: kratší zápis ("1") před delším ("01").
			if len(ca) != len(cb) {
				return len(ca) < len(cb)
			}
		} else if la, lb := strings.ToLower(ca), strings.ToLower(cb); la != lb {
			return la < lb
		}
		a, b = ra, rb
	}
	return len(a) < len(b)
}

// chunk odtrhne úvodní běh číslic, nebo úvodní běh ne-číslic.
func chunk(s string) (head, rest string) {
	digits := isDigit(s[0])
	i := 1
	for i < len(s) && isDigit(s[i]) == digits {
		i++
	}
	return s[:i], s[i:]
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isDigits(s string) bool { return s != "" && isDigit(s[0]) }

// naturalRank vrátí pořadí (od 1) souboru name mezi audio soubory adresáře
// dir v přirozeném řazení. Vrací 0, když adresář nejde přečíst nebo soubor
// v něm není – volající pak sáhne po další pozici v řadě.
func naturalRank(dir, name string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && IsAudioFile(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.SliceStable(names, func(i, j int) bool { return naturalLess(names[i], names[j]) })

	for i, n := range names {
		if n == name {
			return i + 1
		}
	}
	return 0
}
