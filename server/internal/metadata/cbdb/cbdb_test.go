package cbdb

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Výřez ze skutečné stránky s výsledky: každá kniha má dva odkazy – jeden
// s hodnocením v procentech, druhý s názvem – a odkazy jsou relativní
// bez úvodního lomítka.
const searchHTML = `<html><body>
<div id="search_result_box_books" class="search_result_box">
 <div class="search_graphic">
  <div class="search_graphic_box">
   <a href="kniha-975-valka-s-mloky-valka-s-mloky" class="search_graphic_box_img">
     <span class="book_item_rating book_item_rating_0">87%</span>
     <img src="/books/valka-s-mloky-975.jpg" alt="Válka s Mloky" />
   </a>
   <div class="search_graphic_box_content">
     <a href="kniha-975-valka-s-mloky-valka-s-mloky">Válka s Mloky</a>
     <br /><div> (Válka s Mloky)</div>
     <span class="search_author_link">Karel Čapek</span>
   </div>
  </div>
  <div class="search_graphic_box">
   <a href="kniha-96180-valka-s-mloky-valka-s-mloky" class="search_graphic_box_img">
     <span class="book_item_rating">92%</span>
   </a>
   <div class="search_graphic_box_content">
     <a href="kniha-96180-valka-s-mloky-valka-s-mloky">Válka s Mloky</a>
     <span class="search_author_link">Karel Čapek</span>,
     <span class="search_author_link">Pavel Kohout</span>
   </div>
  </div>
  <div class="search_graphic_box">
   <div class="search_graphic_box_content">
     <a href="kniha-12778-valka-s-mloky-doktora-jarose">Válka s mloky doktora Jaroše</a>
   </div>
  </div>
 </div>
</div>
<a href="autor-66-karel-capek">Karel Čapek</a>
<a href="/o-projektu">O projektu</a>
</body></html>`

func TestParseSearchResults(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(searchHTML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	results := parseSearchResults(doc)
	if len(results) != 3 {
		t.Fatalf("výsledků = %d, chtěny 3: %+v", len(results), results)
	}

	first := results[0]
	if first.ID != 975 {
		t.Errorf("ID = %d, chtěno 975", first.ID)
	}
	if first.Title != "Válka s Mloky" {
		t.Errorf("title = %q – procento se nemá brát jako název", first.Title)
	}
	if want := baseURL + "/kniha-975-valka-s-mloky-valka-s-mloky"; first.URL != want {
		t.Errorf("URL = %q, chtěno %q", first.URL, want)
	}
	if first.Source != providerName {
		t.Errorf("source = %q", first.Source)
	}
	if first.Author != "Karel Čapek" {
		t.Errorf("author = %q – bez autora nejde ze stejných názvů vybrat", first.Author)
	}
	// Víc autorů se spojí čárkou.
	if results[1].Author != "Karel Čapek, Pavel Kohout" {
		t.Errorf("author druhého = %q", results[1].Author)
	}
	// Kniha bez uvedeného autora projde dál, jen s prázdným polem.
	if results[2].Title != "Válka s mloky doktora Jaroše" || results[2].Author != "" {
		t.Errorf("třetí = %+v", results[2])
	}

	// Každá kniha jen jednou, i když na ni vede víc odkazů.
	seen := map[int]bool{}
	for _, r := range results {
		if seen[r.ID] {
			t.Errorf("kniha %d je ve výsledcích dvakrát", r.ID)
		}
		seen[r.ID] = true
	}
}

// Výřez z detailu knihy – h1, odkaz na autora, anotace a obálka v og:image.
const bookHTML = `<html><head>
<meta property="og:image" content="https://www.cbdb.cz/img.php?id=975&amp;type=book&amp;size=preview" />
</head><body>
<h1>Válka s Mloky</h1>
<a href="autor-66-karel-capek">Karel Čapek</a>
<span class="rating_number">87 %</span>
<div class="book_description" onclick="book_more_annotation();">
  O čem je kniha Válka s Mloky? Autor ve svém slavném románu popisuje
  nezadržitelnou invazi nepřátelské síly.
</div>
</body></html>`

func TestParseBookPage(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(bookHTML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	meta := parseBookPage(doc)

	if meta.Title != "Válka s Mloky" {
		t.Errorf("title = %q", meta.Title)
	}
	if meta.Author != "Karel Čapek" {
		t.Errorf("author = %q", meta.Author)
	}
	if meta.AuthorID != 66 {
		t.Errorf("author_id = %d, chtěno 66", meta.AuthorID)
	}
	// SEO otázka na začátku anotace se odřezává.
	if strings.HasPrefix(meta.Description, "O čem je kniha") {
		t.Errorf("description si nese SEO úvod: %q", meta.Description)
	}
	if !strings.HasPrefix(meta.Description, "Autor ve svém slavném románu") {
		t.Errorf("description = %q", meta.Description)
	}
	if meta.Rating != 87 {
		t.Errorf("rating = %d, chtěno 87", meta.Rating)
	}
	if !strings.Contains(meta.CoverURL, "id=975") {
		t.Errorf("cover_url = %q", meta.CoverURL)
	}
}

func TestSupports(t *testing.T) {
	c := NewClient()
	tests := map[string]bool{
		"https://www.cbdb.cz/kniha-975-valka-s-mloky": true,
		"https://cbdb.cz/kniha-975":                   true,
		"https://www.databazeknih.cz/prehled-knihy/x": false,
		"https://cbdb.cz.zlo.net/kniha-1":             false,
	}
	for rawURL, want := range tests {
		if got := c.Supports(rawURL); got != want {
			t.Errorf("Supports(%q) = %v, chtěno %v", rawURL, got, want)
		}
	}
}
