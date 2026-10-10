package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// VideoExts jsou přípony souborů, které scanner zakládá jako video.
var VideoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true,
	".webm": true, ".ts": true, ".m2ts": true, ".wmv": true, ".mpg": true, ".mpeg": true,
}

// SubtitleExts jsou přípony titulků, které scanner páruje s videem.
var SubtitleExts = map[string]string{
	".srt": "srt", ".vtt": "vtt", ".ass": "ass", ".ssa": "ass", ".sub": "sub",
}

// IsVideo řekne, jestli cesta vede na video soubor.
func IsVideo(path string) bool { return VideoExts[strings.ToLower(filepath.Ext(path))] }

// Ignored řekne, jestli scanner položku přeskočí: systémové adresáře NAS
// (@eaDir, #recycle), metadata Synology (.vsmeta), rozpracované a skryté
// soubory a ukázky ("sample").
func Ignored(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(name, "."), strings.HasPrefix(name, "@"), strings.HasPrefix(name, "#"):
		return true
	case strings.HasSuffix(lower, ".vsmeta"), strings.HasSuffix(lower, ".part"),
		strings.HasSuffix(lower, ".tmp"), strings.HasSuffix(lower, ".!qb"):
		return true
	case lower == "sample" || strings.HasPrefix(lower, "sample.") || strings.Contains(lower, "-sample."):
		return true
	}
	return false
}

// MovieInfo je výsledek rozboru názvu filmu.
type MovieInfo struct {
	Title         string
	OriginalTitle string
	Year          int
}

var (
	// Rok v závorce: "Název (2003 - Režisér)", "Altamira (2016 FullHD…)".
	parenYearRe = regexp.MustCompile(`[(\[]\s*((?:19|20)\d{2})\b`)
	// Samostatný rok mezi oddělovači: "22.míle.2018.Mile…", "Amadeus-1984-CZ…".
	yearTokenRe = regexp.MustCompile(`(?:^|[\s._\-])((?:19|20)\d{2})(?:$|[\s._\-])`)
	// Značky vydání, za kterými už název nepokračuje.
	releaseTagRe = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(` + strings.Join([]string{
		`480p`, `576p`, `720p`, `1080[pi]`, `2160p`, `4k`, `uhd`, `fullhd`, `hd(?:tv|rip)?`,
		`br-?rip`, `bd-?rip`, `blu-?ray`, `bluray`, `web-?dl`, `web-?rip`, `dvd-?rip`, `dvd(?:scr|r)?`,
		`hdtv`, `x26[45]`, `h\.?26[45]`, `hevc`, `xvid`, `divx`, `ac3`, `dts`, `aac`, `mp3`,
		`cz(?:-?dab(?:ing)?\.?)?`, `sk`, `dabing`, `titulky`, `cztit(?:ulky)?`, `eng`, `multi`, `remux`,
		`proper`, `repack`, `extended`, `unrated`,
	}, "|") + `)(?:$|[\s._\-\])])`)
	separatorsRe = regexp.MustCompile(`[._]+`)
	spacesRe     = regexp.MustCompile(`\s+`)
	// Úvodní značky bez roku: "!(Dokument)-Název", "[CZ] Název", "_ Název".
	leadingTagRe  = regexp.MustCompile(`^[^\pL\pN(\[]*(?:[(\[][^)\]\d]*[)\]][^\pL\pN(\[]*)+`)
	leadingJunkRe = regexp.MustCompile(`^[^\pL\pN(\[]+`)
	// Pořadové číslo složky ve sbírce: "02. Putování s dinosaury".
	ordinalRe = regexp.MustCompile(`^0\d{1,2}[.)]\s+`)
)

// ParseMovie rozebere název souboru nebo adresáře filmu (bez přípony) na
// název, původní název a rok. Zvládá české konvence "Název (rok - režisér)",
// "Český název - Original Title (rok)" i scénové "Název.2018.720p.x264…".
func ParseMovie(name string) MovieInfo {
	name = strings.TrimSpace(name)
	name = leadingTagRe.ReplaceAllString(name, "")
	name = leadingJunkRe.ReplaceAllString(name, "")
	name = ordinalRe.ReplaceAllString(name, "")
	var info MovieInfo
	title := name

	if m := parenYearRe.FindStringSubmatchIndex(name); m != nil && m[0] > 0 {
		info.Year, _ = strconv.Atoi(name[m[2]:m[3]])
		title = name[:m[0]]
	} else if loc := firstYearToken(name); loc != nil {
		info.Year, _ = strconv.Atoi(name[loc[2]:loc[3]])
		title = name[:loc[0]]
	} else if m := releaseTagRe.FindStringIndex(name); m != nil && m[0] > 0 {
		title = name[:m[0]]
	}

	title = cleanTitle(title)
	if t, orig, ok := strings.Cut(title, " - "); ok && strings.TrimSpace(t) != "" && strings.TrimSpace(orig) != "" {
		info.Title, info.OriginalTitle = strings.TrimSpace(t), strings.TrimSpace(orig)
	} else {
		info.Title = title
	}
	if info.Title == "" {
		info.Title = cleanTitle(name)
	}
	return info
}

// firstYearToken najde první samostatný rok, který není hned na začátku
// (film "2012" nebo "1917" má rok jako název).
func firstYearToken(name string) []int {
	offset := 0
	for {
		m := yearTokenRe.FindStringSubmatchIndex(name[offset:])
		if m == nil {
			return nil
		}
		for i := range m {
			m[i] += offset
		}
		if m[2] > 0 {
			return m
		}
		offset = m[3]
	}
}

// cleanTitle převede oddělovače na mezery a ořízne zbytky na okrajích.
// Pomlčky jsou oddělovač jen v názvu bez mezer s víc pomlčkami
// ("Nigel-Marven-a-Invaze") – "Spider-Man" zůstane.
func cleanTitle(s string) string {
	s = separatorsRe.ReplaceAllString(s, " ")
	if !strings.ContainsAny(strings.TrimSpace(s), " ") && strings.Count(s, "-") >= 2 {
		s = strings.ReplaceAll(s, "-", " ")
	}
	s = spacesRe.ReplaceAllString(s, " ")
	return strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == '-' || r == '(' || r == '[' || r == ','
	})
}

// EpisodeInfo je výsledek rozboru názvu dílu.
type EpisodeInfo struct {
	Season  int // 0 = neuvedeno v názvu souboru
	Episode int
	Title   string
	OK      bool
}

var (
	sxxexxRe  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,2})[\s._-]*e(\d{1,3})(?:$|[^0-9])`)
	nxnnRe    = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d{1,2})x(\d{1,3})(?:$|[^0-9])`)
	dilRe     = regexp.MustCompile(`(?i)(?:^|[^0-9])(\d{1,3})\.\s*díl(?:$|[^\pL])`)
	dilWordRe = regexp.MustCompile(`(?i)(?:^|[^\pL])(?:díl|epizoda|episode|ep|e)[\s._-]*(\d{1,3})(?:$|[^0-9])`)
	seasonRe  = regexp.MustCompile(`(?i)^(?:s|season|série|serie|sezóna|sezona|řada|rada)[\s._-]*(\d{1,2})$`)
	seasonRe2 = regexp.MustCompile(`(?i)^(\d{1,2})\.?\s*(?:série|serie|sezóna|sezona|řada|rada|season)$`)
)

// ParseEpisode rozebere název souboru dílu (bez přípony). Rozpozná
// "S01E02", "1x02", "02. díl" i "Díl 2"; název dílu je text za číslem.
func ParseEpisode(name string) EpisodeInfo {
	for _, re := range []*regexp.Regexp{sxxexxRe, nxnnRe} {
		if m := re.FindStringSubmatchIndex(name); m != nil {
			s, _ := strconv.Atoi(name[m[2]:m[3]])
			e, _ := strconv.Atoi(name[m[4]:m[5]])
			return EpisodeInfo{Season: s, Episode: e, Title: episodeTitle(name[m[5]:]), OK: true}
		}
	}
	for _, re := range []*regexp.Regexp{dilRe, dilWordRe} {
		if m := re.FindStringSubmatchIndex(name); m != nil {
			e, _ := strconv.Atoi(name[m[2]:m[3]])
			rest := name[m[3]:]
			if re == dilRe {
				rest = strings.TrimLeft(rest, ". ")
				rest = strings.TrimPrefix(strings.TrimPrefix(rest, "díl"), "Díl")
			}
			return EpisodeInfo{Episode: e, Title: episodeTitle(rest), OK: true}
		}
	}
	return EpisodeInfo{}
}

// LooksLikeEpisode řekne, jestli název nese číslo sezóny a dílu (SxxEyy, NxNN).
// Podle toho se soubor mimo adresáře filmů a seriálů zařadí jako díl.
func LooksLikeEpisode(name string) bool {
	return sxxexxRe.MatchString(name) || nxnnRe.MatchString(name)
}

func episodeTitle(rest string) string {
	t := cleanTitle(rest)
	if m := releaseTagRe.FindStringIndex(" " + t); m != nil {
		if m[0] == 0 {
			return ""
		}
		t = cleanTitle(t[:m[0]-1])
	}
	return t
}

// ParseSeasonDir vrátí číslo řady z názvu adresáře ("S03", "Season 3",
// "Série 2", "2. řada").
func ParseSeasonDir(name string) (int, bool) {
	name = strings.TrimSpace(name)
	for _, re := range []*regexp.Regexp{seasonRe, seasonRe2} {
		if m := re.FindStringSubmatch(name); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n, true
		}
	}
	return 0, false
}

// languageCodes převádí kódy a názvy jazyků z titulků a tagů streamů na ISO 639-1.
var languageCodes = map[string]string{
	"cs": "cs", "cz": "cs", "cze": "cs", "ces": "cs", "czech": "cs", "cesky": "cs", "česky": "cs",
	"sk": "sk", "slo": "sk", "slk": "sk", "slovak": "sk",
	"en": "en", "eng": "en", "english": "en",
	"de": "de", "ger": "de", "deu": "de", "german": "de",
	"fr": "fr", "fre": "fr", "fra": "fr", "french": "fr",
	"es": "es", "spa": "es", "spanish": "es",
	"it": "it", "ita": "it", "italian": "it",
	"pl": "pl", "pol": "pl", "polish": "pl",
	"hu": "hu", "hun": "hu", "ru": "ru", "rus": "ru",
	"ja": "ja", "jpn": "ja", "pt": "pt", "por": "pt",
}

// NormalizeLanguage vrátí kód jazyka ISO 639-1, nebo "" pro neznámý.
func NormalizeLanguage(s string) string {
	return languageCodes[strings.ToLower(strings.TrimSpace(s))]
}

// SubtitleMatch řekne, jestli soubor titulků patří k videu se jménem
// videoStem, a vrátí jazyk a příznak vynucených titulků. Páruje "Film.srt",
// "Film.cs.srt", "Film.forced.cze.srt" i "Film_cz.srt".
func SubtitleMatch(videoStem, subFile string) (lang string, forced bool, ok bool) {
	ext := strings.ToLower(filepath.Ext(subFile))
	if _, known := SubtitleExts[ext]; !known {
		return "", false, false
	}
	stem := strings.TrimSuffix(subFile, filepath.Ext(subFile))
	if !strings.HasPrefix(strings.ToLower(stem), strings.ToLower(videoStem)) {
		return "", false, false
	}
	rest := stem[len(videoStem):]
	if rest != "" && !strings.ContainsRune("._- ", rune(rest[0])) {
		return "", false, false // "Film 2.srt" nepatří k "Film"
	}
	for _, part := range strings.FieldsFunc(strings.ToLower(rest), func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	}) {
		if part == "forced" || part == "vynucene" {
			forced = true
			continue
		}
		if l := NormalizeLanguage(part); l != "" && lang == "" {
			lang = l
		}
	}
	return lang, forced, true
}
