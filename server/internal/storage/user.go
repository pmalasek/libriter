package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"libriter/internal/model"

	"github.com/google/uuid"
)

// CreateUser vytvoří uživatele a přiřadí mu roli v jedné transakci.
// Prázdný login se uloží jako NULL.
func (s *Store) CreateUser(ctx context.Context, displayName, email, login, passwordHash, roleName string) (*model.User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	const qUser = `
		INSERT INTO users (id, display_name, email, login, password_hash)
		VALUES (?1, ?2, ?3, NULLIF(?4, ''), ?5)
		RETURNING id, display_name, email, login, password_hash, created_at, updated_at, color_scheme, theme_mode, ui_language`

	row := tx.QueryRowContext(ctx, qUser, uuid.New(), displayName, email, login, passwordHash)
	u, err := scanUser(row)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, userConflict(err)
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	const qRole = `
		INSERT INTO user_roles (user_id, role_id)
		SELECT ?1, id FROM roles WHERE name = ?2`

	if _, err = tx.ExecContext(ctx, qRole, u.ID, roleName); err != nil {
		return nil, fmt.Errorf("assign role: %w", err)
	}

	u.Role = roleName
	return u, tx.Commit()
}

// GetUserByIdentifier najde uživatele pro přihlášení: identifikátor se
// porovná s loginem i s e-mailem (bez ohledu na velikost písmen). Login nesmí
// obsahovat „@“, takže se nemůže shodovat s cizím e-mailem.
func (s *Store) GetUserByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	const q = `
		SELECT u.id, u.display_name, u.email, u.login, u.password_hash,
		       COALESCE(r.name, 'reader') AS role,
		       u.created_at, u.updated_at, u.color_scheme, u.theme_mode, u.ui_language
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r       ON r.id = ur.role_id
		WHERE lower(u.login) = lower(?1) OR lower(u.email) = lower(?1)
		LIMIT 1`

	row := s.db.QueryRowContext(ctx, q, identifier)
	u, err := scanUserWithRole(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByEmail vrátí uživatele včetně jeho role.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	const q = `
		SELECT u.id, u.display_name, u.email, u.login, u.password_hash,
		       COALESCE(r.name, 'reader') AS role,
		       u.created_at, u.updated_at, u.color_scheme, u.theme_mode, u.ui_language
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r       ON r.id = ur.role_id
		WHERE lower(u.email) = lower(?1)
		LIMIT 1`

	row := s.db.QueryRowContext(ctx, q, email)
	u, err := scanUserWithRole(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByID vrátí uživatele včetně jeho role.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	const q = `
		SELECT u.id, u.display_name, u.email, u.login, u.password_hash,
		       COALESCE(r.name, 'reader') AS role,
		       u.created_at, u.updated_at, u.color_scheme, u.theme_mode, u.ui_language
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r       ON r.id = ur.role_id
		WHERE u.id = ?1
		LIMIT 1`

	row := s.db.QueryRowContext(ctx, q, id)
	u, err := scanUserWithRole(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// ListUsers vrátí všechny uživatele s jejich rolí.
func (s *Store) ListUsers(ctx context.Context) ([]model.User, error) {
	const q = `
		SELECT u.id, u.display_name, u.email, u.login, u.password_hash,
		       COALESCE(r.name, 'reader') AS role,
		       u.created_at, u.updated_at, u.color_scheme, u.theme_mode, u.ui_language
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r       ON r.id = ur.role_id
		ORDER BY u.display_name`

	rows, err := s.db.QueryContext(ctx, q)
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

// UpdateUser aktualizuje display_name, email a login uživatele. Prázdný login
// se uloží jako NULL.
func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, displayName, email, login string) (*model.User, error) {
	const q = `
		UPDATE users
		SET display_name = ?2, email = ?3, login = NULLIF(?4, ''), updated_at = CURRENT_TIMESTAMP
		WHERE id = ?1
		RETURNING id, display_name, email, login, password_hash, created_at, updated_at, color_scheme, theme_mode, ui_language`

	row := s.db.QueryRowContext(ctx, q, id, displayName, email, login)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, userConflict(err)
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

// UpdateUserAppearance mění pouze vzhled, osobní údaje ponechá nedotčené.
func (s *Store) UpdateUserAppearance(ctx context.Context, id uuid.UUID, colorScheme, themeMode string) (*model.User, error) {
	const q = `UPDATE users SET color_scheme = ?2, theme_mode = ?3,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?1`
	res, err := s.db.ExecContext(ctx, q, id, colorScheme, themeMode)
	if err != nil {
		return nil, fmt.Errorf("update user appearance: %w", err)
	}
	if rowsAffected(res) == 0 {
		return nil, ErrNotFound
	}
	return s.GetUserByID(ctx, id)
}

// UpdateUserLanguage mění pouze jazyk rozhraní.
func (s *Store) UpdateUserLanguage(ctx context.Context, id uuid.UUID, uiLanguage string) (*model.User, error) {
	const q = `UPDATE users SET ui_language = ?2,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?1`
	res, err := s.db.ExecContext(ctx, q, id, uiLanguage)
	if err != nil {
		return nil, fmt.Errorf("update user language: %w", err)
	}
	if rowsAffected(res) == 0 {
		return nil, ErrNotFound
	}
	return s.GetUserByID(ctx, id)
}

// UpdateUserPassword nastaví nový hash hesla.
func (s *Store) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	const q = `UPDATE users SET password_hash = ?2, updated_at = CURRENT_TIMESTAMP WHERE id = ?1`
	res, err := s.db.ExecContext(ctx, q, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser smaže uživatele (kaskáda odstraní role, pozice atd.)
// Posledního administrátora smazat nelze – vrátí ErrLastAdmin.
func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete user: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if err := checkNotLastAdmin(ctx, tx, id); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// CountUsersByRole vrátí počet uživatelů s danou rolí.
func (s *Store) CountUsersByRole(ctx context.Context, roleName string) (int, error) {
	n, err := countUsersByRole(ctx, s.db, roleName)
	if err != nil {
		return 0, fmt.Errorf("count users by role: %w", err)
	}
	return n, nil
}

func countUsersByRole(ctx context.Context, q querier, roleName string) (int, error) {
	const sel = `
		SELECT COUNT(*)
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE r.name = ?1`

	var n int
	err := q.QueryRowContext(ctx, sel, roleName).Scan(&n)
	return n, err
}

// checkNotLastAdmin ověří, že uživatel není jediný administrátor. Kontrola
// běží uvnitř transakce volajícího, takže dva souběžné pokusy nemohou projít
// oba naráz.
func checkNotLastAdmin(ctx context.Context, q querier, userID uuid.UUID) error {
	const sel = `
		SELECT COUNT(*)
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE r.name = ?1 AND ur.user_id = ?2`

	var isAdmin int
	if err := q.QueryRowContext(ctx, sel, model.RoleAdmin, userID).Scan(&isAdmin); err != nil {
		return fmt.Errorf("check last admin: %w", err)
	}
	if isAdmin == 0 {
		return nil
	}

	admins, err := countUsersByRole(ctx, q, model.RoleAdmin)
	if err != nil {
		return fmt.Errorf("check last admin: %w", err)
	}
	if admins <= 1 {
		return ErrLastAdmin
	}
	return nil
}

// GetUserRole vrátí název role uživatele.
func (s *Store) GetUserRole(ctx context.Context, userID uuid.UUID) (string, error) {
	const q = `
		SELECT r.name
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = ?1
		ORDER BY r.id
		LIMIT 1`

	var role string
	err := s.db.QueryRowContext(ctx, q, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RoleReader, nil
	}
	return role, err
}

// SetUserRole nahradí roli uživatele novou rolí. Poslednímu administrátorovi
// roli odebrat nelze – vrátí ErrLastAdmin.
func (s *Store) SetUserRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	// Validace, že role existuje
	if _, ok := model.RoleLevel[strings.ToLower(roleName)]; !ok {
		return fmt.Errorf("neznámá role: %s", roleName)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if roleName != model.RoleAdmin {
		if err := checkNotLastAdmin(ctx, tx, userID); err != nil {
			return err
		}
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = ?1`, userID); err != nil {
		return fmt.Errorf("clear roles: %w", err)
	}

	const q = `
		INSERT INTO user_roles (user_id, role_id)
		SELECT ?1, id FROM roles WHERE name = ?2`
	if _, err = tx.ExecContext(ctx, q, userID, roleName); err != nil {
		return fmt.Errorf("set role: %w", err)
	}

	return tx.Commit()
}

// --- helpers ---

type scanner interface {
	Scan(dest ...any) error
}

// userConflict rozliší, který unikátní index uživatel porušil: obsazený login
// vrací ErrLoginConflict, cokoliv jiného (e-mail) ErrConflict.
func userConflict(err error) error {
	if strings.Contains(err.Error(), "users_login_unique") {
		return ErrLoginConflict
	}
	return ErrConflict
}

func scanUser(row scanner) (*model.User, error) {
	var u model.User
	var login sql.NullString
	err := row.Scan(&u.ID, &u.DisplayName, &u.Email, &login, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt, &u.ColorScheme, &u.ThemeMode, &u.UILanguage)
	if err != nil {
		return nil, err
	}
	u.Login = login.String
	return &u, nil
}

func scanUserWithRole(row scanner) (*model.User, error) {
	var u model.User
	var login sql.NullString
	err := row.Scan(&u.ID, &u.DisplayName, &u.Email, &login, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.ColorScheme, &u.ThemeMode, &u.UILanguage)
	if err != nil {
		return nil, err
	}
	u.Login = login.String
	return &u, nil
}
