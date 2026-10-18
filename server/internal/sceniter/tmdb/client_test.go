package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Prázdný český popis dílu se doplní z angličtiny, existující zůstane.
func TestSeasonFallsBackToEnglishOverview(t *testing.T) {
	var langs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("language")
		langs = append(langs, lang)
		s := Season{Number: 1, Episodes: []Episode{
			{Number: 1, Name: "Pilot", Overview: "Český popis"},
			{Number: 2, Name: "Epizoda 2"},
		}}
		if lang == "en-US" {
			s.Episodes[0].Overview = "English one"
			s.Episodes[1].Overview = "English two"
		}
		_ = json.NewEncoder(w).Encode(s)
	}))
	defer srv.Close()

	c := New("key", "cs-CZ")
	c.baseURL = srv.URL
	s, err := c.Season(context.Background(), 42, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Episodes[0].Overview; got != "Český popis" {
		t.Errorf("díl 1: %q", got)
	}
	if got := s.Episodes[1].Overview; got != "English two" {
		t.Errorf("díl 2: %q", got)
	}
	if s.Episodes[1].Name != "Epizoda 2" {
		t.Errorf("název dílu se nemá měnit: %q", s.Episodes[1].Name)
	}
	if len(langs) != 2 || langs[1] != "en-US" {
		t.Errorf("dotazy: %v", langs)
	}
}

// Když mají všechny díly popis, anglický dotaz se neposílá.
func TestSeasonNoFallbackWhenComplete(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(Season{Number: 1, Episodes: []Episode{{Number: 1, Overview: "Popis"}}})
	}))
	defer srv.Close()

	c := New("key", "cs-CZ")
	c.baseURL = srv.URL
	if _, err := c.Season(context.Background(), 42, 1); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("dotazů %d, čekal jsem 1", calls)
	}
}
