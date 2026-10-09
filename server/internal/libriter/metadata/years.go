package metadata

import (
	"regexp"
	"strconv"
	"time"
)

var (
	// Rok v běžném rozsahu autorských dat.
	yearRe = regexp.MustCompile(`\b(1[0-9]{3}|20[0-9]{2})\b`)
	// Rozmezí života: "1890–1938", "1890-1938", "1890 – 1938".
	lifeSpanRe = regexp.MustCompile(`\b(1[0-9]{3}|20[0-9]{2})\s*[–—-]\s*(1[0-9]{3}|20[0-9]{2})\b`)
)

// YearFromDate vytáhne rok z data zapsaného volným textem
// ("1890", "9.1. 1890", "1890-01-09", "9 January 1890"). 0 = nenalezeno.
func YearFromDate(date string) int {
	return plausibleYear(yearRe.FindString(date))
}

// YearsFromText odhadne roky narození a úmrtí z textu životopisu. Hledá
// nejdřív rozmezí ("1890–1938"); jinak vezme první rok jako narození.
//
// Je to odhad z volného textu, proto se používá jen jako záloha, když
// strukturovaná data roky nemají.
func YearsFromText(text string) (birth, death int) {
	if match := lifeSpanRe.FindStringSubmatch(text); match != nil {
		birth, death = plausibleYear(match[1]), plausibleYear(match[2])
		if birth > 0 && death >= birth {
			return birth, death
		}
	}

	return plausibleYear(yearRe.FindString(text)), 0
}

// plausibleYear převede text na rok a odmítne nesmyslné hodnoty.
func plausibleYear(s string) int {
	year, err := strconv.Atoi(s)
	if err != nil || year < 1000 || year > time.Now().Year() {
		return 0
	}
	return year
}
