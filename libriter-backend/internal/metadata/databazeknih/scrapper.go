// internal/metadata/databazeknih/scraper.go
//
// Scraper pro databazeknih.cz
// Použití povoleno provozovatelem webu pro osobní nekomerční účely.
//
// Respektuje slušné chování:
//   - User-Agent identifikuje aplikaci
//   - Rate limiting min. 3s mezi požadavky
//   - Scraping pouze on-demand (nová kniha), ne bulk

package databazeknih

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"libriter/internal/metadata"
	"libriter/internal/metadata/htmlutil"

	"golang.org/x/net/html"
)

// ProviderName je jméno zdroje v nastavení. Handler podle něj v řetězci
// hledá právě tohoto klienta – endpoint /metadata/book/{id} pracuje
// s číselným ID, které ostatní zdroje nesdílejí.
const ProviderName = providerName

const (
	providerName = "databazeknih"
	baseURL      = "https://www.databazeknih.cz"
	host         = "databazeknih.cz"
	minDelay     = 3 * time.Second
	searchLimit  = 10
	// Kolik stránek vydaných knih autora se projde, než to vzdáme.
	// Jedna stránka je 20 knih; víc už je moc požadavků na jedno hledání.
	authorBookPages = 3
)

// Aliasy na sdílené typy – ať se v parserech nečte metadata.BookMetadata.
type (
	SearchResult = metadata.SearchResult
	BookMetadata = metadata.BookMetadata
)

// Client je scraper klient s rate limitingem
type Client struct {
	fetcher *metadata.Fetcher
}

// NewClient vytvoří nový scraper klient
func NewClient() *Client {
	return &Client{fetcher: metadata.NewFetcher(minDelay)}
}

func (c *Client) Name() string { return providerName }

// Supports přijímá jen adresy z databazeknih.cz – slouží zároveň jako
// allowlist pro /metadata/book?url=.
func (c *Client) Supports(rawURL string) bool {
	return metadata.HostMatches(rawURL, host)
}

// Search vyhledá knihy podle dotazu, vrátí max. 10 výsledků.
//
// Web hledá jen v názvech knih – jméno autora v dotazu ignoruje. U běžného
// názvu („Ostrov“) tak vrátí padesát stránek jmenovců a kniha hledaného autora
// mezi prvními deseti není. Proto se autor neposílá do dotazu, ale výsledky se
// podle něj seřadí; a když mezi nimi není vůbec, zkusí se knihy autora.
func (c *Client) Search(ctx context.Context, q metadata.SearchQuery) ([]SearchResult, error) {
	results, err := c.searchTitle(ctx, q.Title)
	if err != nil {
		return nil, err
	}
	if q.Author == "" {
		return results, nil
	}

	for _, r := range results {
		if metadata.AuthorMatches(r.Author, q.Author) {
			return metadata.RankByAuthor(results, q.Author), nil
		}
	}

	// Dva požadavky navíc (autor + jeho knihy), zato se kniha najde. Selhání
	// se ignoruje – zůstanou výsledky podle názvu.
	byAuthor, err := c.searchAuthorBooks(ctx, q)
	if err != nil {
		slog.Debug("hledání přes autora selhalo", "autor", q.Author, "err", err)
	}
	if len(byAuthor) == 0 {
		return results, nil
	}

	merged := append(byAuthor, results...)
	if len(merged) > searchLimit {
		merged = merged[:searchLimit]
	}
	return merged, nil
}

// searchTitle je vyhledávání webu; hledá jen v názvech knih.
func (c *Client) searchTitle(ctx context.Context, title string) ([]SearchResult, error) {
	// Vyhledávací formulář na webu míří na /search?in=books; starší /hledat
	// dnes vrací 404.
	searchURL := fmt.Sprintf("%s/search?in=books&q=%s",
		baseURL,
		url.QueryEscape(title),
	)

	body, err := c.fetch(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search fetch: %w", err)
	}
	defer body.Close()

	return parseSearchResults(body)
}

// searchAuthorBooks najde knihu přes stránku autora: nejdřív autora podle
// jména, pak jeho vydané knihy a v nich hledaný název. Prochází se nejvýš
// authorBookPages stránek výpisu.
func (c *Client) searchAuthorBooks(ctx context.Context, q metadata.SearchQuery) ([]SearchResult, error) {
	authors, err := c.SearchAuthors(ctx, q.Author)
	if err != nil {
		return nil, fmt.Errorf("hledání autora: %w", err)
	}

	// Jen autor, jehož jméno opravdu sedí – hledání vrací i vzdálené shody
	// (na „Samuel Bjørk“ nabídne i Ricki Ostrov).
	var authorURL, authorName string
	for _, a := range authors {
		if metadata.AuthorMatches(a.Name, q.Author) {
			authorURL, authorName = a.URL, a.Name
			break
		}
	}
	if authorURL == "" {
		return nil, nil
	}

	booksURL, ok := booksURLFor(authorURL)
	if !ok {
		return nil, nil
	}

	var found []SearchResult
	for page := 1; page <= authorBookPages; page++ {
		pageURL := booksURL
		if page > 1 {
			pageURL = fmt.Sprintf("%s?page=%d", booksURL, page)
		}

		body, err := c.fetch(ctx, pageURL)
		if err != nil {
			return found, fmt.Errorf("vydané knihy autora: %w", err)
		}
		doc, err := html.Parse(body)
		body.Close()
		if err != nil {
			return found, err
		}

		books := parseAuthorBooks(doc)
		for _, b := range books {
			if titleMatches(b.Title, q.Title) {
				b.Author = authorName
				found = append(found, b)
			}
		}
		// Poslední stránka výpisu – dál už není kam jít.
		if len(found) > 0 || len(books) < authorBooksPerPage {
			break
		}
	}
	return found, nil
}

// FetchBook stáhne metadata knihy podle jejího ID
func (c *Client) FetchBook(ctx context.Context, bookID int) (*BookMetadata, error) {
	// Potřebujeme slug - použijeme redirect přes zkrácené URL
	bookURL := fmt.Sprintf("%s/prehled-knihy/kniha-%d", baseURL, bookID)

	body, err := c.fetch(ctx, bookURL)
	if err != nil {
		return nil, fmt.Errorf("fetch book %d: %w", bookID, err)
	}
	defer body.Close()

	meta, err := parseBookPage(body)
	if err != nil {
		return nil, fmt.Errorf("parse book %d: %w", bookID, err)
	}

	meta.ID = bookID
	meta.SourceURL = bookURL
	c.addOriginalEdition(ctx, meta)
	return meta, nil
}

// addOriginalEdition doplní název a rok originálu ze sekce „Více info“, kterou
// stránka knihy načítá zvlášť (XHR). Rok originálu je rok prvního vydání díla
// a má přednost před rokem českého vydání z infoboxu – podle něj se v knihovně
// řadí a překlad vydaný o dvacet let později by pořadí rozbil.
//
// Selhání se ignoruje – zůstane rok z infoboxu.
func (c *Client) addOriginalEdition(ctx context.Context, meta *BookMetadata) {
	if meta.ID == 0 {
		return
	}

	body, err := c.fetch(ctx, fmt.Sprintf("%s/book-detail-more-info/%d", baseURL, meta.ID))
	if err != nil {
		return
	}
	defer body.Close()

	title, year := parseOriginalEdition(body)
	meta.OriginalTitle = title
	if year > 0 {
		meta.Year = year
	}
}

// parseOriginalEdition vytáhne z fragmentu „Více info“ řádek
//
//	<dt>Originální název</dt> <dd>The Hitchhiker's Guide to the Galaxy, 1979</dd>
//
// Rok za čárkou je nepovinný; u českých knih řádek chybí úplně.
func parseOriginalEdition(r io.Reader) (title string, year int) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", 0
	}

	dt := htmlutil.Find(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "dt" &&
			strings.EqualFold(htmlutil.Collapse(htmlutil.Text(n)), "Originální název")
	})
	if dt == nil {
		return "", 0
	}

	var dd *html.Node
	for sib := dt.NextSibling; sib != nil; sib = sib.NextSibling {
		if sib.Type == html.ElementNode && sib.Data == "dd" {
			dd = sib
			break
		}
	}
	if dd == nil {
		return "", 0
	}

	text := htmlutil.Collapse(htmlutil.Text(dd))
	if m := originalYearRe.FindStringSubmatch(text); m != nil {
		if v, err := strconv.Atoi(m[1]); err == nil && v > 1000 && v <= time.Now().Year()+1 {
			year = v
			text = strings.TrimSpace(strings.TrimSuffix(text[:len(text)-len(m[0])], ","))
		}
	}
	return text, year
}

// FetchByURL stáhne metadata knihy přímo z URL.
// Preferovaná metoda - URL máme ze search výsledků.
func (c *Client) FetchByURL(ctx context.Context, bookURL string) (*BookMetadata, error) {
	body, err := c.fetch(ctx, bookURL)
	if err != nil {
		return nil, fmt.Errorf("fetch book url: %w", err)
	}
	defer body.Close()

	meta, err := parseBookPage(body)
	if err != nil {
		return nil, fmt.Errorf("parse book page: %w", err)
	}

	meta.ID = extractIDFromURL(bookURL)
	meta.SourceURL = bookURL
	meta.Source = providerName
	c.addOriginalEdition(ctx, meta)
	return meta, nil
}

// DownloadCover stáhne obálku knihy a vrátí její obsah
func (c *Client) DownloadCover(ctx context.Context, coverURL string) ([]byte, string, error) {
	body, err := c.fetch(ctx, coverURL)
	if err != nil {
		return nil, "", fmt.Errorf("download cover: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, "", fmt.Errorf("read cover: %w", err)
	}

	// Detekce MIME typu podle prvních bytů
	mime := "image/jpeg"
	if len(data) > 4 && string(data[1:4]) == "PNG" {
		mime = "image/png"
	}

	return data, mime, nil
}

// -------------------------------------------------------------------
//  HTTP fetch s rate limitingem
// -------------------------------------------------------------------

func (c *Client) fetch(ctx context.Context, targetURL string) (io.ReadCloser, error) {
	return c.fetcher.Get(ctx, targetURL, "text/html,application/xhtml+xml")
}

// -------------------------------------------------------------------
//  HTML parsery
// -------------------------------------------------------------------

// parseBookPage parsuje stránku prehled-knihy
func parseBookPage(r io.Reader) (*BookMetadata, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	meta := &BookMetadata{}

	// Procházení DOM
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {

			// Skripty a styly nejsou obsah stránky. JSON-LD blok v hlavičce
			// obsahuje dateModified s aktuálním datem, které by se jinak
			// spletlo s rokem vydání.
			case "script", "style":
				return

			// <h1> = název knihy
			case "h1":
				if meta.Title == "" {
					meta.Title = strings.TrimSpace(htmlutil.Text(n))
				}

			// <meta> tagy - og:image pro obálku, description pro popis
			case "meta":
				prop := htmlutil.Attr(n, "property")
				name := htmlutil.Attr(n, "name")
				content := htmlutil.Attr(n, "content")

				switch {
				case prop == "og:image" && content != "":
					// Chceme full-size, ne bmid_ variantu
					meta.CoverURL = strings.Replace(content, "bmid_", "", 1)
				case (prop == "og:description" || name == "description") && meta.Description == "":
					// og:description je zkrácená - použijeme jen jako fallback
					if meta.Description == "" {
						meta.Description = cleanDescription(content)
					}
				}

			// <a> linky - autor, žánry, nakladatel. Autora tady bereme jen
			// jako záložní cestu; hlavní seznam čte parseAuthors z řádku
			// pod názvem knihy, kde jsou i spoluautoři.
			case "a":
				href := htmlutil.Attr(n, "href")
				text := strings.TrimSpace(htmlutil.Text(n))

				switch {
				case strings.Contains(href, "/autori/") && text != "" && meta.Author == "":
					meta.Author = text
					meta.AuthorID = extractIDFromURL(href)

				case strings.Contains(href, "/zanry/") && text != "":
					meta.Genres = append(meta.Genres, text)

				case strings.Contains(href, "/nakladatelstvi/") && text != "" && meta.Publisher == "":
					meta.Publisher = text
				}
			}
		}

		// Hledáme popis v sekci "O knize" - je v <p> za <h2> s textem "O knize"
		if n.Type == html.TextNode {
			trimmed := strings.TrimSpace(n.Data)

			// Rating - hledáme pattern "86 %" v textu
			if ratingRe.MatchString(trimmed) && meta.Rating == 0 {
				if m := ratingRe.FindStringSubmatch(trimmed); len(m) > 1 {
					if v, err := strconv.Atoi(m[1]); err == nil {
						meta.Rating = v
					}
				}
			}

			// Rok vydání – v infoboxu knihy stojí samostatně („2013, Druhé
			// město“). Rok uvnitř delšího textu (popis, patička) se nebere,
			// tam jde skoro vždy o něco jiného.
			if meta.Year == 0 && yearOnlyRe.MatchString(trimmed) {
				if v, err := strconv.Atoi(trimmed); err == nil && v > 1800 && v <= time.Now().Year()+1 {
					meta.Year = v
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(doc)

	// Popis - zkusíme najít sekci "O knize" lépe
	if desc := extractDescription(doc); desc != "" {
		meta.Description = desc
	}

	if names, id := parseAuthors(doc); len(names) > 0 {
		meta.Author = strings.Join(names, ", ")
		meta.AuthorID = id
		// Jména rozebíráme po jednom – ze složeného řetězce by se jméno
		// s čárkou („Čapek, Karel“) rozdělilo jinak, než jak ho píše stránka.
		for _, name := range names {
			meta.Authors = append(meta.Authors, metadata.SplitAuthors(name)...)
		}
	}

	meta.Series, meta.SeriesPosition = parseSeries(doc)

	if meta.Title == "" {
		return nil, fmt.Errorf("titul nenalezen na stránce")
	}

	return meta, nil
}

// parseSearchResults parsuje výsledky vyhledávání knih.
//
// Jeden výsledek je <p class="new"> a uvnitř odkaz na knihu plus
// <span class="pozn"> s textem "2007, Karel Čapek". Autor je pro výběr
// z výsledků zásadní – stejných názvů bývá víc.
//
// Když se struktura stránky změní, spadne se na hledání holých odkazů:
// výsledky pak nemají autora, ale funkce se aspoň nerozbije úplně.
func parseSearchResults(r io.Reader) ([]SearchResult, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "p" || !htmlutil.HasClass(n, "new") {
			return true
		}

		result, ok := parseSearchRow(n)
		if ok && !seen[result.ID] {
			seen[result.ID] = true
			results = append(results, result)
		}
		return false // řádek je zpracovaný celý, dovnitř už nelezeme
	})

	if len(results) == 0 {
		return parseSearchLinks(doc), nil
	}
	return results, nil
}

// parseSearchRow přečte jeden <p class="new"> s výsledkem.
func parseSearchRow(row *html.Node) (SearchResult, bool) {
	var result SearchResult

	htmlutil.Walk(row, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return true
		}

		switch {
		// Odkaz s názvem knihy. Obálka odkazuje na tutéž adresu, ale text nemá.
		case n.Data == "a" && result.Title == "":
			href := htmlutil.Attr(n, "href")
			if !strings.Contains(href, "/prehled-knihy/") && !strings.Contains(href, "/knihy/") {
				return true
			}
			title := htmlutil.Collapse(htmlutil.Text(n))
			if title == "" || strings.Contains(title, "Více") {
				return true
			}
			result.ID = extractIDFromURL(href)
			result.Title = title
			result.URL = htmlutil.ResolveURL(baseURL, href)

		// Poznámka pod názvem: "2007, Karel Čapek".
		case n.Data == "span" && htmlutil.HasClass(n, "pozn") && result.Author == "":
			result.Year, result.Author = parseNote(htmlutil.Collapse(htmlutil.Text(n)))
		}
		return true
	})

	return result, result.ID > 0 && result.Title != ""
}

// parseNote rozdělí "2007, Karel Čapek" na rok a autory. Rok i autor mohou
// chybět; víc autorů zůstane oddělených čárkou.
func parseNote(note string) (year int, author string) {
	parts := strings.Split(note, ",")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i == 0 && yearOnlyRe.MatchString(part) {
			year, _ = strconv.Atoi(part)
			continue
		}
		if author == "" {
			author = part
		} else {
			author += ", " + part
		}
	}
	return year, author
}

// parseSearchLinks je záložní parser pro případ, že se rozložení stránky změní.
func parseSearchLinks(doc *html.Node) []SearchResult {
	var results []SearchResult
	seen := make(map[int]bool)

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if len(results) >= searchLimit {
			return false
		}
		if n.Type != html.ElementNode || n.Data != "a" {
			return true
		}

		href := htmlutil.Attr(n, "href")
		if !strings.Contains(href, "/prehled-knihy/") && !strings.Contains(href, "/knihy/") {
			return true
		}

		id := extractIDFromURL(href)
		title := htmlutil.Collapse(htmlutil.Text(n))
		if id == 0 || title == "" || seen[id] || strings.Contains(title, "Více") {
			return true
		}

		seen[id] = true
		results = append(results, SearchResult{
			ID:    id,
			Title: title,
			URL:   htmlutil.ResolveURL(baseURL, href),
		})
		return true
	})

	return results
}

// -------------------------------------------------------------------
//  Pomocné funkce
// -------------------------------------------------------------------

var (
	ratingRe = regexp.MustCompile(`(\d{1,3})\s*%`)
	idRe     = regexp.MustCompile(`-(\d+)$`)
	// „Název originálu, 1979“ – rok na konci řádku Originální název.
	originalYearRe = regexp.MustCompile(`,?\s*(\d{4})\s*$`)
	yearOnlyRe     = regexp.MustCompile(`^(?:1[89]\d{2}|20\d{2})$`)
	// Rok kdekoliv v textu – „2023 (1. vydání)“ ve výpisu knih autora.
	yearInTextRe = regexp.MustCompile(`\b(1[89]\d{2}|20\d{2})\b`)
	// „1. díl“ z pruhu série nad názvem knihy.
	seriesPartRe = regexp.MustCompile(`(\d+)\s*\.\s*díl`)
	readMoreRe   = regexp.MustCompile(`\s*(?:\.{3}|…)\s*celý text\s*$`)
)

func extractIDFromURL(u string) int {
	// Odstranit query string a fragment
	if idx := strings.IndexAny(u, "?#"); idx >= 0 {
		u = u[:idx]
	}
	u = strings.TrimRight(u, "/")

	m := idRe.FindStringSubmatch(u)
	if len(m) < 2 {
		return 0
	}
	id, _ := strconv.Atoi(m[1])
	return id
}

func cleanDescription(s string) string {
	// Oříznutí "... od Autor Jméno" na konci (formát meta description)
	if idx := strings.LastIndex(s, "... od "); idx > 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// isReadMoreLink pozná ovládací odkaz "… celý text" nad popisem.
func isReadMoreLink(n *html.Node) bool {
	return n.Data == "a" && htmlutil.HasClass(n, "show_hide_more")
}

// cleanReadMore odřízne zbytek ovládacího odkazu, kdyby se změnila jeho třída.
// Text popisu na stránce je celý, jen je vizuálně zkrácený.
func cleanReadMore(text string) string {
	return strings.TrimSpace(readMoreRe.ReplaceAllString(text, ""))
}

// extractDescription hledá text v sekci "O knize" v DOM
func extractDescription(doc *html.Node) string {
	var result string
	var inSection bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if result != "" {
			return
		}

		if n.Type == html.ElementNode && n.Data == "h2" {
			text := strings.TrimSpace(htmlutil.Text(n))
			inSection = strings.Contains(text, "O knize")
		}

		if inSection && n.Type == html.ElementNode && n.Data == "p" {
			// Odkaz "… celý text" jen rozbaluje už načtený text; do popisu nepatří.
			text := strings.TrimSpace(htmlutil.TextSkipping(n, isReadMoreLink))
			if len(text) > 50 {
				result = cleanReadMore(text)
				inSection = false
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return result
}

// parseSeries přečte pruh se sérií nad názvem knihy:
//
//	<div class="lora book_detail_serie_info">
//	  <p class="inline"><a href='/serie/stoparuv-pruvodce-galaxii-135'>Stopařův průvodce Galaxií</a> série</p>
//	  <span class="nowrap"><a class="arrow" href="…">&lt;</a>
//	    <span class="odright_pet odleft_pet">1. díl</span>
//	    <a class="arrow" href="…">&gt;</a></span>
//	</div>
//
// Kniha mimo sérii blok nemá vůbec; u série bez číslování dílů chybí jen
// pořadí. V obou případech se vrací nula, ne chyba – série je doplněk.
func parseSeries(doc *html.Node) (title string, position int) {
	box := htmlutil.Find(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "div" &&
			htmlutil.HasClass(n, "book_detail_serie_info")
	})
	if box == nil {
		return "", 0
	}

	link := htmlutil.Find(box, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a" &&
			strings.Contains(htmlutil.Attr(n, "href"), "/serie/")
	})
	if link == nil {
		return "", 0
	}

	title = htmlutil.Collapse(htmlutil.Text(link))
	if title == "" {
		// Odkaz může nést jen obrázek; název je pak v atributu title.
		title = htmlutil.Collapse(htmlutil.Attr(link, "title"))
	}
	if title == "" {
		return "", 0
	}

	// Číslo dílu se hledá v textu celého bloku – šipky na sousední díly
	// jsou vedle něj a jejich text je jen „<“ a „>“.
	if m := seriesPartRe.FindStringSubmatch(htmlutil.Collapse(htmlutil.Text(box))); m != nil {
		position, _ = strconv.Atoi(m[1])
	}
	return title, position
}

// parseAuthors přečte autory z řádku pod názvem knihy:
//
//	<p class="lora oddown_midl"><span>
//	  <span class="author"><a href="/autori/leos-kysa-12208">František Kotleta</a>
//	    <span class="pozn_light">(p)</span>,</span>
//	  <span class="author"><a href="/autori/kristyna-snegonova-11744">Kristýna Sněgoňová</a></span>
//	</span></p>
//
// Bere se text odkazu, ne adresa: u pseudonymu míří odkaz na občanské jméno
// (Leoš Kyša), ale kniha je podepsaná pseudonymem. Značka „(p)“ za jménem je
// mimo odkaz, takže se do jména neplete – a stejně tak odkazy na autory
// jinde na stránce („Další knihy autora“), ty v bloku .author nejsou.
//
// firstID je ID prvního autora u zdroje; drží se kvůli odkazu na jeho stránku.
func parseAuthors(doc *html.Node) (names []string, firstID int) {
	htmlutil.Walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "span" || !htmlutil.HasClass(n, "author") {
			return true
		}

		link := htmlutil.Find(n, func(c *html.Node) bool {
			return c.Type == html.ElementNode && c.Data == "a" &&
				strings.Contains(htmlutil.Attr(c, "href"), "/autori/")
		})
		if link == nil {
			return false
		}

		name := htmlutil.Collapse(htmlutil.Text(link))
		if name == "" {
			return false
		}
		if len(names) == 0 {
			firstID = extractIDFromURL(htmlutil.Attr(link, "href"))
		}
		names = append(names, name)
		return false // uvnitř jmenovky autora už nic dalšího není
	})
	return names, firstID
}

// authorBooksPerPage je počet knih na jedné stránce výpisu /vydane-knihy/.
// Kratší stránka znamená, že další už není.
const authorBooksPerPage = 20

// booksURLFor přeloží /autori/<slug>-<id> na /vydane-knihy/<slug>-<id>.
func booksURLFor(authorURL string) (string, bool) {
	u, err := url.Parse(authorURL)
	if err != nil {
		return "", false
	}

	slug, ok := strings.CutPrefix(u.Path, "/autori/")
	if !ok || slug == "" || strings.Contains(slug, "/") {
		return "", false
	}
	return baseURL + "/vydane-knihy/" + slug, true
}

// parseAuthorBooks čte výpis vydaných knih autora:
//
//	<h3 class="midlRowHeight oddown">
//	  <a href="/prehled-knihy/...-ostrov-521083" title="Ostrov">Ostrov</a>
//	  <span class="pozn odl">2023 (1. vydání)</span>
//	</h3>
func parseAuthorBooks(doc *html.Node) []SearchResult {
	var results []SearchResult

	htmlutil.Walk(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode || n.Data != "h3" {
			return true
		}

		link := htmlutil.Find(n, func(c *html.Node) bool {
			return c.Type == html.ElementNode && c.Data == "a" &&
				strings.Contains(htmlutil.Attr(c, "href"), "/prehled-knihy/")
		})
		if link == nil {
			return false
		}

		href := htmlutil.Attr(link, "href")
		title := htmlutil.Collapse(htmlutil.Text(link))
		id := extractIDFromURL(href)
		if title == "" || id == 0 {
			return false
		}

		result := SearchResult{
			ID:     id,
			Title:  title,
			URL:    htmlutil.ResolveURL(baseURL, href),
			Source: providerName,
		}
		if m := yearInTextRe.FindStringSubmatch(htmlutil.Collapse(htmlutil.Text(n))); m != nil {
			result.Year, _ = strconv.Atoi(m[1])
		}
		results = append(results, result)
		return false
	})

	return results
}

// titleMatches porovná název z výpisu s hledaným. Nestačí přesná shoda –
// stránka u některých vydání píše název i se sérií („Atomové šelmy: Aréna“) –,
// ale ani volné hledání po slovech: to by u „Ostrov“ prošlo všechno.
func titleMatches(title, wanted string) bool {
	title = strings.ToLower(htmlutil.Collapse(title))
	wanted = strings.ToLower(htmlutil.Collapse(wanted))
	if title == "" || wanted == "" {
		return false
	}
	return strings.Contains(title, wanted) || strings.Contains(wanted, title)
}
