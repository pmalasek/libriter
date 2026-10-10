package plugin

import "encoding/json"

// VideoMetadata je položka fetch pro druhy movie a show.
type VideoMetadata struct {
	ID            string `json:"id"`
	URL           string `json:"url,omitempty"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title,omitempty"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
	Tagline       string `json:"tagline,omitempty"`
	// Status je stav seriálu tak, jak ho uvádí zdroj („Ended“, „Returning Series“).
	Status     string `json:"status,omitempty"`
	RuntimeMin int    `json:"runtime_min,omitempty"`
	// Rating je hodnocení zdroje v procentech (0–100).
	Rating      int      `json:"rating,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	PosterURL   string   `json:"poster_url,omitempty"`
	BackdropURL string   `json:"backdrop_url,omitempty"`
	// ExternalIDs jsou identifikátory u jiných databází: {"imdb": "tt0123",
	// "tmdb": "603", "csfd": "8365"}.
	ExternalIDs map[string]string `json:"external_ids,omitempty"`
	Credits     VideoCredits      `json:"credits"`
	// Seasons jsou čísla řad seriálu, která zdroj zná.
	Seasons []int `json:"seasons,omitempty"`
}

// VideoCredits jsou tvůrci a obsazení v pořadí, v jakém je uvádí zdroj.
type VideoCredits struct {
	Directors []Person `json:"directors,omitempty"`
	Writers   []Person `json:"writers,omitempty"`
	Creators  []Person `json:"creators,omitempty"`
	Cast      []Person `json:"cast,omitempty"`
}

// Person je osoba z obsazení nebo štábu. V JSON jde zapsat i jako prostý
// řetězec se jménem ("Lana Wachowski") – starší tvar protokolu.
type Person struct {
	// ID je identifikátor osoby u zdroje; podle něj server pozná stejnou
	// osobu v různých titulech.
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
	// ImageURL je fotka osoby; stahuje ji server (jen z image_hosts).
	ImageURL string `json:"image_url,omitempty"`
}

// UnmarshalJSON přijme objekt i prostý řetězec se jménem.
func (p *Person) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err == nil {
		*p = Person{Name: name}
		return nil
	}
	type plain Person
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*p = Person(v)
	return nil
}

// Names vrátí jména osob (pro testy a výpisy).
func Names(people []Person) []string {
	out := make([]string, len(people))
	for i, p := range people {
		out[i] = p.Name
	}
	return out
}

// SeasonMetadata je výstup příkazu season.
type SeasonMetadata struct {
	Number    int               `json:"number"`
	Name      string            `json:"name,omitempty"`
	Overview  string            `json:"overview,omitempty"`
	PosterURL string            `json:"poster_url,omitempty"`
	Episodes  []EpisodeMetadata `json:"episodes"`
}

// EpisodeMetadata je jeden díl řady.
type EpisodeMetadata struct {
	Number     int    `json:"number"`
	Title      string `json:"title,omitempty"`
	Overview   string `json:"overview,omitempty"`
	AirDate    string `json:"air_date,omitempty"` // YYYY-MM-DD
	StillURL   string `json:"still_url,omitempty"`
	RuntimeMin int    `json:"runtime_min,omitempty"`
}
