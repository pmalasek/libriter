package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"mediater/internal/core/textfold"
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
	// Také za "=" a před "," nebo "CZ": "Neřízená střela =2010-DVD", "Zbraň 1-1987,USA", "2-1991CZ".
	yearTokenRe = regexp.MustCompile(`(?:^|[\s._\-=,])((?:19|20)\d{2})(?:$|[\s._\-,)]|(?i:cz))`)
	// Značky vydání, za kterými už název nepokračuje.
	releaseTagRe = regexp.MustCompile(`(?i)(?:^|[\s._\-\[(])(` + strings.Join([]string{
		`480p`, `576p`, `720p`, `1080[pi]`, `2160p`, `4k`, `uhd`, `fullhd`, `hd(?:tv|rip)?`,
		`br-?rip`, `bd-?rip`, `blu-?ray`, `bluray`, `web-?dl`, `web-?rip`, `dvd-?rip`, `dvd(?:scr|r)?`,
		`hdtv`, `x26[45]`, `h\.?26[45]`, `hevc`, `xvid`, `divx`, `ac3`, `dts`, `aac`, `mp3`,
		`cz(?:-?dab(?:ing)?\.?)?`, `sk`, `dab`, `dabing`, `titulky`, `cz-?tit(?:ulky)?`, `eng`, `multi`, `remux`,
		`proper`, `repack`, `extended`, `unrated`, `avi`, `novink[ay]`, `pres[\s._\-]+multiload`, `multiload`,
		`csfd`, `nejstahovanejsi`, `(?:super|top|vysoka)[\s._\-]+kvalit[aou]`,
	}, "|") + `)(?:$|[\s._\-\]),])`)
	// Žánry, kterými názvy souborů často pokračují ("Past-Krimi-Thriller-CZ").
	// Název jimi nikdy nezačíná (film "Drama" zůstane).
	genreTagRe = regexp.MustCompile(`(?i)[\s._\-(,]+(` + strings.Join([]string{
		`komedie`, `drama`, `ak[čc]n[íiýy]`, `thriller`, `krimi`, `horor`, `sci-fi`, `fanta[sz]y`,
		`romanti[cč]k[ýyáé]`, `dobrodru[žz]n[ýyáé]`, `v[áa]le[čc]n[ýyáée]`, `[žz]ivotopisn[ýyáé]`,
		`mysteri[óo]zn[íi]`, `western`, `poeti[cč]k[ýyáé]`, `road[\s._\-]+movie`,
		`[čc]esk[áaýyée][\s._\-]+(?:film|komedie|drama)`,
	}, "|") + `)(?:$|[\s._\-,)])`)
	// Pomlčka s mezerou aspoň z jedné strany odděluje název: "Včera -Yesterday".
	looseDashRe = regexp.MustCompile(`\s+-\s*|\s*-\s+`)
	// Poznámka v závorce bez roku: "Počátek (Inception)", "(Jiří Strach)".
	parenNoteRe  = regexp.MustCompile(`[\s._\-]+[(\[]([^()\[\]]*)(?:[)\]]|$)`)
	separatorsRe = regexp.MustCompile(`[._]+`)
	spacesRe     = regexp.MustCompile(`\s+`)
	// Úvodní značky bez roku: "!(Dokument)-Název", "[CZ] Název", "_ Název".
	leadingTagRe  = regexp.MustCompile(`^[^\pL\pN(\[]*(?:[(\[][^)\]\d]*[)\]][^\pL\pN(\[]*)+`)
	leadingJunkRe = regexp.MustCompile(`^[^\pL\pN(\[]+`)
	// Pořadové číslo složky ve sbírce: "02. Putování s dinosaury".
	ordinalRe = regexp.MustCompile(`^0\d{1,2}[.)]\s+`)
	// Díl filmu rozděleného do více souborů: "pelisky.cd1", "Film - part 2", "Film (disk1)".
	partSuffixRe = regexp.MustCompile(`(?i)[\s._\-(\[]+(?:cd|part|pt|dis[ck]|dvd)[\s._\-]*([1-9])[)\]]?$`)
)

// StripPart odstraní z názvu souboru označení dílu filmu (cd1, part2,
// disk1) a vrátí zbytek a číslo dílu (0 = soubor není díl).
func StripPart(stem string) (string, int) {
	m := partSuffixRe.FindStringSubmatchIndex(stem)
	if m == nil || m[0] == 0 {
		return stem, 0
	}
	n, _ := strconv.Atoi(stem[m[2]:m[3]])
	return stem[:m[0]], n
}

// ParseMovie rozebere název souboru nebo adresáře filmu (bez přípony) na
// název, původní název a rok. Zvládá české konvence "Název (rok - režisér)",
// "Český název - Original Title (rok)" i scénové "Název.2018.720p.x264…".
func ParseMovie(name string) MovieInfo {
	// Názvy souborů z macOS mívají diakritiku rozloženou (NFD).
	name = norm.NFC.String(strings.TrimSpace(name))
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
	}
	// Značky vydání a žánry stojí i před rokem ("Narodni_trida__CZ_dabing__2019").
	for _, re := range []*regexp.Regexp{releaseTagRe, genreTagRe} {
		if m := re.FindStringIndex(title); m != nil && m[0] > 0 {
			title = title[:m[0]]
		}
	}
	var note string
	if m := parenNoteRe.FindStringSubmatchIndex(title); m != nil && m[0] > 0 {
		note = cleanTitle(title[m[2]:m[3]])
		title = title[:m[0]] + " " + title[m[1]:]
	}

	// "Český název - Original Title"; samotné číslo ("RED - 1", "1 - Deník
	// Bridget Jonesové") je pořadí ve sbírce, ne název.
	// Název bez mezer s víc pomlčkami ("oko-dravce-czdab-2008") má pomlčky
	// jako oddělovač i v části, která po uříznutí zbyla.
	if !strings.ContainsAny(name, " _") && strings.Count(name, "-") >= 2 {
		title = strings.ReplaceAll(title, "-", " ")
	}
	segs := strings.Split(looseDashRe.ReplaceAllString(cleanTitle(title), " - "), " - ")
	var names []string
	for _, seg := range segs {
		if seg = strings.TrimSpace(seg); seg != "" && (len(segs) == 1 || !allDigits(seg)) {
			names = append(names, seg)
		}
	}
	if len(names) > 0 {
		info.Title, info.OriginalTitle = names[0], strings.Join(names[1:], " - ")
	}
	if info.OriginalTitle == "" && info.Title != "" {
		info.OriginalTitle = note
	}
	// Český a anglický název bez oddělovače: "Mumie The Mummy".
	if info.OriginalTitle == "" {
		if i := strings.Index(info.Title, " The "); i > 0 {
			info.Title, info.OriginalTitle = info.Title[:i], info.Title[i+1:]
		}
	}
	if info.Title == "" {
		info.Title = cleanTitle(name)
	}
	return info
}

func allDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
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
	// Loose: číslo je jen samostatné číslo v názvu (volné pravidlo), ne
	// "S01E02" ani "2. díl" – může jít o číslování přes všechny řady.
	Loose bool
}

var (
	sxxexxRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,2})[\s._-]*e(\d{1,3})(?:$|[^0-9])`)
	nxnnRe   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d{1,2})x(\d{1,3})(?:$|[^0-9])`)
	// "cali.s6x01", "Mentalista S0421".
	snxnnRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,2})x(\d{1,3})(?:$|[^0-9])`)
	snnnnRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{2})(\d{2})(?:$|[^0-9])`)
	// "02. díl", "3 díl", "4.dil", "Hniezdo_11.diel".
	dilRe     = regexp.MustCompile(`(?i)(?:^|[^0-9])(\d{1,3})\.?[\s._-]*(díl|dil|diel)(?:$|[^\pL])`)
	dilWordRe = regexp.MustCompile(`(?i)(?:^|[^\pL])(?:díl|dil|diel|epizoda|episode|ep|e)[\s._-]*(\d{1,3})(?:$|[^0-9])`)
	// Řada a díl spojené pomlčkou: "08-14.Tajna komora", "Pernikovy-tata-04-13-Ztrata".
	pairRe = regexp.MustCompile(`(?:^|[\s._-])(\d{2})-(\d{2,3})(?:$|[\s._\-])`)
	// Řada a díl jako čtyři číslice: "trp0324", "0501", "Kriminálka Anděl - 0201".
	ssEERe = regexp.MustCompile(`(?:^|[^0-9])(\d{2})(\d{2})(?:$|[^0-9])`)
	// Římské číslo dílu hned za názvem seriálu: "Gagman IV.".
	romanRe = regexp.MustCompile(`^[\s._-]*([IVX]{1,5})(?:$|[\s._-])`)
	// Díl X z Y: "Případ_pro_exorcistu_2ze3".
	partOfRe = regexp.MustCompile(`(?i)(?:^|[^0-9])(\d{1,3})[\s._]*(?:ze|of)[\s._]*\d{1,3}(?:$|[^0-9])`)
	// Samostatné číslo kdekoli v názvu: "Colombo 07.Vražda", "Hospoda-47-klientka",
	// "025. Loupeznik", "Svatby v Benátkách (19) - Navždy spolu", "Hraběnky • 13 • Milionář".
	numTokenRe = regexp.MustCompile(`(?:^|[\s._\-(\[•,#])(\d{1,3})(?:$|[\s._\-()\[\]•,!])`)

	seasonWords = `s|season|série|serie|ser|sezóna|sezona|řada|rada`
	// Řada na konci názvu složky, i za názvem seriálu: "S03", "ser.04",
	// "Terapie S01", "Season 3", "Battlestar Galactica 1. série", "2. řada".
	seasonRe  = regexp.MustCompile(`(?i)(?:^|[\s._-])(?:` + seasonWords + `)[\s._-]*(\d{1,2})$`)
	seasonRe2 = regexp.MustCompile(`(?i)(?:^|[\s._-])(\d{1,2})\.?\s*(?:série|serie|sezóna|sezona|řada|rada|season)$`)
	// Roky za řadou: "S03 (2012)", "S02 (2010-2011)".
	trailingParenRe = regexp.MustCompile(`\s*[(\[][^)\]]*[)\]]\s*$`)
)

// ParseEpisode rozebere název souboru dílu (bez přípony). Rozpozná
// "S01E02", "1x02", "02. díl" i "Díl 2"; název dílu je text za číslem.
// Když nic z toho nenajde, vezme číslo, které v názvu stojí samostatně
// (na konci, "08-14", "2ze3", nebo první takové číslo). Číslo z názvu
// seriálu ("Babylon 5") odstraní ParseEpisodeIn.
func ParseEpisode(name string) EpisodeInfo {
	if ep := parseEpisodeStrict(name); ep.OK {
		return ep
	}
	return parseEpisodeLoose(name, 0, false)
}

// ParseEpisodeIn je ParseEpisode pro soubor ve složce seriálu show a řady
// season: volná pravidla hledají číslo až za názvem seriálu, pokud jím název
// souboru začíná, a číslo řady hned za ním přeskočí ("Kriminálka Anděl 3
// (2012) 12 - Tajné pohřby", "Dům na kopci 1 - 9 - Hrůzostrašné sny").
func ParseEpisodeIn(name, show string, season int) EpisodeInfo {
	if ep := parseEpisodeStrict(name); ep.OK {
		return ep
	}
	if rest := stripShowPrefix(name, show); rest != name {
		if ep := parseEpisodeLoose(rest, season, true); ep.OK {
			return ep
		}
		if m := romanRe.FindStringSubmatch(rest); m != nil {
			if e := romanValue(m[1]); e > 0 {
				return EpisodeInfo{Episode: e, OK: true, Loose: true}
			}
		}
	}
	return parseEpisodeLoose(name, season, false)
}

// ParseEmbeddedTitle hledá číslo dílu v názvu vloženém v souboru (tag title).
// Ten bývá nespolehlivý ("MASH", "Ripped by Pulp3"), proto jen jednoznačné
// zápisy: "S03x07", "8x14", "3 díl".
func ParseEmbeddedTitle(title string) EpisodeInfo {
	return parseEpisodeStrict(title)
}

func parseEpisodeStrict(name string) EpisodeInfo {
	for _, re := range []*regexp.Regexp{sxxexxRe, snxnnRe, snnnnRe, nxnnRe} {
		if m := re.FindStringSubmatchIndex(name); m != nil {
			s, _ := strconv.Atoi(name[m[2]:m[3]])
			e, _ := strconv.Atoi(name[m[4]:m[5]])
			return EpisodeInfo{Season: s, Episode: e, Title: episodeTitle(name[m[5]:]), OK: true}
		}
	}
	if m := dilRe.FindStringSubmatchIndex(name); m != nil {
		e, _ := strconv.Atoi(name[m[2]:m[3]])
		return EpisodeInfo{Episode: e, Title: episodeTitle(name[m[5]:]), OK: true}
	}
	if m := dilWordRe.FindStringSubmatchIndex(name); m != nil {
		e, _ := strconv.Atoi(name[m[2]:m[3]])
		return EpisodeInfo{Episode: e, Title: episodeTitle(name[m[3]:]), OK: true}
	}
	return EpisodeInfo{}
}

// parseEpisodeLoose hledá samostatné číslo: řadu a díl "08-14", čtyři
// číslice začínající řadou season ("trp0324"), díl "2ze3", jinak první
// samostatné číslo. afterShow: name je zbytek za názvem seriálu, takže
// číslo řady na jeho začátku se přeskočí. Bez něj by se díl čísloval podle
// pořadí souborů, které se při postupném kopírování mění. Za číslem bývají
// značky vydání ("Desate-Kralovstvi-3-cz", "AC3 5.1"), proto se nejdřív
// hledá v názvu bez nich.
func parseEpisodeLoose(name string, season int, afterShow bool) EpisodeInfo {
	name = strings.TrimSpace(name)
	candidates := []string{name}
	if loc := releaseTagRe.FindStringIndex(name); loc != nil && loc[0] > 0 {
		candidates = []string{strings.TrimSpace(name[:loc[0]]), name}
	}
	// Jen když první číslo je řada ze složky: "Díly 12-16" ani
	// "topinka-13-13" řadu neurčují.
	if m := pairRe.FindStringSubmatch(candidates[0]); m != nil {
		s, _ := strconv.Atoi(m[1])
		if e, _ := strconv.Atoi(m[2]); s == season && e > 0 {
			return EpisodeInfo{Season: s, Episode: e, OK: true, Loose: true}
		}
	}
	if season > 0 {
		for _, m := range ssEERe.FindAllStringSubmatch(candidates[0], -1) {
			// "1981" je rok, ne 19. řada.
			if m[1] == "19" || m[1] == "20" {
				continue
			}
			s, _ := strconv.Atoi(m[1])
			if e, _ := strconv.Atoi(m[2]); s == season && e > 0 {
				return EpisodeInfo{Season: s, Episode: e, OK: true, Loose: true}
			}
		}
	}
	for _, c := range candidates {
		if m := partOfRe.FindStringSubmatch(c); m != nil {
			if e, _ := strconv.Atoi(m[1]); e > 0 {
				return EpisodeInfo{Episode: e, OK: true, Loose: true}
			}
		}
		nums := numTokens(c)
		// Číslo řady před číslem dílu: "Seriál 3 (2012) 12". Číslo s nulou
		// na začátku je díl ("Okres na severu 01_13" = díl 1 ze 13), stejně
		// jako číslo, za kterým je text ("Druhy dech 1.Telecí léta … 698").
		if afterShow && season > 0 && len(nums) > 1 && nums[0].text == strconv.Itoa(season) &&
			!hasLetter(c[nums[0].end:nums[1].start]) {
			nums = nums[1:]
		}
		if len(nums) > 0 {
			if e, _ := strconv.Atoi(nums[0].text); e > 0 {
				return EpisodeInfo{Episode: e, OK: true, Loose: true}
			}
		}
	}
	return EpisodeInfo{}
}

// romanValue převede římské číslo (I–XXXIX) na číslo, 0 = neplatné.
func romanValue(s string) int {
	values := map[byte]int{'I': 1, 'V': 5, 'X': 10}
	n := 0
	for i := 0; i < len(s); i++ {
		v := values[s[i]]
		if i+1 < len(s) && v < values[s[i+1]] {
			n -= v
		} else {
			n += v
		}
	}
	return n
}

type numToken struct {
	text       string
	start, end int
}

func hasLetter(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

// numTokens vrátí samostatná čísla v názvu zleva doprava. Oddělovač mezi
// dvěma čísly patří oběma ("Seriál 3 12").
func numTokens(name string) []numToken {
	var out []numToken
	for at := 0; at < len(name); {
		m := numTokenRe.FindStringSubmatchIndex(name[at:])
		if m == nil {
			break
		}
		out = append(out, numToken{name[at+m[2] : at+m[3]], at + m[2], at + m[3]})
		at += m[3]
	}
	return out
}

var wordRe = regexp.MustCompile(`[\pL\pN]+`)

// stripShowPrefix odřízne ze začátku názvu název seriálu (bez ohledu na
// diakritiku, velikost písmen a oddělovače): "Desate-Kralovstvi-3-cz" ve
// složce "Desáté království" → "-3-cz". Když název seriálem nezačíná nebo
// by nic nezbylo, vrátí name beze změny.
func stripShowPrefix(name, show string) string {
	want := wordRe.FindAllString(show, -1)
	got := wordRe.FindAllStringIndex(name, -1)
	if len(want) == 0 || len(got) <= len(want) {
		return name
	}
	for i, w := range want {
		if textfold.Fold(w) != textfold.Fold(name[got[i][0]:got[i][1]]) {
			return name
		}
	}
	return name[got[len(want)-1][1]:]
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
// "Série 2", "2. řada", "ser.04", "S03 (2012)", "Terapie S01",
// "Battlestar Galactica 1. série").
func ParseSeasonDir(name string) (int, bool) {
	name = strings.TrimSpace(trailingParenRe.ReplaceAllString(name, ""))
	for _, re := range []*regexp.Regexp{seasonRe, seasonRe2} {
		if m := re.FindStringSubmatch(name); m != nil {
			n, _ := strconv.Atoi(m[1])
			return n, true
		}
	}
	return 0, false
}

// ParseSeasonDirIn je ParseSeasonDir pro podsložku seriálu show: pozná
// i řadu zapsanou jen číslem za názvem seriálu ("Californication 6").
func ParseSeasonDirIn(name, show string) (int, bool) {
	if n, ok := ParseSeasonDir(name); ok {
		return n, true
	}
	rest := strings.TrimSpace(trailingParenRe.ReplaceAllString(stripShowPrefix(name, show), ""))
	if rest != name {
		rest = strings.Trim(rest, " ._-")
		if n, err := strconv.Atoi(rest); err == nil && n >= 0 && n < 100 {
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
