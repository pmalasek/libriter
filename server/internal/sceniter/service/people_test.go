package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"mediater/internal/core/config"
	"mediater/internal/core/db"
	"mediater/internal/sceniter/migrations"
	"mediater/internal/sceniter/model"
	"mediater/internal/sceniter/storage"
)

func (e *env) applyMovie(t *testing.T, id uuid.UUID, provider, metaID string, genres []string, credits ...model.Credit) {
	t.Helper()
	_, err := e.store.ApplyMovieMeta(context.Background(), id, storage.TitleMeta{
		Provider: provider, ID: metaID, Genres: genres, Credits: credits,
	}, model.MatchManual)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreditsAndFilmography(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	matrix, _ := e.movie(t, "Matrix")
	john, _ := e.movie(t, "John Wick")
	pelisky, _ := e.movie(t, "Pelíšky")

	e.applyMovie(t, matrix, "tmdb", "603", []string{"Akční", "Sci-Fi"},
		model.Credit{Name: "Lana Wachowski", Role: model.RoleDirector, ProviderID: "9340"},
		model.Credit{Name: "Lana Wachowski", Role: model.RoleWriter, ProviderID: "9340"},
		model.Credit{Name: "Keanu Reeves", Role: model.RoleCast, Character: "Neo", ProviderID: "6384", ImagePath: "person-tmdb-6384.jpg"},
	)
	e.applyMovie(t, john, "tmdb", "245891", []string{"Akční"},
		model.Credit{Name: "Keanu Reeves", Role: model.RoleCast, Character: "John Wick", ProviderID: "6384"},
	)
	// Stejný herec ze zdroje bez fotky (ČSFD) – jiná osoba, stejné jméno.
	e.applyMovie(t, pelisky, "csfd", "8365", []string{"Komedie"},
		model.Credit{Name: "Keanu Reeves", Role: model.RoleCast, ProviderID: "77"},
	)

	m, err := e.store.GetMovie(ctx, matrix)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Credits) != 3 || m.Credits[0].Role != model.RoleDirector || m.Credits[2].Character != "Neo" ||
		m.Credits[2].ProfilePath == nil || *m.Credits[2].ProfilePath != "person-tmdb-6384.jpg" {
		t.Fatalf("obsazení %+v", m.Credits)
	}
	keanu := m.Credits[2].PersonID

	// Fotka se při dalším párování bez ImagePath neztratí a osoba je táž.
	j, _ := e.store.GetMovie(ctx, john)
	if len(j.Credits) != 1 || j.Credits[0].PersonID != keanu || j.Credits[0].ProfilePath == nil {
		t.Fatalf("John Wick %+v", j.Credits)
	}
	// Osoba z ČSFD bez fotky převezme fotku osoby se stejným jménem.
	p, _ := e.store.GetMovie(ctx, pelisky)
	if p.Credits[0].PersonID == keanu || p.Credits[0].ProfilePath == nil {
		t.Fatalf("Pelíšky %+v", p.Credits)
	}

	// Filmografie spojí obě osoby se stejným jménem.
	fc, err := e.store.PersonCredits(ctx, keanu)
	if err != nil {
		t.Fatal(err)
	}
	if len(fc) != 3 {
		t.Fatalf("filmografie %+v", fc)
	}
	// Režie i scénář v jednom titulu jsou jedna položka se dvěma rolemi.
	fc, _ = e.store.PersonCredits(ctx, m.Credits[0].PersonID)
	if len(fc) != 1 || len(fc[0].Roles) != 2 || fc[0].Movie == nil || fc[0].Movie.ID != matrix {
		t.Fatalf("Wachowski %+v", fc)
	}

	people, err := e.store.SearchPeople(ctx, "keanu", 10)
	if err != nil || len(people) != 1 || people[0].ProfilePath == nil {
		t.Fatalf("hledání osob %+v %v", people, err)
	}

	// Podobné: společný žánr Akční.
	sim, err := e.store.SimilarMovies(ctx, matrix, 12)
	if err != nil || len(sim) != 1 || sim[0].ID != john {
		t.Fatalf("podobné %+v %v", sim, err)
	}

	// Výpis obsazení nenese, detail ano.
	list, _ := e.store.ListMovies(ctx, storage.MovieFilter{})
	for _, mv := range list {
		if len(mv.Credits) != 0 {
			t.Fatalf("výpis s obsazením %+v", mv)
		}
	}

	// Zdroj bez obsazení původní obsazení nesmaže.
	e.applyMovie(t, john, "tmdb", "245891", nil)
	if j, _ := e.store.GetMovie(ctx, john); len(j.Credits) != 1 {
		t.Fatalf("obsazení po párování bez credits %+v", j.Credits)
	}
}

func TestPersonUpgradedFromName(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a, _ := e.movie(t, "A")
	b, _ := e.movie(t, "B")
	// Zdroj bez ID: osoba jen podle jména.
	e.applyMovie(t, a, "ukazka", "1", nil, model.Credit{Name: "Jan Hřebejk", Role: model.RoleDirector})
	before, _ := e.store.GetMovie(ctx, a)
	// Stejné jméno s ID zdroje osobu „povýší“, nevznikne druhá.
	e.applyMovie(t, b, "csfd", "2", nil, model.Credit{Name: "Jan Hřebejk", Role: model.RoleDirector, ProviderID: "3195"})
	after, _ := e.store.GetMovie(ctx, b)
	if before.Credits[0].PersonID != after.Credits[0].PersonID {
		t.Fatalf("osoba se nesloučila: %v × %v", before.Credits[0].PersonID, after.Credits[0].PersonID)
	}
}

func TestBackfillCredits(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, config.DBConfig{Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := db.MigrateModule(ctx, conn, "sceniter", migrations.FS); err != nil {
		t.Fatal(err)
	}
	store := storage.New(conn)
	lib, err := store.CreateLibrary(ctx, storage.LibraryInput{Dir: "Filmy", Name: "Filmy", Kind: "movie", Language: "cs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.EnsureMovie(ctx, storage.MovieKey{LibraryID: lib.ID, SourceKey: "Filmy/X", Title: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE sc_movies SET credits_json = ?2 WHERE id = ?1`, id,
		`[{"name":"Jan Hřebejk","role":"director"},{"name":"Miroslav Donutil","role":"cast","character":"Šebek"}]`); err != nil {
		t.Fatal(err)
	}
	n, err := store.BackfillCredits(ctx)
	if err != nil || n != 1 {
		t.Fatalf("backfill %d %v", n, err)
	}
	m, _ := store.GetMovie(ctx, id)
	if len(m.Credits) != 2 || m.Credits[1].Character != "Šebek" {
		t.Fatalf("přenesené obsazení %+v", m.Credits)
	}
	// Podruhé už nic.
	if n, err := store.BackfillCredits(ctx); err != nil || n != 0 {
		t.Fatalf("druhý backfill %d %v", n, err)
	}
}
