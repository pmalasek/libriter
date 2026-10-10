package plugin

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

// VideoCredits jsou tvůrci a obsazení.
type VideoCredits struct {
	Directors []string     `json:"directors,omitempty"`
	Writers   []string     `json:"writers,omitempty"`
	Creators  []string     `json:"creators,omitempty"`
	Cast      []CastMember `json:"cast,omitempty"`
}

// CastMember je herec a jeho role.
type CastMember struct {
	Name      string `json:"name"`
	Character string `json:"character,omitempty"`
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
