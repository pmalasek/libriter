package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestGenresByLibrary(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	matrix, _ := e.movie(t, "Matrix")
	pelisky, _ := e.movie(t, "Pelíšky")
	e.applyMovie(t, matrix, "tmdb", "603", []string{"Sci-Fi", "Akční"})
	e.applyMovie(t, pelisky, "csfd", "8365", []string{"Komedie"})

	for _, c := range []struct {
		lib  uuid.UUID
		want string
	}{
		{uuid.Nil, "Akční,Komedie,Sci-Fi"},
		{e.movieLib, "Akční,Komedie,Sci-Fi"},
		{e.showLib, ""},
	} {
		g, err := e.store.Genres(ctx, c.lib)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(g, ","); got != c.want {
			t.Errorf("knihovna %s: žánry %q, čekáno %q", c.lib, got, c.want)
		}
	}
}
