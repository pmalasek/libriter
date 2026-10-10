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

func TestParseSeasonDir(t *testing.T) {
	ok := map[string]int{"S01": 1, "S3": 3, "Season 2": 2, "Série 4": 4, "2. řada": 2, "Sezóna 10": 10, "season_05": 5}
	for in, want := range ok {
		if got, found := ParseSeasonDir(in); !found || got != want {
			t.Errorf("ParseSeasonDir(%q) = %d,%v, chci %d", in, got, found, want)
		}
	}
	for _, in := range []string{"Extras", "Docent", "S01E01"} {
		if _, found := ParseSeasonDir(in); found {
			t.Errorf("ParseSeasonDir(%q) našel řadu", in)
		}
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
