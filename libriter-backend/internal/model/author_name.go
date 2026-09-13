// internal/model/author_name.go
//
// Jméno autora rozdělené na křestní / prostřední / příjmení a parsování
// zapsaných jmen z audio tagů, názvů adresářů a API požadavků.

package model

import "strings"

// AuthorName je jméno autora rozdělené na části.
// Prostřední jméno (a u jednoslovných jmen i křestní) může být prázdné.
type AuthorName struct {
	First  string
	Middle string
	Last   string
}

// UnknownAuthor je zástupné jméno pro soubory, kde autor není uveden.
var UnknownAuthor = AuthorName{Last: "Neznámý autor"}

// Full vrátí celé jméno ve tvaru "Křestní Prostřední Příjmení".
func (n AuthorName) Full() string {
	parts := make([]string, 0, 3)
	for _, p := range []string{n.First, n.Middle, n.Last} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}

// IsEmpty vrátí true, pokud jméno neobsahuje žádnou část.
func (n AuthorName) IsEmpty() bool {
	return n.Full() == ""
}

// Předložky a předpony víceslovných příjmení ("van Beethoven", "de Gaulle").
// Slouží k rozpoznání tvaru "Příjmení, Křestní" od seznamu autorů.
var nameParticles = map[string]bool{
	"van": true, "von": true, "de": true, "del": true, "della": true,
	"da": true, "di": true, "du": true, "la": true, "le": true,
	"den": true, "der": true, "ten": true, "ter": true,
	"mac": true, "mc": true, "ibn": true, "bin": true, "al": true,
}

// ParseAuthorName rozdělí zapsané jméno na části:
//
//	"Jan Amos Komenský"      → {Jan, Amos, Komenský}
//	"Komenský, Jan Amos"     → {Jan, Amos, Komenský}
//	"Karel Čapek"            → {Karel, "", Čapek}
//	"Homér"                  → {"", "", Homér}
//
// Jednoslovné jméno se ukládá jako příjmení – podle něj se řadí a hledá.
func ParseAuthorName(s string) AuthorName {
	s = collapseSpaces(s)
	if s == "" {
		return AuthorName{}
	}

	// Tvar "Příjmení, Křestní Prostřední"
	if isInvertedName(s) {
		last, given, _ := strings.Cut(s, ",")
		first, middle := cutFirstWord(collapseSpaces(given))
		return AuthorName{First: first, Middle: middle, Last: collapseSpaces(last)}
	}

	// Tvar "Křestní [Prostřední…] Příjmení"
	words := strings.Fields(strings.ReplaceAll(s, ",", " "))
	switch len(words) {
	case 0:
		return AuthorName{}
	case 1:
		return AuthorName{Last: words[0]}
	default:
		return AuthorName{
			First:  words[0],
			Middle: strings.Join(words[1:len(words)-1], " "),
			Last:   words[len(words)-1],
		}
	}
}

// ParseAuthorNames rozdělí hodnotu tagu na jednotlivé autory.
// Duplicity (stejné celé jméno) se vynechávají, pořadí zůstává zachováno.
//
//	"Karel Čapek; Josef Čapek"  → dva autoři
//	"Čapek, Karel"              → jeden autor
func ParseAuthorNames(s string) []AuthorName {
	var (
		out  []AuthorName
		seen = make(map[string]bool)
	)
	for _, piece := range splitAuthorList(s) {
		n := ParseAuthorName(piece)
		full := n.Full()
		if n.IsEmpty() || seen[full] {
			continue
		}
		seen[full] = true
		out = append(out, n)
	}
	return out
}

// JoinAuthorNames vrátí celá jména oddělená čárkou (pro logy a hlášky).
func JoinAuthorNames(names []AuthorName) string {
	full := make([]string, 0, len(names))
	for _, n := range names {
		full = append(full, n.Full())
	}
	return strings.Join(full, ", ")
}

// splitAuthorList rozdělí hodnotu tagu na jednotlivá jména.
//
// Oddělovače: NUL (ID3v2.4 vícehodnotový rámec), ";", "/", "&", "|",
// " a ", " and " a čárka – ta ale jen tehdy, nejde-li o tvar "Příjmení, Křestní".
func splitAuthorList(s string) []string {
	const sep = "\x00"
	replacer := strings.NewReplacer(
		";", sep, "/", sep, "&", sep, "|", sep,
		" a ", sep, " A ", sep, " and ", sep, " AND ", sep,
	)

	var out []string
	for _, piece := range strings.Split(replacer.Replace(s), sep) {
		piece = collapseSpaces(piece)
		if piece == "" {
			continue
		}
		if strings.Contains(piece, ",") && !isInvertedName(piece) {
			for _, sub := range strings.Split(piece, ",") {
				if sub = collapseSpaces(sub); sub != "" {
					out = append(out, sub)
				}
			}
			continue
		}
		out = append(out, piece)
	}
	return out
}

// isInvertedName rozpozná tvar "Příjmení, Křestní [Prostřední]" a odliší ho
// od seznamu autorů oddělených čárkou ("Karel Čapek, Josef Čapek").
//
// Před čárkou smí stát jen samotné příjmení, případně s předložkou
// ("van Beethoven, Ludwig"); za čárkou nejvýše tři slova.
func isInvertedName(s string) bool {
	before, after, ok := strings.Cut(s, ",")
	if !ok || strings.Contains(after, ",") {
		return false
	}

	head, tail := strings.Fields(before), strings.Fields(after)
	if len(tail) == 0 || len(tail) > 3 {
		return false
	}
	switch len(head) {
	case 1:
		return true
	case 2:
		return nameParticles[strings.ToLower(head[0])]
	default:
		return false
	}
}

// cutFirstWord rozdělí text na první slovo a zbytek.
func cutFirstWord(s string) (first, rest string) {
	words := strings.Fields(s)
	if len(words) == 0 {
		return "", ""
	}
	return words[0], strings.Join(words[1:], " ")
}

// collapseSpaces ořízne okraje a sjednotí vnitřní mezery na jednu.
func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
