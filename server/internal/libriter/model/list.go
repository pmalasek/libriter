package model

import (
	"time"

	"github.com/google/uuid"
)

// Štíhlé záznamy pro stránkované výpisy. Nesou jen to, co potřebuje
// dlaždice nebo řádek seznamu – popisy, životopisy a cesty k souborům se
// čtou až v detailu.

// AuthorRef je autor knihy ve výpisu: odkaz a jméno k zobrazení.
type AuthorRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// BookSummary je kniha ve výpisu.
type BookSummary struct {
	ID              uuid.UUID   `json:"id"`
	Title           string      `json:"title"`
	Authors         []AuthorRef `json:"authors"` // seřazeno podle book_authors.position
	SeriesID        *uuid.UUID  `json:"series_id,omitempty"`
	SeriesPosition  *int16      `json:"series_position,omitempty"`
	SeriesTitle     *string     `json:"series_title,omitempty"`
	DurationSeconds int         `json:"duration_seconds"`
	ChapterCount    int         `json:"chapter_count"`
	CoverPath       *string     `json:"cover_path,omitempty"`
	Language        string      `json:"language"`
	PublishedYear   *int        `json:"published_year,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

// AuthorSummary je autor ve výpisu i s počtem knih v knihovně.
type AuthorSummary struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"first_name"`
	MiddleName string    `json:"middle_name"`
	LastName   string    `json:"last_name"`
	Name       string    `json:"name"`
	ImagePath  *string   `json:"image_path,omitempty"`
	BirthYear  *int      `json:"birth_year,omitempty"`
	DeathYear  *int      `json:"death_year,omitempty"`
	BookCount  int       `json:"book_count"`
}

// SeriesCover je díl série, jehož obálka se ukazuje na kartě série.
type SeriesCover struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	SeriesPosition *int16    `json:"series_position,omitempty"`
	CoverPath      *string   `json:"cover_path,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SeriesSummary je série ve výpisu: počet dílů, autoři (nejčastější první)
// a první díly pro stoh obálek.
type SeriesSummary struct {
	ID        uuid.UUID     `json:"id"`
	Title     string        `json:"title"`
	BookCount int           `json:"book_count"`
	Authors   []AuthorRef   `json:"authors"`
	Covers    []SeriesCover `json:"covers"`
}
