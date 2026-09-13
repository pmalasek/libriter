// Package metadata sjednocuje zdroje knižních metadat (databazeknih.cz,
// cbdb.cz, OpenLibrary, Google Books) za jedno rozhraní.
//
// Poskytovatelé se zapojují do Chain, která je zkouší v pořadí ze
// konfigurace – první, kdo něco najde, vyhrává. Díky tomu rozbitý scraper
// cizího webu nezablokuje celou funkci, jen se přeskočí.
package metadata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"libriter/internal/model"
)

// SearchQuery je dotaz na knihu. Author je nepovinný, ale hodně pomáhá:
// vyhledávání českých webů prochází jen názvy knih a jméno autora přilepené
// za název ignoruje, takže „Ostrov“ od Samuela Bjørka se mezi stovkou jiných
// Ostrovů nenajde. Zdroj s vlastním hledáním podle autora (Google Books,
// OpenLibrary) ho pošle rovnou v dotazu, scraper podle něj výsledky seřadí
// nebo zkusí knihu najít přes stránku autora.
type SearchQuery struct {
	Title  string
	Author string
}

// String složí dotaz do jednoho řetězce.
func (q SearchQuery) String() string {
	return strings.TrimSpace(q.Title + " " + q.Author)
}

// IsEmpty říká, že dotaz nemá co hledat.
func (q SearchQuery) IsEmpty() bool {
	return strings.TrimSpace(q.Title) == "" && strings.TrimSpace(q.Author) == ""
}

// SearchResult je jeden výsledek vyhledávání. Author a Year mohou zůstat
// prázdné – ne každý zdroj je dává už v seznamu, doplní se až detailem.
type SearchResult struct {
	// ID je interní identifikátor u zdroje; 0, pokud zdroj číselné ID nemá.
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int    `json:"year"`
	// URL se posílá zpět do /metadata/book?url= pro stažení detailu.
	URL string `json:"url"`
	// Source je jméno poskytovatele, ze kterého výsledek pochází.
	Source string `json:"source"`
}

// BookAuthor je jeden autor knihy se jménem rozděleným stejně, jako se jméno
// ukládá u autora (model.ParseAuthorName). Seznam skládá SplitAuthors z pole
// Author, ať klient jméno neparsuje sám a rozdělení je všude stejné.
type BookAuthor struct {
	Name       string `json:"name"`
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name"`
	LastName   string `json:"last_name"`
}

// BookMetadata jsou metadata jedné knihy. Nevyplněná pole zůstávají nulová –
// zdroje se v tom, co nabízejí, dost liší.
type BookMetadata struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	// Author je autor (nebo autoři, oddělení čárkou) tak, jak ho uvádí zdroj;
	// Authors je totéž rozebrané na jednotlivé autory a části jmen.
	Author   string       `json:"author"`
	AuthorID int          `json:"author_id"`
	Authors  []BookAuthor `json:"authors"`

	Description string   `json:"description"`
	Genres      []string `json:"genres"`
	CoverURL    string   `json:"cover_url"`
	// Rating je hodnocení zdroje v procentech (0–100), přepočtené na společnou
	// škálu. Není to internal_rating knihy (1–5).
	Rating    int    `json:"rating"`
	Publisher string `json:"publisher"`
	// Year je rok prvního vydání díla (u překladů rok originálu), pokud ho
	// zdroj zná; jinak rok vydání, které zdroj popisuje.
	Year int `json:"year"`
	// OriginalTitle je název originálu u překladů; prázdný u původních děl
	// a u zdrojů, které ho nedávají.
	OriginalTitle string `json:"original_title"`
	// Series je název knižní série, do které dílo patří, a SeriesPosition
	// pořadí dílu v ní. Zná je jen část zdrojů; u knihy mimo sérii zůstávají
	// obě pole nulová, u série bez číslování jen SeriesPosition.
	Series         string `json:"series"`
	SeriesPosition int    `json:"series_position"`
	SourceURL      string `json:"source_url"`
	Source         string `json:"source"`
}

// AuthorMetadata jsou metadata jednoho autora. Nevyplněná pole zůstávají nulová.
type AuthorMetadata struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// FirstName, MiddleName a LastName jsou Name rozdělené stejně, jako se
	// jméno ukládá u autora (model.ParseAuthorName). Doplňuje je Chain, aby
	// klient nemusel jméno parsovat sám a rozdělení bylo všude stejné.
	FirstName  string `json:"first_name"`
	MiddleName string `json:"middle_name"`
	LastName   string `json:"last_name"`
	Bio        string `json:"bio"`
	// Pseudonyms jsou jména, pod kterými autor vydává. Zdroje vedou autora pod
	// občanským jménem (Frode Sander Øien), ale knihy v knihovně jsou
	// podepsané pseudonymem (Samuel Bjørk); podle tohoto seznamu klient pozná,
	// že jméno nemá čím přepisovat.
	Pseudonyms []string `json:"pseudonyms"`
	// ImageURL je adresa fotky u zdroje. Stahuje se až na vyžádání
	// (PUT /authors/{id}/image), do databáze se ukládá soubor, ne odkaz.
	ImageURL  string `json:"image_url"`
	BirthYear int    `json:"birth_year"`
	DeathYear int    `json:"death_year"`
	SourceURL string `json:"source_url"`
	Source    string `json:"source"`
}

// AuthorSearchResult je jeden výsledek hledání autora.
type AuthorSearchResult struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// Note je krátký doplněk pro odlišení jmenovců – roky života,
	// nejznámější dílo, nebo „pseudonym“ u jmen, pod kterými autor vydává.
	Note      string `json:"note"`
	BirthYear int    `json:"birth_year"`
	DeathYear int    `json:"death_year"`
	URL       string `json:"url"`
	Source    string `json:"source"`
}

// Provider je jeden zdroj metadat knih.
type Provider interface {
	// Name je krátký identifikátor používaný v konfiguraci (METADATA_PROVIDERS).
	Name() string
	// Supports říká, jestli poskytovatel umí stáhnout detail z této URL.
	// Slouží zároveň jako allowlist – handler přes něj pouští /metadata/book?url=.
	Supports(rawURL string) bool
	Search(ctx context.Context, q SearchQuery) ([]SearchResult, error)
	FetchByURL(ctx context.Context, rawURL string) (*BookMetadata, error)
}

// AuthorProvider umí navíc metadata autorů. Implementují ho jen zdroje, které
// autory znají jako samostatné záznamy – Google Books je nemá, ten zůstane
// pouze u knih.
type AuthorProvider interface {
	Provider
	SupportsAuthorURL(rawURL string) bool
	// SupportsImageURL je allowlist pro stahování obrázků. Fotky bývají na
	// jiném hostiteli než stránky (covers.openlibrary.org), a bez tohoto
	// omezení by šlo server donutit stáhnout cokoliv odkudkoliv.
	SupportsImageURL(rawURL string) bool
	SearchAuthors(ctx context.Context, query string) ([]AuthorSearchResult, error)
	FetchAuthorByURL(ctx context.Context, rawURL string) (*AuthorMetadata, error)
}

// ErrNoProvider znamená, že žádný nakonfigurovaný zdroj danou URL neumí.
var ErrNoProvider = errors.New("žádný zdroj metadat neumí tuto adresu")

// Chain zkouší poskytovatele v zadaném pořadí.
type Chain struct {
	providers []Provider
}

func NewChain(providers ...Provider) *Chain {
	return &Chain{providers: providers}
}

// Providers vrací jména zdrojů v pořadí, ve kterém se zkoušejí.
func (c *Chain) Providers() []string {
	names := make([]string, 0, len(c.providers))
	for _, p := range c.providers {
		names = append(names, p.Name())
	}
	return names
}

// Search vrátí výsledky prvního zdroje, který něco najde. Zdroj, který selže
// nebo nic nevrátí, se přeskočí.
//
// Chyba se vrací jen tehdy, když nic nenašel nikdo A aspoň jeden zdroj selhal –
// jinak by se rozbitý scraper tvářil jako „kniha nenalezena“. Když všechny
// zdroje odpověděly a shodly se na prázdnu, je to prázdný seznam bez chyby.
func (c *Chain) Search(ctx context.Context, q SearchQuery) ([]SearchResult, error) {
	var firstErr error

	for _, p := range c.providers {
		results, err := p.Search(ctx, q)
		switch {
		case err != nil:
			// Cizí weby se rozbíjejí; zalogujeme a jdeme na další zdroj.
			slog.Warn("zdroj metadat selhal při hledání", "zdroj", p.Name(), "err", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", p.Name(), err)
			}
		case len(results) > 0:
			for i := range results {
				results[i].Source = p.Name()
			}
			return results, nil
		default:
			slog.Debug("zdroj metadat nic nenašel", "zdroj", p.Name(), "dotaz", q.String())
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

// Supports vrátí true, pokud URL zvládne některý z nakonfigurovaných zdrojů.
func (c *Chain) Supports(rawURL string) bool {
	for _, p := range c.providers {
		if p.Supports(rawURL) {
			return true
		}
	}
	return false
}

// FetchByURL předá požadavek tomu zdroji, který URL umí. Tady se fallback
// nedělá – adresa jednoznačně určuje zdroj.
func (c *Chain) FetchByURL(ctx context.Context, rawURL string) (*BookMetadata, error) {
	for _, p := range c.providers {
		if !p.Supports(rawURL) {
			continue
		}
		meta, err := p.FetchByURL(ctx, rawURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name(), err)
		}
		meta.Source = p.Name()
		// Zdroj, který autory uvádí jako samostatné záznamy, si seznam složil
		// sám a přesněji – přepisovat ho rozborem řetězce nemá smysl.
		if len(meta.Authors) == 0 {
			meta.Authors = SplitAuthors(meta.Author)
		}
		return meta, nil
	}
	return nil, ErrNoProvider
}

// SplitAuthors rozebere pole Author na jednotlivé autory. Zdroje je uvádějí
// různě – jedno jméno (databazeknih), nebo víc jmen oddělených čárkou
// (OpenLibrary, Google Books, cbdb) –, tvar "Příjmení, Křestní" se přitom
// za seznam nepovažuje.
//
// Volá ji Chain; handler, který si zdroj volá sám, ji musí zavolat taky.
func SplitAuthors(s string) []BookAuthor {
	names := model.ParseAuthorNames(s)
	authors := make([]BookAuthor, 0, len(names))
	for _, n := range names {
		authors = append(authors, BookAuthor{
			Name:       n.Full(),
			FirstName:  n.First,
			MiddleName: n.Middle,
			LastName:   n.Last,
		})
	}
	return authors
}

// AuthorMatches říká, jestli jméno autora z výsledku odpovídá hledanému.
// Stačí shoda příjmení: zdroje píšou jména různě („Samuel Bjørk“,
// „Bjørk, Samuel“) a u knih s víc autory je hledaný autor jen jedním
// ze jmen v řetězci.
func AuthorMatches(resultAuthor, wanted string) bool {
	last := model.ParseAuthorName(wanted).Last
	if last == "" {
		return false
	}
	for _, word := range strings.Fields(strings.ReplaceAll(resultAuthor, ",", " ")) {
		if strings.EqualFold(word, last) {
			return true
		}
	}
	return false
}

// RankByAuthor přesune dopředu výsledky, které napsal hledaný autor; pořadí
// uvnitř obou skupin zůstává. Zdroj, který hledá jen v názvech, tak aspoň
// nenechá správnou knihu až pod pěti jmenovci.
func RankByAuthor(results []SearchResult, author string) []SearchResult {
	if author == "" {
		return results
	}

	match := make([]SearchResult, 0, len(results))
	rest := make([]SearchResult, 0, len(results))
	for _, r := range results {
		if AuthorMatches(r.Author, author) {
			match = append(match, r)
		} else {
			rest = append(rest, r)
		}
	}
	return append(match, rest...)
}

// --- autoři ---

// authorProviders vrátí zdroje, které umí autory, v pořadí z konfigurace.
func (c *Chain) authorProviders() []AuthorProvider {
	providers := make([]AuthorProvider, 0, len(c.providers))
	for _, p := range c.providers {
		if ap, ok := p.(AuthorProvider); ok {
			providers = append(providers, ap)
		}
	}
	return providers
}

// AuthorProviders vrací jména zdrojů, které umí autory.
func (c *Chain) AuthorProviders() []string {
	names := []string{}
	for _, p := range c.authorProviders() {
		names = append(names, p.Name())
	}
	return names
}

// SearchAuthors zkouší zdroje v pořadí, vrátí výsledky prvního, který něco
// najde. Chybové chování je stejné jako u Search.
func (c *Chain) SearchAuthors(ctx context.Context, query string) ([]AuthorSearchResult, error) {
	var firstErr error

	for _, p := range c.authorProviders() {
		results, err := p.SearchAuthors(ctx, query)
		switch {
		case err != nil:
			slog.Warn("zdroj metadat selhal při hledání autora", "zdroj", p.Name(), "err", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", p.Name(), err)
			}
		case len(results) > 0:
			for i := range results {
				results[i].Source = p.Name()
			}
			return results, nil
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

// FetchAuthorByURL předá požadavek zdroji, kterému adresa patří.
func (c *Chain) FetchAuthorByURL(ctx context.Context, rawURL string) (*AuthorMetadata, error) {
	for _, p := range c.authorProviders() {
		if !p.SupportsAuthorURL(rawURL) {
			continue
		}
		meta, err := p.FetchAuthorByURL(ctx, rawURL)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name(), err)
		}
		meta.Source = p.Name()
		name := model.ParseAuthorName(meta.Name)
		meta.FirstName, meta.MiddleName, meta.LastName = name.First, name.Middle, name.Last
		return meta, nil
	}
	return nil, ErrNoProvider
}

// SupportsImageURL říká, jestli adresa obrázku patří některému zapnutému
// zdroji. Handler se na to ptá dřív, než začne cokoliv stahovat.
func (c *Chain) SupportsImageURL(rawURL string) bool {
	for _, p := range c.authorProviders() {
		if p.SupportsImageURL(rawURL) {
			return true
		}
	}
	return false
}

// HostMatches ověří, že URL má http(s) schéma a hostitele host (volitelně
// s předponou "www."). Poskytovatelé jím implementují Supports.
func HostMatches(rawURL, host string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return strings.EqualFold(u.Hostname(), host) ||
		strings.EqualFold(u.Hostname(), "www."+host)
}
