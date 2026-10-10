package model

import (
	"time"

	"github.com/google/uuid"
)

// Štíhlé záznamy pro stránkované výpisy – jen to, co potřebuje dlaždice
// nebo řádek. Popisy, obsazení, žánry a ID u databází se čtou až v detailu.

// MovieSummary je film ve výpisu.
type MovieSummary struct {
	ID               uuid.UUID  `json:"id"`
	LibraryID        uuid.UUID  `json:"library_id"`
	Title            string     `json:"title"`
	OriginalTitle    *string    `json:"original_title,omitempty"`
	Year             *int       `json:"year,omitempty"`
	PosterPath       *string    `json:"poster_path,omitempty"`
	BackdropPath     *string    `json:"backdrop_path,omitempty"`
	Rating           *int       `json:"rating,omitempty"`
	MatchState       string     `json:"match_state"`
	MatchAttemptedAt *time.Time `json:"match_attempted_at,omitempty"`
	SourceKey        string     `json:"source_key"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DurationSec      *float64   `json:"duration_sec,omitempty"`
	MediaID          *uuid.UUID `json:"media_id,omitempty"`
}

// Summary zúží film na záznam výpisu.
func (m Movie) Summary() MovieSummary {
	return MovieSummary{
		ID: m.ID, LibraryID: m.LibraryID, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year,
		PosterPath: m.PosterPath, BackdropPath: m.BackdropPath, Rating: m.Rating,
		MatchState: m.MatchState, MatchAttemptedAt: m.MatchAttemptedAt, SourceKey: m.SourceKey,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, DurationSec: m.DurationSec, MediaID: m.MediaID,
	}
}

// ShowSummary je seriál ve výpisu.
type ShowSummary struct {
	ID               uuid.UUID  `json:"id"`
	LibraryID        uuid.UUID  `json:"library_id"`
	Title            string     `json:"title"`
	OriginalTitle    *string    `json:"original_title,omitempty"`
	Year             *int       `json:"year,omitempty"`
	PosterPath       *string    `json:"poster_path,omitempty"`
	BackdropPath     *string    `json:"backdrop_path,omitempty"`
	Rating           *int       `json:"rating,omitempty"`
	MatchState       string     `json:"match_state"`
	MatchAttemptedAt *time.Time `json:"match_attempted_at,omitempty"`
	SourceKey        string     `json:"source_key"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	SeasonCount      int        `json:"season_count"`
	EpisodeCount     int        `json:"episode_count"`
}

// Summary zúží seriál na záznam výpisu.
func (s Show) Summary() ShowSummary {
	return ShowSummary{
		ID: s.ID, LibraryID: s.LibraryID, Title: s.Title, OriginalTitle: s.OriginalTitle, Year: s.Year,
		PosterPath: s.PosterPath, BackdropPath: s.BackdropPath, Rating: s.Rating,
		MatchState: s.MatchState, MatchAttemptedAt: s.MatchAttemptedAt, SourceKey: s.SourceKey,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, SeasonCount: s.SeasonCount, EpisodeCount: s.EpisodeCount,
	}
}

// MovieSummaries zúží seznam filmů.
func MovieSummaries(list []Movie) []MovieSummary {
	out := make([]MovieSummary, len(list))
	for i := range list {
		out[i] = list[i].Summary()
	}
	return out
}

// ShowSummaries zúží seznam seriálů.
func ShowSummaries(list []Show) []ShowSummary {
	out := make([]ShowSummary, len(list))
	for i := range list {
		out[i] = list[i].Summary()
	}
	return out
}
