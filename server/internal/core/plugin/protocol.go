// Package plugin spouští zdroje metadat jako samostatné programy.
//
// Plugin je spustitelný soubor mediater-meta-<jméno> v adresáři PLUGIN_DIR.
// Server ho volá pro každý požadavek znovu: `<soubor> <příkaz>`, na stdin
// pošle jeden JSON objekt a ze stdout přečte jeden JSON objekt. Stderr jde
// do logu serveru. Plugin tak může být v libovolném jazyce; protokol popisuje
// docs/plugins.md.
//
// Stav mezi voláními plugin nemá – odstup mezi požadavky na cizí web proto
// drží server (Manifest.MinIntervalMS), stejně jako stahování obrázků
// (plugin vrací jen adresy, server je stáhne a ověří proti ImageHosts).
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ProtocolVersion je verze protokolu, kterou server umí.
const ProtocolVersion = 1

// FilePrefix je předpona souborů, které server považuje za pluginy.
const FilePrefix = "mediater-meta-"

// Příkazy protokolu.
const (
	CmdDescribe = "describe"
	CmdSearch   = "search"
	CmdFetch    = "fetch"
	CmdSeason   = "season"
)

// Druhy záznamů, které plugin umí.
const (
	KindBook   = "book"
	KindAuthor = "author"
	KindMovie  = "movie"
	KindShow   = "show"
)

// Kódy chyb, které plugin vrací v ErrorBody.Code.
const (
	ErrCodeNotFound    = "not_found"
	ErrCodeRateLimited = "rate_limited"
	ErrCodeUnavailable = "unavailable"
	ErrCodeBadRequest  = "bad_request"
)

// ConfigField je jedna položka konfigurace pluginu (typicky API klíč).
type ConfigField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Secret   bool   `json:"secret,omitempty"`
	Required bool   `json:"required,omitempty"`
	// Env je proměnná prostředí serveru s výchozí hodnotou (TMDB_API_KEY).
	// Hodnota uložená v administraci má přednost.
	Env string `json:"env,omitempty"`
}

// Manifest je odpověď na příkaz describe.
type Manifest struct {
	Protocol int      `json:"protocol"`
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	Version  string   `json:"version"`
	Kinds    []string `json:"kinds"`
	// Languages jsou jazyky (ISO 639-1), pro které má zdroj smysl; "*" = všechny.
	Languages []string      `json:"languages,omitempty"`
	Config    []ConfigField `json:"config,omitempty"`
	// URLPatterns jsou pro každý druh regulární výrazy (syntaxe RE2) adres
	// detailu, které plugin umí stáhnout. Slouží jako allowlist pro fetch
	// podle URL – cizí adresu server pluginu vůbec nepošle.
	URLPatterns map[string][]string `json:"url_patterns,omitempty"`
	// ImageHosts jsou hostitelé obrázků (obálky, plakáty, fotky), ze kterých
	// smí server stahovat. Hostitel platí i s předponou "www.".
	ImageHosts []string `json:"image_hosts,omitempty"`
	// MinIntervalMS je nejmenší odstup mezi voláními pluginu.
	MinIntervalMS int `json:"min_interval_ms,omitempty"`
}

var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]{0,39}$`)

// Validate ověří manifest a zkompiluje vzory adres.
func (m *Manifest) validate() (map[string][]*regexp.Regexp, error) {
	if m.Protocol != ProtocolVersion {
		return nil, fmt.Errorf("nepodporovaná verze protokolu %d", m.Protocol)
	}
	if !namePattern.MatchString(m.Name) {
		return nil, fmt.Errorf("neplatné jméno %q", m.Name)
	}
	if len(m.Kinds) == 0 {
		return nil, fmt.Errorf("plugin neuvádí žádný druh záznamů")
	}
	patterns := make(map[string][]*regexp.Regexp, len(m.URLPatterns))
	for kind, list := range m.URLPatterns {
		for _, p := range list {
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, fmt.Errorf("vzor adresy %q: %w", p, err)
			}
			patterns[kind] = append(patterns[kind], re)
		}
	}
	return patterns, nil
}

// Supports říká, jestli plugin umí daný druh záznamů.
func (m *Manifest) Supports(kind string) bool {
	for _, k := range m.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Query je hledaný záznam. Vyplněná pole záleží na druhu: kniha má název
// a autora, film a seriál název, původní název a rok, autor jen Title.
type Query struct {
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title,omitempty"`
	Author        string `json:"author,omitempty"`
	Year          int    `json:"year,omitempty"`
}

// Request je vstup příkazů search, fetch a season.
type Request struct {
	Kind     string `json:"kind"`
	Language string `json:"language,omitempty"`
	// Query vyplňuje search.
	Query *Query `json:"query,omitempty"`
	// ID nebo URL vyplňuje fetch (a season ID seriálu).
	ID  string `json:"id,omitempty"`
	URL string `json:"url,omitempty"`
	// Season je číslo sezóny pro příkaz season.
	Season int `json:"season,omitempty"`
	// Episodes jsou čísla dílů řady, které má server na disku (season).
	// Plugin, který stahuje každý díl zvlášť, může ostatní vynechat;
	// prázdné = všechny.
	Episodes []int             `json:"episodes,omitempty"`
	Config   map[string]string `json:"config,omitempty"`
}

// SearchResult je jeden výsledek hledání.
type SearchResult struct {
	ID            string `json:"id"`
	URL           string `json:"url,omitempty"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title,omitempty"`
	Year          int    `json:"year,omitempty"`
	Author        string `json:"author,omitempty"`
	Note          string `json:"note,omitempty"`
	ImageURL      string `json:"image_url,omitempty"`
	BirthYear     int    `json:"birth_year,omitempty"`
	DeathYear     int    `json:"death_year,omitempty"`
}

// SearchResponse je výstup příkazu search.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// FetchResponse je výstup příkazu fetch. Tvar Item záleží na druhu: kniha
// a autor jsou metadata.BookMetadata a AuthorMetadata Libriteru, film
// a seriál VideoMetadata.
type FetchResponse struct {
	Item json.RawMessage `json:"item"`
}

// SeasonResponse je výstup příkazu season.
type SeasonResponse struct {
	Season SeasonMetadata `json:"season"`
}

// ErrorBody je chyba, kterou plugin vrátí na stdout s nenulovým exit kódem.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse obaluje ErrorBody.
type ErrorResponse struct {
	Error *ErrorBody `json:"error"`
}

// Error je chyba vrácená pluginem.
type Error struct {
	Plugin  string
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("plugin %s: %s: %s", e.Plugin, e.Code, e.Message)
}

// IsNotFound říká, že plugin záznam nenašel.
func IsNotFound(err error) bool { return codeOf(err) == ErrCodeNotFound }

// IsRateLimited říká, že zdroj odmítá požadavky (HTTP 429, ochrana proti
// botům) nebo je plugin po takovém odmítnutí pozastavený.
func IsRateLimited(err error) bool { return codeOf(err) == ErrCodeRateLimited }

func codeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// HostMatches ověří, že URL má http(s) schéma a hostitele host (volitelně
// s předponou "www.").
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
