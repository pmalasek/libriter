package databazeknih

import (
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"libriter/internal/metadata"
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

// Rok vydání se bere jen ze samostatného čísla v infoboxu. JSON-LD skript má
// dateModified s aktuálním datem a popis i patička obsahují jiné letopočty –
// nic z toho není rok vydání.
func TestParseBookPageYear(t *testing.T) {
	const page = `<html><head>
	<script type="application/ld+json">{"@type":"Book","name":"Aristokratka ve varu","dateModified":"2026-09-01"}</script>
	</head><body>
	<h1>Aristokratka ve varu</h1>
	<div class="bookRightDiv"><div class="lora lineHeightMid">
	  <a href='/zanry/romany-12'>Romány</a><br />
	  2013
	  <span class='pozn'>,</span> <a href='/nakladatelstvi/druhe-mesto-3940'>Druhé město</a>
	</div></div>
	<p>Pokračování knihy z roku 2012, která se odehrává v roce 1966.</p>
	<footer><p>© 2008 - 2026 Databazeknih.cz</p></footer>
	</body></html>`

	meta, err := parseBookPage(strings.NewReader(page))
	if err != nil {
		t.Fatalf("parseBookPage: %v", err)
	}
	if meta.Year != 2013 {
		t.Errorf("year = %d, chtěno 2013", meta.Year)
	}
	if meta.Publisher != "Druhé město" {
		t.Errorf("publisher = %q", meta.Publisher)
	}
}

func TestParseOriginalEdition(t *testing.T) {
	tests := []struct {
		name      string
		fragment  string
		wantTitle string
		wantYear  int
	}{
		{
			"překlad s rokem",
			`<div class='book-details__row'><dt>Originální název</dt>
			 <dd>The Hitchhiker&#039;s Guide to the Galaxy, 1979</dd></div>
			 <div class='book-details__row'><dt>Počet stran</dt><dd>146</dd></div>`,
			"The Hitchhiker's Guide to the Galaxy", 1979,
		},
		{
			"bez roku",
			`<dt>Originální název</dt><dd>Solaris</dd>`,
			"Solaris", 0,
		},
		{
			"česká kniha bez řádku",
			`<dt>Počet stran</dt><dd>240</dd><dt>Jazyk vydání</dt><dd>český</dd>`,
			"", 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, year := parseOriginalEdition(strings.NewReader(tt.fragment))
			if title != tt.wantTitle || year != tt.wantYear {
				t.Errorf("= (%q, %d), chtěno (%q, %d)", title, year, tt.wantTitle, tt.wantYear)
			}
		})
	}
}

// Sérii a díl bere parser z pruhu nad názvem knihy. Šipky na sousední díly
// jsou ve stejném bloku a nesmí se do názvu ani do čísla dílu připlést.
func TestParseBookPageSeries(t *testing.T) {
	tests := []struct {
		name         string
		block        string
		wantSeries   string
		wantPosition int
	}{
		{
			"série s dílem",
			`<div class="lora book_detail_serie_info">
			   <p class="inline"><a class="odright_pet" href='/serie/stoparuv-pruvodce-galaxii-135?lang=cz'
			      title='Stopařův průvodce Galaxií'>Stopařův průvodce Galaxií</a> série</p>
			   <span class="nowrap">
			     <a class="odleft_pet arrow" href="/prehled-knihy/omnibus-259682">&lt;</a>
			     <span class="odright_pet odleft_pet">1. díl</span>
			     <a class="arrow" href="/prehled-knihy/restaurant-na-konci-vesmiru-3997">&gt;</a>
			   </span>
			 </div>`,
			"Stopařův průvodce Galaxií", 1,
		},
		{
			"série bez číslování dílů",
			`<div class="lora book_detail_serie_info">
			   <p class="inline"><a href='/serie/povidky-999'>Povídky</a> série</p>
			 </div>`,
			"Povídky", 0,
		},
		{
			"kniha mimo sérii",
			`<div class="orangeBoxLight"></div>`,
			"", 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := `<html><body>` + tt.block + `<h1>Stopařův průvodce Galaxií</h1></body></html>`

			meta, err := parseBookPage(strings.NewReader(page))
			if err != nil {
				t.Fatalf("parseBookPage: %v", err)
			}
			if meta.Series != tt.wantSeries {
				t.Errorf("series = %q, chtěno %q", meta.Series, tt.wantSeries)
			}
			if meta.SeriesPosition != tt.wantPosition {
				t.Errorf("series position = %d, chtěno %d", meta.SeriesPosition, tt.wantPosition)
			}
		})
	}
}

// Kniha může mít víc autorů; pseudonym se bere z textu odkazu, protože odkaz
// sám míří na občanské jméno. Odkazy na autory mimo řádek pod názvem
// („Další knihy autora“) do seznamu nepatří.
func TestParseBookPageAuthors(t *testing.T) {
	const page = `<html><body>
	<h1>Aréna</h1>
	<p class="lora oddown_midl"><span>
	  <span class="author">
	    <a href="/autori/leos-kysa-12208">František Kotleta</a> <span class='pozn_light'>(p)</span>,
	  </span>
	  <span class="author">
	    <a href="/autori/kristyna-snegonova-11744">Kristýna Sněgoňová</a>
	  </span>
	</span></p>
	<div class="other_books"><a href="/autori/leos-kysa-12208">Leoš Kyša</a></div>
	</body></html>`

	meta, err := parseBookPage(strings.NewReader(page))
	if err != nil {
		t.Fatalf("parseBookPage: %v", err)
	}

	if meta.Author != "František Kotleta, Kristýna Sněgoňová" {
		t.Errorf("author = %q", meta.Author)
	}
	if meta.AuthorID != 12208 {
		t.Errorf("author id = %d, chtěno 12208", meta.AuthorID)
	}

	want := []string{"František Kotleta", "Kristýna Sněgoňová"}
	var got []string
	for _, a := range meta.Authors {
		got = append(got, a.Name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("authors = %v, chtěno %v", got, want)
	}
	if meta.Authors[1].FirstName != "Kristýna" || meta.Authors[1].LastName != "Sněgoňová" {
		t.Errorf("rozdělení jména = %+v", meta.Authors[1])
	}
}

// Autora, který vydává pod pseudonymem, vede web pod občanským jménem
// a pseudonym má v řádku pod ním. Klient podle seznamu pozná, že jméno
// ve své knihovně přepisovat nemá.
func TestParseAuthorPagePseudonyms(t *testing.T) {
	const page = `<html><body>
	<h1>Frode Sander Øien</h1>
	<h2 class="norm">
	  <a href="/vydane-knihy-pseudonym/samuel-bjork-2596">Samuel Bjørk</a>
	  <span class='gray'> · pseudonym</span>
	</h2>
	</body></html>`

	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	meta := parseAuthorPage(doc)
	if meta.Name != "Frode Sander Øien" {
		t.Errorf("name = %q", meta.Name)
	}
	if !reflect.DeepEqual(meta.Pseudonyms, []metadata.BookAuthor{{Name: "Samuel Bjørk"}}) {
		t.Errorf("pseudonyms = %v", meta.Pseudonyms)
	}
}

// Hledání autorů vypisuje jméno, pod kterým autor vydává, a za ním značku
// „(pseudonym)“ – ta patří do poznámky, ne do jména.
func TestParseAuthorSearchResultsPseudonym(t *testing.T) {
	const page = `<html><body>
	<div class='autbox'>
	  <a href='/autori/frode-sander-ien-83556'>
	    <div class='circle_aut' title='Frode Sander Øien'></div>
	    Samuel Bjørk
	    <span class="odleft_pet pozn">(pseudonym)</span>
	  </a><br /><span class='pozn_light'>1969</span>
	</div>
	<div class='autbox'>
	  <a href='/autori/barbara-samuel-54060'>Barbara Samuel</a>
	</div>
	</body></html>`

	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	results := parseAuthorSearchResults(doc)
	if len(results) != 2 {
		t.Fatalf("počet výsledků = %d, chtěno 2", len(results))
	}
	if results[0].Name != "Samuel Bjørk" || results[0].Note != "pseudonym" {
		t.Errorf("první výsledek = %q / %q", results[0].Name, results[0].Note)
	}
	if results[1].Name != "Barbara Samuel" || results[1].Note != "" {
		t.Errorf("druhý výsledek = %q / %q", results[1].Name, results[1].Note)
	}
}

// Výpis vydaných knih autora je jediná cesta ke knize s obyčejným názvem –
// vyhledávání webu prochází jen názvy a „Ostrov“ jich má padesát stránek.
func TestParseAuthorBooks(t *testing.T) {
	const page = `<html><body>
	<h3 class='midlRowHeight oddown'>
	  <a href='/prehled-knihy/holger-munch-a-mia-krugerova-ostrov-521083' title='Ostrov'>Ostrov</a>
	  <span class='pozn odl'>2023 (1. vydání)</span>
	</h3>
	<h3 class='midlRowHeight oddown'>
	  <a href='/prehled-knihy/sova-495064' title='Sova'>Sova</a>
	  <span class='pozn odl'>2021</span>
	</h3>
	<h3>Nadpis bez knihy</h3>
	</body></html>`

	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	books := parseAuthorBooks(doc)
	if len(books) != 2 {
		t.Fatalf("počet knih = %d, chtěno 2", len(books))
	}
	if books[0].Title != "Ostrov" || books[0].ID != 521083 || books[0].Year != 2023 {
		t.Errorf("první kniha = %+v", books[0])
	}
	if books[1].Year != 2021 {
		t.Errorf("rok druhé knihy = %d", books[1].Year)
	}
}

func TestTitleMatches(t *testing.T) {
	tests := []struct {
		title, wanted string
		want          bool
	}{
		{"Ostrov", "Ostrov", true},
		{"ostrov", "Ostrov", true},
		// Některá vydání mají v názvu i sérii.
		{"Atomové šelmy: Aréna", "Aréna", true},
		{"Ostrov", "Ostrov pokladů", true},
		{"Ostrov pokladů", "Sova", false},
		{"", "Ostrov", false},
	}

	for _, tt := range tests {
		if got := titleMatches(tt.title, tt.wanted); got != tt.want {
			t.Errorf("titleMatches(%q, %q) = %v", tt.title, tt.wanted, got)
		}
	}
}

func TestBooksURLFor(t *testing.T) {
	got, ok := booksURLFor("https://www.databazeknih.cz/autori/frode-sander-ien-83556")
	if !ok || got != "https://www.databazeknih.cz/vydane-knihy/frode-sander-ien-83556" {
		t.Errorf("booksURLFor = %q, %v", got, ok)
	}
	if _, ok := booksURLFor("https://www.databazeknih.cz/prehled-knihy/ostrov-12234"); ok {
		t.Error("booksURLFor přijal adresu, která není autor")
	}
}
