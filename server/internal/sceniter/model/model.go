// Package model obsahuje typy videoknihovny Sceniter. JSON tvary zrcadlí
// sceniter-shared/src/types.ts.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Stavy spárování titulu se zdrojem metadat.
const (
	MatchUnmatched = "unmatched" // nespárováno – čeká na ruční zásah
	MatchAuto      = "auto"      // spárováno automaticky po skenu
	MatchManual    = "manual"    // spárováno ručně – scanner ho nepřepíše
	MatchIgnored   = "ignored"   // admin nechce párovat
)

// Druhy titulů v API (watchlist, hodnocení, rozkoukanost).
const (
	KindMovie   = "movie"
	KindShow    = "show"
	KindEpisode = "episode"
)

// Library je knihovna: podsložka VIDEO_ROOT s typem obsahu a zdrojem metadat.
type Library struct {
	ID   uuid.UUID `json:"id"`
	Dir  string    `json:"dir"`
	Name string    `json:"name"`
	// Kind je KindMovie nebo KindShow.
	Kind string `json:"kind"`
	// Language je jazyk metadat (ISO 639-1).
	Language string `json:"language"`
	// Provider je jméno pluginu, se kterým se tituly párují automaticky.
	Provider *string `json:"provider,omitempty"`
	// Icon je název ikony v menu (vybírá se z nabídky webu); nil = podle typu.
	Icon      *string   `json:"icon,omitempty"`
	Position  int       `json:"position"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// ItemCount je počet filmů nebo seriálů se soubory.
	ItemCount int `json:"item_count"`
}

// Credit je osoba z obsazení nebo štábu.
type Credit struct {
	Name      string `json:"name"`
	Role      string `json:"role"` // "cast" | "director" | "writer" | "creator"
	Character string `json:"character,omitempty"`
}

// Movie je film v knihovně.
type Movie struct {
	ID            uuid.UUID         `json:"id"`
	LibraryID     uuid.UUID         `json:"library_id"`
	Title         string            `json:"title"`
	OriginalTitle *string           `json:"original_title,omitempty"`
	Year          *int              `json:"year,omitempty"`
	Overview      *string           `json:"overview,omitempty"`
	Tagline       *string           `json:"tagline,omitempty"`
	RuntimeMin    *int              `json:"runtime_min,omitempty"`
	MetaProvider  *string           `json:"meta_provider,omitempty"`
	MetaID        *string           `json:"meta_id,omitempty"`
	ExternalIDs   map[string]string `json:"external_ids"`
	PosterPath    *string           `json:"poster_path,omitempty"`
	BackdropPath  *string           `json:"backdrop_path,omitempty"`
	Rating        *int              `json:"rating,omitempty"`
	Credits       []Credit          `json:"credits"`
	Genres        []string          `json:"genres"`
	MatchState    string            `json:"match_state"`
	SourceKey     string            `json:"source_key"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`

	// Odvozené pro výpisy: délka hlavního souboru a jeho ID.
	DurationSec *float64   `json:"duration_sec,omitempty"`
	MediaID     *uuid.UUID `json:"media_id,omitempty"`
}

// Show je seriál.
type Show struct {
	ID            uuid.UUID         `json:"id"`
	LibraryID     uuid.UUID         `json:"library_id"`
	Title         string            `json:"title"`
	OriginalTitle *string           `json:"original_title,omitempty"`
	Year          *int              `json:"year,omitempty"`
	Overview      *string           `json:"overview,omitempty"`
	Status        *string           `json:"status,omitempty"`
	MetaProvider  *string           `json:"meta_provider,omitempty"`
	MetaID        *string           `json:"meta_id,omitempty"`
	ExternalIDs   map[string]string `json:"external_ids"`
	PosterPath    *string           `json:"poster_path,omitempty"`
	BackdropPath  *string           `json:"backdrop_path,omitempty"`
	Rating        *int              `json:"rating,omitempty"`
	Credits       []Credit          `json:"credits"`
	Genres        []string          `json:"genres"`
	MatchState    string            `json:"match_state"`
	SourceKey     string            `json:"source_key"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`

	// Odvozené pro výpisy.
	SeasonCount  int `json:"season_count"`
	EpisodeCount int `json:"episode_count"`
}

// Season je řada seriálu.
type Season struct {
	ID         uuid.UUID `json:"id"`
	ShowID     uuid.UUID `json:"show_id"`
	Number     int       `json:"number"`
	Title      *string   `json:"title,omitempty"`
	Overview   *string   `json:"overview,omitempty"`
	PosterPath *string   `json:"poster_path,omitempty"`
	Episodes   []Episode `json:"episodes"`
}

// Episode je díl seriálu.
type Episode struct {
	ID         uuid.UUID `json:"id"`
	SeasonID   uuid.UUID `json:"season_id"`
	ShowID     uuid.UUID `json:"show_id"`
	Season     int       `json:"season"`
	Number     int       `json:"number"`
	Title      *string   `json:"title,omitempty"`
	Overview   *string   `json:"overview,omitempty"`
	AirDate    *string   `json:"air_date,omitempty"`
	StillPath  *string   `json:"still_path,omitempty"`
	RuntimeMin *int      `json:"runtime_min,omitempty"`
	CreatedAt  time.Time `json:"created_at"`

	// Odvozené: hlavní soubor dílu a název seriálu (pro výpisy napříč seriály).
	DurationSec *float64   `json:"duration_sec,omitempty"`
	MediaID     *uuid.UUID `json:"media_id,omitempty"`
	ShowTitle   string     `json:"show_title,omitempty"`
}

// MediaFile je fyzický soubor s videem.
type MediaFile struct {
	ID           uuid.UUID          `json:"id"`
	MovieID      *uuid.UUID         `json:"movie_id,omitempty"`
	EpisodeID    *uuid.UUID         `json:"episode_id,omitempty"`
	FilePath     string             `json:"file_path"`
	SizeBytes    int64              `json:"size_bytes"`
	ModTime      *time.Time         `json:"-"`
	Container    *string            `json:"container,omitempty"`
	DurationSec  *float64           `json:"duration_sec,omitempty"`
	Bitrate      *int64             `json:"bitrate,omitempty"`
	VideoCodec   *string            `json:"video_codec,omitempty"`
	VideoProfile *string            `json:"video_profile,omitempty"`
	Width        *int               `json:"width,omitempty"`
	Height       *int               `json:"height,omitempty"`
	PixFmt       *string            `json:"pix_fmt,omitempty"`
	ProbedAt     *time.Time         `json:"probed_at,omitempty"`
	ProbeError   *string            `json:"probe_error,omitempty"`
	Streams      []Stream           `json:"streams"`
	Subtitles    []ExternalSubtitle `json:"external_subtitles"`
}

// Stream je stopa v souboru (podle ffprobe).
type Stream struct {
	Index     int     `json:"index"`
	Kind      string  `json:"kind"` // video | audio | subtitle
	Codec     *string `json:"codec,omitempty"`
	Language  *string `json:"language,omitempty"`
	Title     *string `json:"title,omitempty"`
	Channels  *int    `json:"channels,omitempty"`
	IsDefault bool    `json:"is_default"`
	IsForced  bool    `json:"is_forced"`
}

// ExternalSubtitle je soubor titulků vedle videa.
type ExternalSubtitle struct {
	ID       uuid.UUID `json:"id"`
	FilePath string    `json:"-"`
	Language *string   `json:"language,omitempty"`
	Format   string    `json:"format"`
	IsForced bool      `json:"is_forced"`
}

// WatchProgress je rozkoukanost filmu nebo dílu uživatelem.
type WatchProgress struct {
	MovieID     *uuid.UUID `json:"movie_id,omitempty"`
	EpisodeID   *uuid.UUID `json:"episode_id,omitempty"`
	MediaID     *uuid.UUID `json:"media_id,omitempty"`
	PositionSec float64    `json:"position_sec"`
	DurationSec *float64   `json:"duration_sec,omitempty"`
	Completed   bool       `json:"completed"`
	RecordedAt  time.Time  `json:"recorded_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ContinueItem je položka „Pokračovat ve sledování“: rozkoukaný film,
// rozkoukaný díl, nebo další díl po dokoukaném.
type ContinueItem struct {
	Kind     string         `json:"kind"` // movie | episode
	Movie    *Movie         `json:"movie,omitempty"`
	Episode  *Episode       `json:"episode,omitempty"`
	Show     *Show          `json:"show,omitempty"`
	Progress *WatchProgress `json:"progress,omitempty"`
	// Next je true u dalšího dílu, který uživatel ještě nezačal.
	Next bool `json:"next"`
}

// UserState je vztah uživatele k titulu (detail filmu nebo seriálu).
type UserState struct {
	InWatchlist bool           `json:"in_watchlist"`
	Rating      *int           `json:"rating,omitempty"`
	Progress    *WatchProgress `json:"progress,omitempty"`
}

// MarshalCredits a UnmarshalCredits převádí obsazení do sloupce credits_json.
func MarshalCredits(c []Credit) *string {
	if len(c) == 0 {
		return nil
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func UnmarshalCredits(s *string) []Credit {
	out := []Credit{}
	if s == nil || *s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(*s), &out)
	return out
}

// MarshalExternalIDs a UnmarshalExternalIDs převádí ID u jiných databází do
// sloupce external_ids_json.
func MarshalExternalIDs(ids map[string]string) *string {
	if len(ids) == 0 {
		return nil
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func UnmarshalExternalIDs(s *string) map[string]string {
	out := map[string]string{}
	if s == nil || *s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(*s), &out)
	return out
}
