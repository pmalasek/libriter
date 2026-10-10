package scanner

import "testing"

// Názvy jsou ze skutečné knihovny (movies/ v repozitáři) a z běžných konvencí.
func TestParseMovie(t *testing.T) {
	cases := []struct {
		in    string
		title string
		orig  string
		year  int
	}{
		{"21 gramů (2003 - Alejandro González Iñárritu)", "21 gramů", "", 2003},
		{"Amadeus-1984-CZ-Dab.-2.0-256kbps-byzld", "Amadeus", "", 1984},
		{"22.míle.2018.Mile.22.BRRip.720p.x264.AC3.2.0.CZ.Dabing", "22 míle", "", 2018},
		{"Křídla vášně - The Wings of the Dove (1997)", "Křídla vášně", "The Wings of the Dove", 1997},
		{"Altamira (2016 FullHD-H264-BLRip ENG CZ-dab_8k)_HD", "Altamira", "", 2016},
		{"The.Matrix.1999.1080p.BluRay.x264", "The Matrix", "", 1999},
		{"Pelíšky (1999)", "Pelíšky", "", 1999},
		{"1917 (2019)", "1917", "", 2019},
		{"2012.2009.720p", "2012", "", 2009},
		{"Spider-Man (2002)", "Spider-Man", "", 2002},
		{"Vesničko má středisková", "Vesničko má středisková", "", 0},
		{"Obecná škola CZ Dabing 720p", "Obecná škola", "", 0},
		{"!(Dokument)-Nigel-Marven-a-Invaze-Obřích-Hadů-cz", "Nigel Marven a Invaze Obřích Hadů", "", 0},
		{"02. Putovani s dinosaury", "Putovani s dinosaury", "", 0},
		{"[Dokument] Planeta Země (2006)", "Planeta Země", "", 2006},
		{"Putovani.s.dinosaury.e01.1999.PDTV.XviD.CZ", "Putovani s dinosaury e01", "", 1999},
		// Značky a žánry před rokem.
		{"Narodni_trida__CZ_dabing__2019", "Narodni trida", "", 2019},
		{"Past-Krimi-Thriller-CZ-dabing-1999", "Past", "", 1999},
		{"Odstřelovač_CZdab_Akční__Thriller__Krimi__Mysteriózní_USA__2007", "Odstřelovač", "", 2007},
		{"oko-dravce-czdab-akcni-mysteriozni-usa-nemecko-2008", "oko dravce", "", 2008},
		{"Tátova volha-česká komedie_drama  2018", "Tátova volha", "", 2018},
		{"Drama", "Drama", "", 0},
		// Rok za "=" a před ",".
		{"Neřízená střela =2010-DVD-CZ", "Neřízená střela", "", 2010},
		{"Smrtonosna zbran 2-1989,USA-akcny,krimi,thriller", "Smrtonosna zbran 2", "", 1989},
		// Poznámka v závorce, podtitul a dvojjazyčný název.
		{"Počátek (Inception) CZ AVI", "Počátek", "Inception", 0},
		{"Mizerové - Na život a na smrt", "Mizerové", "Na život a na smrt", 0},
		{"Mumie.The.Mummy.2017.1080p.BluRay.x264", "Mumie", "The Mummy", 2017},
		{"Včera_-Yesterday_cz_dabing__2019__BRRip", "Včera", "Yesterday", 2019},
		// Samotné číslo za/před pomlčkou je pořadí, ne původní název.
		{"Dej si pohov kámoši - 1", "Dej si pohov kámoši", "", 0},
		{"RED - 1", "RED", "", 0},
		{"1 - Deník Bridget Jonesové", "Deník Bridget Jonesové", "", 0},
		{"Piráti z Karibiku - 3 - Na konci světa cz (2007)", "Piráti z Karibiku", "Na konci světa", 2007},
		// Rozložená diakritika (NFD) se složí.
		{"Terminál", "Terminál", "", 0},
		{"X-Men 1 (2000)-CZ dabing; akční", "X-Men 1", "", 2000},
	}
	for _, c := range cases {
		got := ParseMovie(c.in)
		if got.Title != c.title || got.OriginalTitle != c.orig || got.Year != c.year {
			t.Errorf("ParseMovie(%q) = %+v, chci {%q %q %d}", c.in, got, c.title, c.orig, c.year)
		}
	}
}

func TestParseEpisode(t *testing.T) {
	cases := []struct {
		in      string
		season  int
		episode int
		title   string
	}{
		{"Docent - 01x02 - Preso s mlíčkem", 1, 2, "Preso s mlíčkem"},
		{"Docent - 02x03 - Zvláštní opatření", 2, 3, "Zvláštní opatření"},
		{"Docent - 02. díl - Příliš mnoho podezřelých", 0, 2, "Příliš mnoho podezřelých"},
		{"Docent - 01. díl - Únos", 0, 1, "Únos"},
		{"Show.Name.S03E07.Episode.Title.720p.WEB-DL", 3, 7, "Episode Title"},
		{"show_name_s1e10", 1, 10, ""},
		{"Nemocnice na kraji města - Díl 4", 0, 4, ""},
		{"Bitva_o_nebe_nad_Británií_4", 0, 4, ""},
		{"Planeta Země - 03", 0, 3, ""},
		{"Desate-Kralovstvi-3-cz", 0, 3, ""},
		{"Show 4 720p", 0, 4, ""},
		{"Show-12-eng", 0, 12, ""},
		{"Labyrint 06.díl", 0, 6, ""},
		{"Doktor z vejminku - 3 díl - Láska a úklady", 0, 3, "Láska a úklady"},
		{"Atentat-4.dil-Pravo-CZ", 0, 4, "Pravo"},
		{"Hniezdo_11.diel", 0, 11, ""},
		{"Mentalista S0421 Rubínové střevíčky", 4, 21, "Rubínové střevíčky"},
		{"cali.s6x01.unforgiven.cz", 6, 1, "unforgiven"},
		{"01", 0, 1, ""},
		{"01. Okupace", 0, 1, ""},
		{"025. Loupeznik", 0, 25, ""},
		{"Colombo 07.Vražda.podle.plánu.1972", 0, 7, ""},
		{"Hospoda-47-klientka", 0, 47, ""},
		{"Svatby v Benátkách (19) - Navždy spolu", 0, 19, ""},
		{"Hraběnky • 13 • Milionář", 0, 13, ""},
		{"Pozadí_událostí__3_", 0, 3, ""},
		{"Případ_pro_exorcistu_2ze3__2015__Detektivové_od_Nejsvětější_trojice", 0, 2, ""},
		{"Okres na severu 11_13 TV seriál 1981, CZ", 0, 11, ""},
		{"Vraždy.u.jezera.8.Štvanice.HDTV.CZ", 0, 8, ""},
	}
	for _, c := range cases {
		got := ParseEpisode(c.in)
		if !got.OK || got.Season != c.season || got.Episode != c.episode || got.Title != c.title {
			t.Errorf("ParseEpisode(%q) = %+v, chci {%d %d %q}", c.in, got, c.season, c.episode, c.title)
		}
	}
	if got := ParseEpisode("Docent"); got.OK {
		t.Errorf("ParseEpisode bez čísla = %+v", got)
	}
}

func TestParseEpisodeIn(t *testing.T) {
	cases := []struct {
		name, show      string
		season, episode int
	}{
		{"Babylon 5 - 03 - Návrat", "Babylon 5", 0, 3},
		{"Desate-Kralovstvi-3-cz", "Desáté království", 1, 3},
		{"Vodník 2 (2019)", "Vodník", 0, 2},
		{"Jiný seriál 4", "Babylon 5", 0, 4},
		{"Kriminálka Anděl 3 (2012) 12 - Tajné pohřby", "Kriminálka Anděl", 3, 12},
		{"Kriminálka Anděl 3 (2012) 03 - Pohřbená zaživa", "Kriminálka Anděl", 3, 3},
		{"Dům_na_kopci_1_-_10_-_Skličující_ticho", "Dům na kopci", 1, 10},
		{"Na_-_vodě_-_8(1)", "Na vode", 1, 8},
		{"03. Exodus část 1.", "Battlestar Galactica", 3, 3},
		{"Okres na severu 01_13 TV seriál 1981, CZ", "Okres na severu", 1, 1},
		{"trp0324", "Terapie", 3, 24},
		{"0507", "Útěk z vězení", 5, 7},
		{"Kriminálka Anděl - 0213 - Pistole", "Kriminálka Anděl", 2, 13},
		{"Gagman IV.", "Gagman", 1, 4},
		{"08-14.Tajna komora", "Přátelé", 8, 14},
		{"Pernikovy-tata-04-13-Ztrata-tvare--cz-dabing", "Breaking Bad", 4, 13},
		{"THE_OUTSIDER_01-05_CZ_dabing_2020", "Outsider", 1, 5},
		{"Druhy-dech---1.Telecí-léta-(1988)-MONTY-698", "Druhý dech", 1, 1},
		{"strazmistr-topinka-13-13-zlocin-na-svatbe", "Strážmistr Topinka", 1, 13},
		{"Česká soda_Díly 12-16 (1995)", "Česká soda", 1, 12},
		{"Okres na severu 1981", "Okres na severu", 19, 0},
	}
	for _, c := range cases {
		if got := ParseEpisodeIn(c.name, c.show, c.season); got.OK != (c.episode > 0) || got.Episode != c.episode {
			t.Errorf("ParseEpisodeIn(%q, %q) = %+v, chci díl %d", c.name, c.show, got, c.episode)
		}
	}
}

func TestParseEmbeddedTitle(t *testing.T) {
	for title, want := range map[string]int{
		"Gumy.S01E09.1080p.WebRip.x.264.Akim":           9,
		"Pan casu S03x07 - 42 dvdrip cz (warezfilm.cz)": 7,
		"p��tel�: 8x14 - tajemn� komora (2002)":         14,
		"3 díl - Láska a úklady":                        3,
		"MASH":                                          0,
		"Ripped by Pulp3 @ Warforum.cz":                 0,
		"MISTR_A_MARKETKA_1_2.Title1":                   0,
	} {
		got := ParseEmbeddedTitle(title)
		if got.Episode != want || got.OK != (want > 0) {
			t.Errorf("ParseEmbeddedTitle(%q) = %+v, chci %d", title, got, want)
		}
	}
}

func TestParseSeasonDir(t *testing.T) {
	ok := map[string]int{"S01": 1, "S3": 3, "Season 2": 2, "Série 4": 4, "2. řada": 2, "Sezóna 10": 10, "season_05": 5,
		"ser.04": 4, "S03 (2012)": 3, "S02 (2010-2011)": 2, "Terapie S01": 1, "Battlestar Galactica 1. série": 1}
	for in, want := range ok {
		if got, found := ParseSeasonDir(in); !found || got != want {
			t.Errorf("ParseSeasonDir(%q) = %d,%v, chci %d", in, got, found, want)
		}
	}
	for _, in := range []string{"Extras", "Docent", "S01E01", "Californication 6", "Babylon 5"} {
		if _, found := ParseSeasonDir(in); found {
			t.Errorf("ParseSeasonDir(%q) našel řadu", in)
		}
	}
	if n, ok := ParseSeasonDirIn("Californication 6", "Californication"); !ok || n != 6 {
		t.Errorf("ParseSeasonDirIn(Californication 6) = %d,%v", n, ok)
	}
	if _, ok := ParseSeasonDirIn("Extras 2", "Californication"); ok {
		t.Error("ParseSeasonDirIn(Extras 2) našel řadu")
	}
}

func TestSubtitleMatch(t *testing.T) {
	stem := "Křídla vášně - The Wings of the Dove (1997)"
	cases := []struct {
		file   string
		lang   string
		forced bool
		ok     bool
	}{
		{stem + ".srt", "", false, true},
		{stem + ".cs.srt", "cs", false, true},
		{stem + ".forced.cze.srt", "cs", true, true},
		{stem + "_eng.srt", "en", false, true},
		{stem + ".avi.vsmeta", "", false, false},
		{stem + "2.srt", "", false, false},
		{"Jiný film.srt", "", false, false},
	}
	for _, c := range cases {
		lang, forced, ok := SubtitleMatch(stem, c.file)
		if lang != c.lang || forced != c.forced || ok != c.ok {
			t.Errorf("SubtitleMatch(%q) = %q,%v,%v, chci %q,%v,%v", c.file, lang, forced, ok, c.lang, c.forced, c.ok)
		}
	}
}

func TestIgnored(t *testing.T) {
	for _, n := range []string{"@eaDir", ".DS_Store", "#recycle", "film.avi.vsmeta", "film.mkv.part", "sample.mkv", "Film-sample.mkv"} {
		if !Ignored(n) {
			t.Errorf("Ignored(%q) = false", n)
		}
	}
	for _, n := range []string{"Docent", "Film.mkv", "Samples of Life (2001).mkv"} {
		if Ignored(n) {
			t.Errorf("Ignored(%q) = true", n)
		}
	}
}

func TestStripPart(t *testing.T) {
	cases := []struct {
		in   string
		base string
		part int
	}{
		{"pelisky.cd1", "pelisky", 1},
		{"Pelíšky (1999) CD2", "Pelíšky (1999)", 2},
		{"Film - part 2", "Film", 2},
		{"Film_disk1", "Film", 1},
		{"Film (pt3)", "Film", 3},
		{"Apollo 13", "Apollo 13", 0},
		{"cd1", "cd1", 0},
	}
	for _, c := range cases {
		base, part := StripPart(c.in)
		if base != c.base || part != c.part {
			t.Errorf("StripPart(%q) = %q, %d; chci %q, %d", c.in, base, part, c.base, c.part)
		}
	}
}

func TestMovieKey(t *testing.T) {
	cases := []struct {
		rel   string
		key   string
		title string
		year  int
	}{
		{"Filmy/Divadelní hry/Lakomec (1980).avi", "Filmy/Divadelní hry/Lakomec (1980)", "Lakomec", 1980},
		{"Filmy/Detektivky Hany Proškové/Vražda v zahradě.avi", "Filmy/Detektivky Hany Proškové/Vražda v zahradě", "Vražda v zahradě", 0},
		{"Filmy/Pelíšky (1999)/pelisky.cd2.avi", "Filmy/Pelíšky (1999)/pelisky", "pelisky", 0},
		{"Filmy/Amadeus (1984).mkv", "Filmy/Amadeus (1984)", "Amadeus", 1984},
	}
	for _, c := range cases {
		key, info := movieKeyOf(c.rel)
		if key != c.key || info.Title != c.title || info.Year != c.year {
			t.Errorf("movieKeyOf(%q) = %q, %+v; chci %q, %q, %d", c.rel, key, info, c.key, c.title, c.year)
		}
	}
}

func TestParseProbeTitle(t *testing.T) {
	p := parseProbe([]byte(`{"streams":[],"format":{"format_name":"avi","tags":{"TITLE":" Pan casu S03x07 "}}}`))
	if p.Title != "Pan casu S03x07" {
		t.Errorf("Title = %q", p.Title)
	}
}
