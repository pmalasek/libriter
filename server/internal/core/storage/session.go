package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mediater/internal/core/model"

	"github.com/google/uuid"
)

// Verze tokenů a relace zařízení. Sloupec token_version se čte a zapisuje
// jen tady – běžné dotazy na uživatele (scanUser) o něm nevědí.

// UserAuth je to, co ověření tokenu potřebuje vědět o uživateli.
type UserAuth struct {
	Role         string
	TokenVersion int
}

// GetUserAuth vrátí roli a verzi tokenů uživatele. Neexistující uživatel
// vrátí ErrNotFound.
func (s *Store) GetUserAuth(ctx context.Context, id uuid.UUID) (UserAuth, error) {
	const q = `
		SELECT COALESCE(r.name, 'reader'), u.token_version
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r       ON r.id = ur.role_id
		WHERE u.id = ?1
		LIMIT 1`
	var a UserAuth
	err := s.db.QueryRowContext(ctx, q, id).Scan(&a.Role, &a.TokenVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return UserAuth{}, ErrNotFound
	}
	if err != nil {
		return UserAuth{}, fmt.Errorf("get user auth: %w", err)
	}
	return a, nil
}

// LogoutAll zvýší verzi tokenů uživatele (všechny dosud vydané tokeny tím
// přestanou platit) a odvolá všechny jeho relace zařízení. Vrací novou verzi.
func (s *Store) LogoutAll(ctx context.Context, userID uuid.UUID, now time.Time) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("logout all: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	ver, err := bumpTokenVersion(ctx, tx, userID, now)
	if err != nil {
		return 0, err
	}
	return ver, tx.Commit()
}

// bumpTokenVersion zvýší verzi tokenů a odvolá relace zařízení uvnitř
// transakce volajícího.
func bumpTokenVersion(ctx context.Context, tx *sql.Tx, userID uuid.UUID, now time.Time) (int, error) {
	var ver int
	err := tx.QueryRowContext(ctx,
		`UPDATE users SET token_version = token_version + 1 WHERE id = ?1 RETURNING token_version`,
		userID).Scan(&ver)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("bump token version: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE device_sessions SET revoked_at = ?2 WHERE user_id = ?1 AND revoked_at IS NULL`,
		userID, now.Unix()); err != nil {
		return 0, fmt.Errorf("revoke device sessions: %w", err)
	}
	return ver, nil
}

// CreateDeviceSession založí relaci zařízení pro nově vydaný mobilní nebo
// televizní token.
func (s *Store) CreateDeviceSession(ctx context.Context, userID uuid.UUID, scope, deviceName string, now time.Time) (*model.DeviceSession, error) {
	d := &model.DeviceSession{
		ID:         uuid.New(),
		UserID:     userID,
		Scope:      scope,
		DeviceName: deviceName,
		CreatedAt:  time.Unix(now.Unix(), 0),
		LastSeenAt: time.Unix(now.Unix(), 0),
	}
	const q = `INSERT INTO device_sessions (id, user_id, scope, device_name, created_at, last_seen_at)
		VALUES (?1, ?2, ?3, ?4, ?5, ?5)`
	if _, err := s.db.ExecContext(ctx, q, d.ID, userID, scope, deviceName, now.Unix()); err != nil {
		return nil, fmt.Errorf("create device session: %w", err)
	}
	return d, nil
}

const deviceSessionColumns = `id, user_id, scope, device_name, created_at, last_seen_at, revoked_at`

func scanDeviceSession(row RowScanner) (*model.DeviceSession, error) {
	var (
		d                 model.DeviceSession
		created, lastSeen int64
		revoked           sql.NullInt64
	)
	if err := row.Scan(&d.ID, &d.UserID, &d.Scope, &d.DeviceName, &created, &lastSeen, &revoked); err != nil {
		return nil, err
	}
	d.CreatedAt = time.Unix(created, 0)
	d.LastSeenAt = time.Unix(lastSeen, 0)
	if revoked.Valid {
		t := time.Unix(revoked.Int64, 0)
		d.RevokedAt = &t
	}
	return &d, nil
}

// GetDeviceSession vrátí relaci zařízení (i odvolanou). Neznámé ID vrátí
// ErrNotFound.
func (s *Store) GetDeviceSession(ctx context.Context, id uuid.UUID) (*model.DeviceSession, error) {
	d, err := scanDeviceSession(s.db.QueryRowContext(ctx,
		`SELECT `+deviceSessionColumns+` FROM device_sessions WHERE id = ?1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device session: %w", err)
	}
	return d, nil
}

// ListDeviceSessions vrátí platné (neodvolané) relace uživatele, naposledy
// aktivní první.
func (s *Store) ListDeviceSessions(ctx context.Context, userID uuid.UUID) ([]model.DeviceSession, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+deviceSessionColumns+` FROM device_sessions
		WHERE user_id = ?1 AND revoked_at IS NULL
		ORDER BY last_seen_at DESC, created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list device sessions: %w", err)
	}
	defer rows.Close()

	out := []model.DeviceSession{}
	for rows.Next() {
		d, err := scanDeviceSession(rows)
		if err != nil {
			return nil, fmt.Errorf("list device sessions: %w", err)
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// RevokeDeviceSession odvolá relaci zařízení uživatele a vrátí ji. Cizí,
// neznámá nebo už odvolaná relace vrátí ErrNotFound.
func (s *Store) RevokeDeviceSession(ctx context.Context, userID, id uuid.UUID, now time.Time) (*model.DeviceSession, error) {
	d, err := scanDeviceSession(s.db.QueryRowContext(ctx, `UPDATE device_sessions SET revoked_at = ?3
		WHERE id = ?1 AND user_id = ?2 AND revoked_at IS NULL
		RETURNING `+deviceSessionColumns, id, userID, now.Unix()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("revoke device session: %w", err)
	}
	return d, nil
}

// TouchDeviceSession zapíše čas poslední aktivity relace, pokud je starší
// než minSeen. Podmínka v dotazu brání zbytečným zápisům z více požadavků.
func (s *Store) TouchDeviceSession(ctx context.Context, id uuid.UUID, now, minSeen time.Time) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE device_sessions SET last_seen_at = ?2 WHERE id = ?1 AND last_seen_at < ?3`,
		id, now.Unix(), minSeen.Unix()); err != nil {
		return fmt.Errorf("touch device session: %w", err)
	}
	return nil
}
