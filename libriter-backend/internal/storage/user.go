package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"libriter/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// CreateUser vytvoří uživatele a přiřadí mu roli v jedné transakci.
func (s *Store) CreateUser(ctx context.Context, displayName, email, passwordHash, roleName string) (*model.User, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const qUser = `
		INSERT INTO user_data.users (display_name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, display_name, email, password_hash, created_at, updated_at`

	row := tx.QueryRow(ctx, qUser, displayName, email, passwordHash)
	u, err := scanUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	const qRole = `
		INSERT INTO user_data.user_roles (user_id, role_id)
		SELECT $1, id FROM user_data.roles WHERE name = $2`

	if _, err = tx.Exec(ctx, qRole, u.ID, roleName); err != nil {
		return nil, fmt.Errorf("assign role: %w", err)
	}

	u.Role = roleName
	return u, tx.Commit(ctx)
}

// GetUserByEmail vrátí uživatele včetně jeho role.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	const q = `
		SELECT u.id, u.display_name, u.email, u.password_hash,
		       COALESCE(r.name, 'reader') AS role,
		       u.created_at, u.updated_at
		FROM user_data.users u
		LEFT JOIN user_data.user_roles ur ON ur.user_id = u.id
		LEFT JOIN user_data.roles r        ON r.id = ur.role_id
		WHERE lower(u.email) = lower($1)
		LIMIT 1`

	row := s.db.QueryRow(ctx, q, email)
	u, err := scanUserWithRole(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByID vrátí uživatele včetně jeho role.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	const q = `
		SELECT u.id, u.display_name, u.email, u.password_hash,
		       COALESCE(r.name, 'reader') AS role,
		       u.created_at, u.updated_at
		FROM user_data.users u
		LEFT JOIN user_data.user_roles ur ON ur.user_id = u.id
		LEFT JOIN user_data.roles r        ON r.id = ur.role_id
		WHERE u.id = $1
		LIMIT 1`

	row := s.db.QueryRow(ctx, q, id)
	u, err := scanUserWithRole(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// ListUsers vrátí všechny uživatele s jejich rolí.
func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	const q = `
		SELECT u.id, u.display_name, u.email, u.password_hash,
		       COALESCE(r.name, 'reader') AS role,
		       u.created_at, u.updated_at
		FROM user_data.users u
		LEFT JOIN user_data.user_roles ur ON ur.user_id = u.id
		LEFT JOIN user_data.roles r        ON r.id = ur.role_id
		ORDER BY u.display_name`

	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		u, err := scanUserWithRole(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

// UpdateUser aktualizuje display_name a email uživatele.
func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, displayName, email string) (*model.User, error) {
	const q = `
		UPDATE user_data.users
		SET display_name = $2, email = $3
		WHERE id = $1
		RETURNING id, display_name, email, password_hash, created_at, updated_at`

	row := s.db.QueryRow(ctx, q, id, displayName, email)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, err
	}
	// Načteme roli samostatně
	role, rerr := s.GetUserRole(ctx, u.ID)
	if rerr == nil {
		u.Role = role
	}
	return u, nil
}

// UpdateUserPassword nastaví nový hash hesla.
func (s *Store) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	const q = `UPDATE user_data.users SET password_hash = $2 WHERE id = $1`
	tag, err := s.db.Exec(ctx, q, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser smaže uživatele (kaskáda odstraní role, pozice atd.)
func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM user_data.users WHERE id = $1`
	tag, err := s.db.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetUserRole vrátí název role uživatele.
func (s *Store) GetUserRole(ctx context.Context, userID uuid.UUID) (string, error) {
	const q = `
		SELECT r.name
		FROM user_data.user_roles ur
		JOIN user_data.roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.id
		LIMIT 1`

	var role string
	err := s.db.QueryRow(ctx, q, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RoleReader, nil
	}
	return role, err
}

// SetUserRole nahradí roli uživatele novou rolí.
func (s *Store) SetUserRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	// Validace, že role existuje
	if _, ok := model.RoleLevel[strings.ToLower(roleName)]; !ok {
		return fmt.Errorf("neznámá role: %s", roleName)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx, `DELETE FROM user_data.user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear roles: %w", err)
	}

	const q = `
		INSERT INTO user_data.user_roles (user_id, role_id)
		SELECT $1, id FROM user_data.roles WHERE name = $2`
	if _, err = tx.Exec(ctx, q, userID, roleName); err != nil {
		return fmt.Errorf("set role: %w", err)
	}

	return tx.Commit(ctx)
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.DisplayName, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func scanUserWithRole(row scanner) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.DisplayName, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
