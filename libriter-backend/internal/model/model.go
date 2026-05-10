package model

import (
	"time"

	"github.com/google/uuid"
)

// Role names
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleReader = "reader"
)

// RoleLevel maps role name to its privilege level (nižší = více práv)
var RoleLevel = map[string]int{
	RoleAdmin:  1,
	RoleEditor: 2,
	RoleReader: 3,
}

type Role struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type Permission struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type Rating struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	BookID    uuid.UUID `json:"book_id"`
	Rating    int16     `json:"rating"`
	Review    *string   `json:"review,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Author struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Bio       *string   `json:"bio,omitempty"`
	ImagePath *string   `json:"image_path,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Series struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Book struct {
	ID              uuid.UUID  `json:"id"`
	AuthorID        uuid.UUID  `json:"author_id"`
	SeriesID        *uuid.UUID `json:"series_id,omitempty"`
	SeriesPosition  *int16     `json:"series_position,omitempty"`
	Title           string     `json:"title"`
	Narrator        *string    `json:"narrator,omitempty"`
	DurationSeconds int        `json:"duration_seconds"`
	FilePath        string     `json:"-"`
	CoverPath       *string    `json:"cover_path,omitempty"`
	Language        string     `json:"language"`
	Description     *string    `json:"description,omitempty"`
	InternalRating  *int16     `json:"internal_rating,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Chapter struct {
	ID                 uuid.UUID `json:"id"`
	BookID             uuid.UUID `json:"book_id"`
	Position           int       `json:"position"`
	Title              string    `json:"title"`
	FilePath           string    `json:"-"` // relativní cesta k audio souboru od AUDIO_ROOT
	StartOffsetSeconds int       `json:"start_offset_seconds"`
	DurationSeconds    int       `json:"duration_seconds"`
}

type User struct {
	ID           uuid.UUID `json:"id"`
	DisplayName  string    `json:"display_name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PlaybackPosition struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	BookID          uuid.UUID  `json:"book_id"`
	DeviceID        *uuid.UUID `json:"device_id,omitempty"`
	PositionSeconds int        `json:"position_seconds"`
	PlaybackSpeed   float64    `json:"playback_speed"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Bookmark struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	BookID          uuid.UUID  `json:"book_id"`
	ChapterID       *uuid.UUID `json:"chapter_id,omitempty"`
	PositionSeconds int        `json:"position_seconds"`
	Note            *string    `json:"note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}
