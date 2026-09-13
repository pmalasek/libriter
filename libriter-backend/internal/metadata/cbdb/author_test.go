package cbdb

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Stránka s výsledky má bloky pro knihy i autory a odkazy "autor-…" jsou
// i u knih (jako jejich autoři). Hledání autorů smí číst jen svůj blok.
const authorSearchHTML = `<html><body>
<div id="search_result_box_books" class="search_result_box">
 <div class="search_graphic_box">
  <div class="search_graphic_box_content">
   <a href="kniha-975-valka-s-mloky">Válka s Mloky</a>
   <span class="search_author_link">Někdo Jiný</span>
   <a href="autor-114763-karin-vratna-militka">Karin Vrátná Militká</a>
  </div>
 </div>
</div>
<div id="search_result_box_authors" class="search_result_box display_none">
 <h2>Nalezeno v autorech</h2>
 <div class="search_graphic mb-5">
  <div class="search_graphic_box">
   <a href="autor-66-karel-capek" class="search_graphic_box_img"><img src="/authors/karel-capek-66.jpg" alt="Karel Čapek" /></a>
   <div class="search_graphic_box_content">
    <a href="autor-66-karel-capek">Karel Čapek</a><br /><br />
    *09.01.1890
   </div>
  </div>
  <div class="search_graphic_box">
   <a href="autor-28981-abe-capek" class="search_graphic_box_img"><img src="/authors/0.jpg" alt="Abe Čapek" /></a>
   <div class="search_graphic_box_content">
    <a href="autor-28981-abe-capek">Abe Čapek</a><br /><br />
   </div>
  </div>
 </div>
</div>
</body></html>`

func TestParseAuthorSearchResults(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(authorSearchHTML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	results := parseAuthorSearchResults(doc)
	if len(results) != 2 {
		t.Fatalf("výsledků = %d, chtěny 2: %+v", len(results), results)
	}

	first := results[0]
	if first.ID != 66 || first.Name != "Karel Čapek" {
		t.Errorf("první = %+v", first)
	}
	if want := baseURL + "/autor-66-karel-capek"; first.URL != want {
		t.Errorf("URL = %q, chtěno %q", first.URL, want)
	}
	// Rok narození odliší jmenovce.
	if first.BirthYear != 1890 || first.Note != "* 1890" {
		t.Errorf("roky = %d, note = %q", first.BirthYear, first.Note)
	}

	// Autor bez data narození projde, jen bez roku.
	if results[1].Name != "Abe Čapek" || results[1].BirthYear != 0 {
		t.Errorf("druhý = %+v", results[1])
	}

	// Autor z bloku knih se sem nesmí dostat.
	for _, r := range results {
		if r.ID == 114763 {
			t.Error("do výsledků se dostal autor z bloku knih")
		}
	}
}

func TestParseAuthorSearchResultsWithoutBox(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<html><body><a href="autor-66-karel-capek">Karel Čapek</a></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	// Bez bloku autorů se nesmí vracet náhodné odkazy ze stránky.
	if got := parseAuthorSearchResults(doc); len(got) != 0 {
		t.Errorf("bez bloku autorů vráceno %+v", got)
	}
}

const authorPageHTML = `<html><head>
<meta property="og:image" content="https://www.cbdb.cz/img.php?id=66&amp;type=author&amp;size=preview" />
<script type="application/ld+json">
{"@context":"https://schema.org/","@type":"Person","@id":"https://www.cbdb.cz/autor-66-karel-capek",
 "url":"https://www.cbdb.cz/autor-66-karel-capek","name":"Karel Čapek",
 "description":"Krátký popis z JSON-LD."}
</script>
</head><body>
<h1>Karel Čapek</h1>
<div class="author_lifestory" onclick="show_lifestory();">
  Životopis - Karel Čapek (1890–1938): Narodil se v Malých Svatoňovicích.
</div>
</body></html>`

func TestParseAuthorPage(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(authorPageHTML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	meta := parseAuthorPage(doc)

	if meta.Name != "Karel Čapek" {
		t.Errorf("name = %q", meta.Name)
	}
	// Delší životopis z HTML má přednost před krátkým popisem z JSON-LD.
	if strings.Contains(meta.Bio, "JSON-LD") {
		t.Errorf("použit krátký popis místo životopisu: %q", meta.Bio)
	}
	// Nadpis "Životopis - <jméno>:" do textu nepatří.
	if strings.HasPrefix(meta.Bio, "Životopis") {
		t.Errorf("v životopisu zůstal nadpis: %q", meta.Bio)
	}
	if !strings.HasPrefix(meta.Bio, "Narodil se") {
		t.Errorf("bio = %q", meta.Bio)
	}
	// JSON-LD roky nemá, odhadnou se z textu.
	if meta.BirthYear != 1890 || meta.DeathYear != 1938 {
		t.Errorf("roky = %d–%d, chtěno 1890–1938", meta.BirthYear, meta.DeathYear)
	}
	if !strings.Contains(meta.ImageURL, "id=66") {
		t.Errorf("image_url = %q", meta.ImageURL)
	}
}

func TestSupportsAuthorURL(t *testing.T) {
	c := NewClient()
	tests := map[string]bool{
		"https://www.cbdb.cz/autor-66-karel-capek": true,
		"https://www.cbdb.cz/kniha-975-valka":      false,
		"https://openlibrary.org/authors/OL1A":     false,
	}
	for rawURL, want := range tests {
		if got := c.SupportsAuthorURL(rawURL); got != want {
			t.Errorf("SupportsAuthorURL(%q) = %v, chtěno %v", rawURL, got, want)
		}
	}
}
