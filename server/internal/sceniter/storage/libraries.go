package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"mediater/internal/sceniter/model"
)

const libraryCols = `
	l.id, l.dir, l.name, l.kind, l.language, l.provider, l.icon, l.position, l.enabled, l.created_at, l.updated_at,
	CASE l.kind
	  WHEN 'movie' THEN (SELECT count(*) FROM sc_movies m WHERE m.library_id = l.id
	                       AND EXISTS (SELECT 1 FROM sc_media_files f WHERE f.movie_id = m.id))
	  ELSE (SELECT count(*) FROM sc_shows s WHERE s.library_id = l.id
	          AND EXISTS (SELECT 1 FROM sc_seasons se JOIN sc_episodes e ON e.season_id = se.id
	                      JOIN sc_media_files f ON f.episode_id = e.id WHERE se.show_id = s.id))
	END`

func scanLibrary(sc interface{ Scan(...any) error }) (model.Library, error) {
	var (
		l              model.Library
		provider, icon sql.NullString
	)
	err := sc.Scan(&l.ID, &l.Dir, &l.Name, &l.Kind, &l.Language, &provider, &icon, &l.Position, &l.Enabled,
		&l.CreatedAt, &l.UpdatedAt, &l.ItemCount)
	l.Provider, l.Icon = ptrString(provider), ptrString(icon)
	return l, err
}

// ListLibraries vrátí knihovny v nastaveném pořadí; onlyEnabled vynechá vypnuté.
func (s *Store) ListLibraries(ctx context.Context, onlyEnabled bool) ([]model.Library, error) {
	q := `SELECT ` + libraryCols + ` FROM sc_libraries l`
	if onlyEnabled {
		q += ` WHERE l.enabled = 1`
	}
	q += ` ORDER BY l.position, l.name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	defer rows.Close()
	out := []model.Library{}
	for rows.Next() {
		l, err := scanLibrary(rows)
		if err != nil {
			return nil, fmt.Errorf("list libraries: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetLibrary vrátí knihovnu podle ID.
func (s *Store) GetLibrary(ctx context.Context, id uuid.UUID) (*model.Library, error) {
	l, err := scanLibrary(s.db.QueryRowContext(ctx, `SELECT `+libraryCols+` FROM sc_libraries l WHERE l.id = ?1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get library: %w", err)
	}
	return &l, nil
}

// LibraryInput jsou upravitelné vlastnosti knihovny.
type LibraryInput struct {
	Dir      string
	Name     string
	Kind     string
	Language string
	Provider string
	Icon     string
	Enabled  bool
}

// CreateLibrary založí knihovnu na konci pořadí. Obsazená složka je ErrConflict.
func (s *Store) CreateLibrary(ctx context.Context, in LibraryInput) (*model.Library, error) {
	id := uuid.New()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sc_libraries (id, dir, name, kind, language, provider, icon, enabled, position)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, (SELECT COALESCE(max(position), -1) + 1 FROM sc_libraries))`,
		id, in.Dir, in.Name, in.Kind, in.Language, nullStr(in.Provider), nullStr(in.Icon), in.Enabled)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("create library: %w", err)
	}
	return s.GetLibrary(ctx, id)
}

// UpdateLibrary uloží název, jazyk, zdroj, ikonu a zapnutí. Složku ani typ změnit
// nejde – obsah by neodpovídal tomu, co už je naskenované.
func (s *Store) UpdateLibrary(ctx context.Context, id uuid.UUID, in LibraryInput) (*model.Library, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE sc_libraries SET name = ?2, language = ?3, provider = ?4, enabled = ?5, icon = ?6,
		  updated_at = CURRENT_TIMESTAMP
		WHERE id = ?1`, id, in.Name, in.Language, nullStr(in.Provider), in.Enabled, nullStr(in.Icon))
	if err != nil {
		return nil, fmt.Errorf("update library: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return s.GetLibrary(ctx, id)
}

// DeleteLibrary smaže knihovnu i všechny její tituly a soubory (kaskádou).
func (s *Store) DeleteLibrary(ctx context.Context, id uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sc_libraries WHERE id = ?1`, id)
	if err != nil {
		return fmt.Errorf("delete library: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderLibraries nastaví pořadí podle seznamu ID. Seznam musí obsahovat
// každou knihovnu právě jednou.
func (s *Store) ReorderLibraries(ctx context.Context, ids []uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sc_libraries`).Scan(&count); err != nil {
		return err
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if len(ids) != count || len(seen) != count {
		return fmt.Errorf("%w: pořadí musí obsahovat každou knihovnu právě jednou", ErrInvalid)
	}
	for i, id := range ids {
		res, err := tx.ExecContext(ctx, `UPDATE sc_libraries SET position = ?2 WHERE id = ?1`, id, i)
		if err != nil {
			return fmt.Errorf("reorder libraries: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("%w: neznámá knihovna %s", ErrInvalid, id)
		}
	}
	return tx.Commit()
}
