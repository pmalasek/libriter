// Package model obsahuje typy identity společné všem produktům: uživatele,
// role a záznamy auditu. Doménové typy si drží každý produkt sám.
package model

import (
	"encoding/json"
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

// UILanguages jsou podporované jazyky rozhraní; první je výchozí.
var UILanguages = []string{"en", "cs", "fr", "de", "es"}

// DefaultUILanguage je jazyk rozhraní pro nové uživatele.
const DefaultUILanguage = "en"

type User struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	// Login je volitelné přihlašovací jméno; prázdné = uživatel se hlásí jen e-mailem.
	Login        string `json:"login"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	ColorScheme  string `json:"color_scheme"`
	ThemeMode    string `json:"theme_mode"`
	// UILanguage je jazyk uživatelského rozhraní (cs, en, fr, de, es).
	UILanguage string    `json:"ui_language"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AuditEntry je záznam administrativní akce. ActorEmail je snímek z doby
// zápisu – účet mohl mezitím zmizet a ActorID je pak NULL.
type AuditEntry struct {
	ID          int64           `json:"id"`
	ActorID     *uuid.UUID      `json:"actor_id,omitempty"`
	ActorEmail  string          `json:"actor_email"`
	Action      string          `json:"action"`
	TargetType  string          `json:"target_type"`
	TargetID    string          `json:"target_id"`
	TargetLabel string          `json:"target_label"`
	Details     json.RawMessage `json:"details"`
	CreatedAt   time.Time       `json:"created_at"`
}
