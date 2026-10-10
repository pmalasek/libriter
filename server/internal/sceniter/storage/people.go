package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
)

// Osoby (sc_people) a jejich účast na titulech (sc_credits).
//
// Osoba ze zdroje metadat se pozná podle (provider, provider_id). Osoby
// známé jen jménem (přenos ze starého credits_json, zdroj bez ID) se při
// prvním výskytu stejného jména s ID „povýší“. Stránka osoby a fotky se
// navíc slučují podle jména, takže herec spárovaný jednou přes ČSFD
// a jednou přes TMDB má jednu filmografii.

// showHasFiles omezí seriály na ty, které mají aspoň jeden díl se souborem.
const showHasFiles = ` EXISTS (SELECT 1 FROM sc_seasons se JOIN sc_episodes e ON e.season_id = se.id
	JOIN sc_media_files mf ON mf.episode_id = e.id WHERE se.show_id = s.id)`

// samePerson vybere ID všech osob se stejným jménem jako osoba ?1.
const samePerson = `(SELECT id FROM sc_people WHERE name = (SELECT name FROM sc_people WHERE id = ?1))`

// personPhoto vrátí fotku osoby p, nebo fotku jiné osoby se stejným jménem.
const personPhoto = `COALESCE(p.profile_path,
	(SELECT p2.profile_path FROM sc_people p2 WHERE p2.name = p.name AND p2.profile_path IS NOT NULL LIMIT 1))`

// setCredits nahradí obsazení titulu. Prázdný seznam nic nemění (zdroj
// obsazení nevrátil – ponechá se původní).
func setCredits(ctx context.Context, tx *sql.Tx, kind string, id uuid.UUID, provider string, credits []model.Credit) error {
	if len(credits) == 0 {
		return nil
	}
	col := "movie_id"
	if kind == model.KindShow {
		col = "show_id"
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sc_credits WHERE `+col+` = ?1`, id); err != nil {
		return fmt.Errorf("credits: %w", err)
	}
	for i, c := range credits {
		c.Name = strings.TrimSpace(c.Name)
		if c.Name == "" {
			continue
		}
		pid, err := resolvePerson(ctx, tx, provider, c)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sc_credits (`+col+`, person_id, role, character, position)
			VALUES (?1, ?2, ?3, ?4, ?5)`, id, pid, c.Role, nullStr(c.Character), i); err != nil {
			return fmt.Errorf("credit: %w", err)
		}
	}
	return nil
}

// resolvePerson najde nebo založí osobu a vrátí její ID.
func resolvePerson(ctx context.Context, tx *sql.Tx, provider string, c model.Credit) (uuid.UUID, error) {
	var id uuid.UUID
	if c.ProviderID != "" && provider != "" {
		err := tx.QueryRowContext(ctx, `SELECT id FROM sc_people WHERE provider = ?1 AND provider_id = ?2`,
			provider, c.ProviderID).Scan(&id)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE sc_people SET name = ?2,
				updated_at = CASE WHEN ?3 IS NOT NULL AND ?3 IS NOT profile_path THEN CURRENT_TIMESTAMP ELSE updated_at END,
				profile_path = COALESCE(?3, profile_path)
				WHERE id = ?1`, id, c.Name, nullStr(c.ImagePath))
			return id, err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("person: %w", err)
		}
		// Osoba známá zatím jen jménem dostane ID zdroje.
		err = tx.QueryRowContext(ctx, `SELECT id FROM sc_people WHERE name = ?1 AND provider_id IS NULL LIMIT 1`,
			c.Name).Scan(&id)
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE sc_people SET provider = ?2, provider_id = ?3,
				profile_path = COALESCE(?4, profile_path), updated_at = CURRENT_TIMESTAMP WHERE id = ?1`,
				id, provider, c.ProviderID, nullStr(c.ImagePath))
			return id, err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("person: %w", err)
		}
		id = uuid.New()
		_, err = tx.ExecContext(ctx, `INSERT INTO sc_people (id, name, provider, provider_id, profile_path)
			VALUES (?1, ?2, ?3, ?4, ?5)`, id, c.Name, provider, c.ProviderID, nullStr(c.ImagePath))
		if err != nil {
			return uuid.Nil, fmt.Errorf("person: %w", err)
		}
		return id, nil
	}

	// Bez ID u zdroje: podle jména, přednost má osoba s ID a fotkou.
	err := tx.QueryRowContext(ctx, `SELECT id FROM sc_people WHERE name = ?1
		ORDER BY provider_id IS NULL, profile_path IS NULL LIMIT 1`, c.Name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("person: %w", err)
	}
	id = uuid.New()
	if _, err := tx.ExecContext(ctx, `INSERT INTO sc_people (id, name, profile_path) VALUES (?1, ?2, ?3)`,
		id, c.Name, nullStr(c.ImagePath)); err != nil {
		return uuid.Nil, fmt.Errorf("person: %w", err)
	}
	return id, nil
}

// creditsByTitle vrátí obsazení a štáb titulu v pořadí ze zdroje.
func (s *Store) creditsByTitle(ctx context.Context, kind string, id uuid.UUID) ([]model.Credit, error) {
	col := "movie_id"
	if kind == model.KindShow {
		col = "show_id"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, c.role, c.character, `+personPhoto+`, p.updated_at
		FROM sc_credits c JOIN sc_people p ON p.id = c.person_id
		WHERE c.`+col+` = ?1 ORDER BY c.position`, id)
	if err != nil {
		return nil, fmt.Errorf("credits: %w", err)
	}
	defer rows.Close()
	out := []model.Credit{}
	for rows.Next() {
		var (
			c                model.Credit
			character, photo sql.NullString
			updated          time.Time
		)
		if err := rows.Scan(&c.PersonID, &c.Name, &c.Role, &character, &photo, &updated); err != nil {
			return nil, fmt.Errorf("credits: %w", err)
		}
		c.Character, c.ProfilePath, c.UpdatedAt = character.String, ptrString(photo), &updated
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetPerson vrátí osobu; bez vlastní fotky převezme fotku osoby se stejným jménem.
func (s *Store) GetPerson(ctx context.Context, id uuid.UUID) (*model.Person, error) {
	var (
		p                    model.Person
		provider, pid, photo sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `SELECT p.id, p.name, p.provider, p.provider_id, `+personPhoto+`, p.updated_at
		FROM sc_people p WHERE p.id = ?1`, id).Scan(&p.ID, &p.Name, &provider, &pid, &photo, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get person: %w", err)
	}
	p.Provider, p.ProviderID, p.ProfilePath = ptrString(provider), ptrString(pid), ptrString(photo)
	return &p, nil
}

// PersonPhoto vrátí uloženou fotku osoby u zdroje (prázdné = osoba nebo
// fotka zatím není) – matcher podle ní nestahuje fotku znovu.
func (s *Store) PersonPhoto(ctx context.Context, provider, providerID string) (string, error) {
	var photo sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT profile_path FROM sc_people WHERE provider = ?1 AND provider_id = ?2`,
		provider, providerID).Scan(&photo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return photo.String, err
}

// PersonCredits vrátí filmografii osoby (i osob se stejným jménem) ze
// zapnutých knihoven, jen tituly se soubory. Více rolí v jednom titulu
// (režie i scénář) se spojí. Řazení: nejnovější první.
func (s *Store) PersonCredits(ctx context.Context, id uuid.UUID) ([]model.PersonCredit, error) {
	out := []model.PersonCredit{}
	byMovie := map[uuid.UUID]int{}
	rows, err := s.db.QueryContext(ctx, `SELECT `+movieCols+`, mf.id, mf.duration_sec, c.role, c.character
		FROM sc_credits c
		JOIN sc_movies m ON m.id = c.movie_id
		JOIN sc_media_files mf ON mf.id = `+mainMovieFile+`
		JOIN sc_libraries l ON l.id = m.library_id AND l.enabled = 1
		WHERE c.person_id IN `+samePerson+`
		ORDER BY m.year DESC NULLS LAST, m.title, c.position`, id)
	if err != nil {
		return nil, fmt.Errorf("person movies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			mediaID   *uuid.UUID
			duration  sql.NullFloat64
			role      string
			character sql.NullString
		)
		m, err := scanMovie(rows, &mediaID, &duration, &role, &character)
		if err != nil {
			return nil, fmt.Errorf("person movies: %w", err)
		}
		m.MediaID, m.DurationSec = mediaID, ptrFloat(duration)
		if i, ok := byMovie[m.ID]; ok {
			out[i].Roles = appendRole(out[i].Roles, role)
			if out[i].Character == "" {
				out[i].Character = character.String
			}
			continue
		}
		byMovie[m.ID] = len(out)
		out = append(out, model.PersonCredit{Roles: []string{role}, Character: character.String, Movie: &m})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	byShow := map[uuid.UUID]int{}
	rows, err = s.db.QueryContext(ctx, `SELECT `+showCols+`, `+showCounts+`, c.role, c.character
		FROM sc_credits c
		JOIN sc_shows s ON s.id = c.show_id
		JOIN sc_libraries l ON l.id = s.library_id AND l.enabled = 1
		WHERE c.person_id IN `+samePerson+` AND`+showHasFiles+`
		ORDER BY s.year DESC NULLS LAST, s.title, c.position`, id)
	if err != nil {
		return nil, fmt.Errorf("person shows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			role      string
			character sql.NullString
		)
		sh, err := scanShow(rows, &role, &character)
		if err != nil {
			return nil, fmt.Errorf("person shows: %w", err)
		}
		if i, ok := byShow[sh.ID]; ok {
			out[i].Roles = appendRole(out[i].Roles, role)
			if out[i].Character == "" {
				out[i].Character = character.String
			}
			continue
		}
		byShow[sh.ID] = len(out)
		out = append(out, model.PersonCredit{Roles: []string{role}, Character: character.String, Show: &sh})
	}
	return out, rows.Err()
}

func appendRole(roles []string, role string) []string {
	for _, r := range roles {
		if r == role {
			return roles
		}
	}
	return append(roles, role)
}

// SearchPeople najde osoby podle části jména. Vrací jen osoby, které mají
// titul v zapnuté knihovně; stejná jména se slučují (přednost s fotkou).
func (s *Store) SearchPeople(ctx context.Context, q string, limit int) ([]model.Person, error) {
	out := []model.Person{}
	if strings.TrimSpace(q) == "" {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, `+personPhoto+`, p.updated_at FROM sc_people p
		WHERE p.name LIKE ?1 ESCAPE '\'
		  AND EXISTS (SELECT 1 FROM sc_credits c
		    LEFT JOIN sc_movies m ON m.id = c.movie_id
		    LEFT JOIN sc_shows s ON s.id = c.show_id
		    JOIN sc_libraries l ON l.id = COALESCE(m.library_id, s.library_id) AND l.enabled = 1
		    WHERE c.person_id = p.id)
		ORDER BY p.name, p.profile_path IS NULL, p.provider_id IS NULL`, likePattern(q))
	if err != nil {
		return nil, fmt.Errorf("search people: %w", err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var (
			p     model.Person
			photo sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.Name, &photo, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("search people: %w", err)
		}
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		p.ProfilePath = ptrString(photo)
		out = append(out, p)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// SimilarMovies vrátí filmy ze stejné knihovny se společnými žánry:
// nejvíc společných žánrů první, pak podle hodnocení.
func (s *Store) SimilarMovies(ctx context.Context, id uuid.UUID, limit int) ([]model.Movie, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+movieCols+`, mf.id, mf.duration_sec FROM sc_movies m
		JOIN sc_media_files mf ON mf.id = `+mainMovieFile+`
		JOIN (SELECT mg.movie_id, count(*) AS shared FROM sc_movie_genres mg
		      WHERE mg.genre_id IN (SELECT genre_id FROM sc_movie_genres WHERE movie_id = ?1)
		      GROUP BY mg.movie_id) g ON g.movie_id = m.id
		WHERE m.id <> ?1 AND m.library_id = (SELECT library_id FROM sc_movies WHERE id = ?1)
		ORDER BY g.shared DESC, m.rating DESC NULLS LAST, m.title
		LIMIT ?2`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("similar movies: %w", err)
	}
	defer rows.Close()
	out := []model.Movie{}
	for rows.Next() {
		var (
			mediaID  *uuid.UUID
			duration sql.NullFloat64
		)
		m, err := scanMovie(rows, &mediaID, &duration)
		if err != nil {
			return nil, fmt.Errorf("similar movies: %w", err)
		}
		m.MediaID, m.DurationSec = mediaID, ptrFloat(duration)
		out = append(out, m)
	}
	return out, rows.Err()
}

// SimilarShows je obdoba SimilarMovies pro seriály.
func (s *Store) SimilarShows(ctx context.Context, id uuid.UUID, limit int) ([]model.Show, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+showCols+`, `+showCounts+` FROM sc_shows s
		JOIN (SELECT sg.show_id, count(*) AS shared FROM sc_show_genres sg
		      WHERE sg.genre_id IN (SELECT genre_id FROM sc_show_genres WHERE show_id = ?1)
		      GROUP BY sg.show_id) g ON g.show_id = s.id
		WHERE s.id <> ?1 AND s.library_id = (SELECT library_id FROM sc_shows WHERE id = ?1) AND`+showHasFiles+`
		ORDER BY g.shared DESC, s.rating DESC NULLS LAST, s.title
		LIMIT ?2`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("similar shows: %w", err)
	}
	defer rows.Close()
	out := []model.Show{}
	for rows.Next() {
		sh, err := scanShow(rows)
		if err != nil {
			return nil, fmt.Errorf("similar shows: %w", err)
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// BackfillCredits jednorázově přenese obsazení ze starého sloupce
// credits_json do sc_people a sc_credits (osoby jen podle jména). Po
// přenosu je sloupec NULL, další volání nic nedělá.
func (s *Store) BackfillCredits(ctx context.Context) (int, error) {
	total := 0
	for _, kind := range []string{model.KindMovie, model.KindShow} {
		table := "sc_movies"
		if kind == model.KindShow {
			table = "sc_shows"
		}
		type pending struct {
			id      uuid.UUID
			credits []model.Credit
		}
		var todo []pending
		rows, err := s.db.QueryContext(ctx, `SELECT id, credits_json FROM `+table+` WHERE credits_json IS NOT NULL`)
		if err != nil {
			return total, fmt.Errorf("backfill credits: %w", err)
		}
		for rows.Next() {
			var (
				id  uuid.UUID
				raw string
			)
			if err := rows.Scan(&id, &raw); err != nil {
				rows.Close()
				return total, fmt.Errorf("backfill credits: %w", err)
			}
			todo = append(todo, pending{id, model.UnmarshalCredits(&raw)})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return total, err
		}
		if len(todo) == 0 {
			continue
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return total, err
		}
		for _, t := range todo {
			// Titul, který už obsazení v sc_credits má (spárovaný po migraci), se nepřepisuje.
			var has int
			col := "movie_id"
			if kind == model.KindShow {
				col = "show_id"
			}
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sc_credits WHERE `+col+` = ?1`, t.id).Scan(&has); err != nil {
				tx.Rollback() //nolint:errcheck
				return total, err
			}
			if has == 0 {
				if err := setCredits(ctx, tx, kind, t.id, "", t.credits); err != nil {
					tx.Rollback() //nolint:errcheck
					return total, err
				}
			}
			if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET credits_json = NULL WHERE id = ?1`, t.id); err != nil {
				tx.Rollback() //nolint:errcheck
				return total, err
			}
		}
		if err := tx.Commit(); err != nil {
			return total, err
		}
		total += len(todo)
	}
	return total, nil
}
