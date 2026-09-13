package databazeknih

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Výřez ze skutečné stránky s výsledky: obálka odkazuje na tutéž adresu jako
// název a pod názvem je <span class="pozn"> s rokem a autorem.
const searchHTML = `<html><body>
<p class='new'>
  <a href="/prehled-knihy/valka-s-mloky-160">
    <picture><img src="/img/books/valka-s-mloky-160.jpg" alt="Obálka knihy Válka s Mloky" /></picture>
  </a>
  <img src='img/content/squares/red.gif' title='86%' class='odr' />
  <a class='new' type='book' href='/prehled-knihy/valka-s-mloky-160'>Válka s Mloky</a>
  <br /><span class='pozn'>
    2007,
    Karel Čapek
  </span>
</p>
<p class='new'>
  <a class='new' type='book' href='/prehled-knihy/valka-s-mloky-5898'>Válka s mloky</a>
  <br /><span class='pozn'>1981, Karel Čapek, Josef Čapek</span>
</p>
<p class='new'>
  <a class='new' type='book' href='/prehled-knihy/bez-roku-1234'>Kniha bez roku</a>
  <br /><span class='pozn'>Jan Novák</span>
</p>
</body></html>`

func TestParseSearchResults(t *testing.T) {
	results, err := parseSearchResults(strings.NewReader(searchHTML))
	if err != nil {
		t.Fatalf("parseSearchResults: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("výsledků = %d, chtěny 3: %+v", len(results), results)
	}

	first := results[0]
	if first.ID != 160 || first.Title != "Válka s Mloky" {
		t.Errorf("první = %+v", first)
	}
	// Bez autora nejde ze stejných názvů vybrat ten správný.
	if first.Author != "Karel Čapek" {
		t.Errorf("author = %q, chtěno %q", first.Author, "Karel Čapek")
	}
	if first.Year != 2007 {
		t.Errorf("year = %d, chtěno 2007", first.Year)
	}
	if want := baseURL + "/prehled-knihy/valka-s-mloky-160"; first.URL != want {
		t.Errorf("URL = %q, chtěno %q", first.URL, want)
	}

	// Víc autorů zůstane oddělených čárkou.
	if results[1].Author != "Karel Čapek, Josef Čapek" || results[1].Year != 1981 {
		t.Errorf("druhý = %+v", results[1])
	}

	// Poznámka bez roku je celá autorem.
	if results[2].Author != "Jan Novák" || results[2].Year != 0 {
		t.Errorf("třetí = %+v", results[2])
	}
}

// Když se rozložení stránky změní, nesmí hledání přestat fungovat úplně –
// jen přijde o autora a rok.
func TestParseSearchResultsFallback(t *testing.T) {
	const changedHTML = `<html><body><div class="neco-noveho">
	  <a href="/prehled-knihy/valka-s-mloky-160">Válka s Mloky</a>
	  <a href="/prehled-knihy/valka-s-mloky-160">Válka s Mloky</a>
	  <a href="/uzivatele/nekdo">Nějaký uživatel</a>
	</div></body></html>`

	results, err := parseSearchResults(strings.NewReader(changedHTML))
	if err != nil {
		t.Fatalf("parseSearchResults: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("výsledků = %d, chtěn 1: %+v", len(results), results)
	}
	if results[0].ID != 160 || results[0].Title != "Válka s Mloky" {
		t.Errorf("výsledek = %+v", results[0])
	}
}

func TestParseNote(t *testing.T) {
	tests := []struct {
		note       string
		wantYear   int
		wantAuthor string
	}{
		{"2007, Karel Čapek", 2007, "Karel Čapek"},
		{"1981, Karel Čapek, Josef Čapek", 1981, "Karel Čapek, Josef Čapek"},
		{"Karel Čapek", 0, "Karel Čapek"},
		{"2007", 2007, ""},
		{"", 0, ""},
		// Čtyřmístné číslo jinde než na začátku je součást jména, ne rok.
		{"Agent 2000", 0, "Agent 2000"},
	}

	for _, tt := range tests {
		year, author := parseNote(tt.note)
		if year != tt.wantYear || author != tt.wantAuthor {
			t.Errorf("parseNote(%q) = (%d, %q), chtěno (%d, %q)",
				tt.note, year, author, tt.wantYear, tt.wantAuthor)
		}
	}
}

func TestSupports(t *testing.T) {
	c := NewClient()
	tests := map[string]bool{
		"https://www.databazeknih.cz/prehled-knihy/valka-s-mloky-160": true,
		"https://databazeknih.cz/autori/karel-capek-101":              true,
		"https://www.cbdb.cz/kniha-975":                               false,
		"https://databazeknih.cz.zlo.net/x":                           false,
	}
	for rawURL, want := range tests {
		if got := c.Supports(rawURL); got != want {
			t.Errorf("Supports(%q) = %v, chtěno %v", rawURL, got, want)
		}
	}
}

// Popis knihy je na stránce celý, ale je k němu přilepený ovládací odkaz
// "… celý text", který text jen rozbaluje. Do anotace nepatří.
func TestExtractDescriptionStripsReadMore(t *testing.T) {
	const page = `<html><body>
	<h2>O knize</h2>
	<p class='new2 odtop'><span>Autor ve svém slavném románu popisuje nezadržitelnou invazi
	nepřátelské síly. V knize uvedeno chybné ISBN 80-86201-39-2</span><a href='#' id='160'
	class='show_hide_more ll' bid='160'>... celý text</a></p>
	</body></html>`

	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	got := extractDescription(doc)
	if strings.Contains(got, "celý text") {
		t.Errorf("v popisu zůstal ovládací odkaz: %q", got)
	}
	if !strings.HasSuffix(got, "ISBN 80-86201-39-2") {
		t.Errorf("popis = %q", got)
	}
}

func TestCleanReadMore(t *testing.T) {
	tests := map[string]string{
		"Popis knihy. ... celý text": "Popis knihy.",
		"Popis knihy. … celý text":   "Popis knihy.",
		"Popis knihy.":               "Popis knihy.",
		// "celý text" uprostřed věty se nesmí odstranit.
		"Nečetl jsem celý text knihy.": "Nečetl jsem celý text knihy.",
	}
	for in, want := range tests {
		if got := cleanReadMore(in); got != want {
			t.Errorf("cleanReadMore(%q) = %q, chtěno %q", in, got, want)
		}
	}
}

// Přehled autora nese jen zkrácený životopis; celý je na /zivotopis/.
func TestBioURLFor(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"https://www.databazeknih.cz/autori/karel-capek-101", baseURL + "/zivotopis/karel-capek-101", true},
		{"https://databazeknih.cz/autori/karel-capek-101", baseURL + "/zivotopis/karel-capek-101", true},
		{"https://www.databazeknih.cz/prehled-knihy/valka-160", "", false},
		{"https://www.databazeknih.cz/autori/", "", false},
		{"https://www.databazeknih.cz/autori/a/b", "", false},
	}
	for _, tt := range tests {
		got, ok := bioURLFor(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("bioURLFor(%q) = (%q, %v), chtěno (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}
