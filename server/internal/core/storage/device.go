package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DeviceCode je rozpracované párování televize.
type DeviceCode struct {
	DeviceHash string
	UserCode   string
	DeviceName string
	UserID     uuid.UUID // uuid.Nil, dokud kód nikdo neschválil
	ExpiresAt  time.Time
	LastPollAt time.Time // nulový čas = TV se ještě neptala
}

const deviceCodeColumns = `device_hash, user_code, device_name, user_id, expires_at, last_poll_at`

func scanDeviceCode(row interface{ Scan(...any) error }) (*DeviceCode, error) {
	var (
		d                 DeviceCode
		userID            sql.NullString
		expires, lastPoll int64
	)
	if err := row.Scan(&d.DeviceHash, &d.UserCode, &d.DeviceName, &userID, &expires, &lastPoll); err != nil {
		return nil, err
	}
	if userID.Valid {
		id, err := uuid.Parse(userID.String)
		if err != nil {
			return nil, fmt.Errorf("device code user_id: %w", err)
		}
		d.UserID = id
	}
	d.ExpiresAt = time.Unix(expires, 0)
	if lastPoll > 0 {
		d.LastPollAt = time.Unix(lastPoll, 0)
	}
	return &d, nil
}

// PurgeDeviceCodes smaže prošlá párování a vrátí počet zbylých.
func (s *Store) PurgeDeviceCodes(ctx context.Context, now time.Time) (int, error) {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM device_codes WHERE expires_at <= ?1`, now.Unix()); err != nil {
		return 0, fmt.Errorf("purge device codes: %w", err)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_codes`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count device codes: %w", err)
	}
	return n, nil
}

// InsertDeviceCode uloží nové párování. Obsazený user_code vrátí ErrConflict.
func (s *Store) InsertDeviceCode(ctx context.Context, d DeviceCode) error {
	const q = `INSERT INTO device_codes (device_hash, user_code, device_name, expires_at) VALUES (?1, ?2, ?3, ?4)`
	_, err := s.db.ExecContext(ctx, q, d.DeviceHash, d.UserCode, d.DeviceName, d.ExpiresAt.Unix())
	if IsUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("insert device code: %w", err)
	}
	return nil
}

// DeviceCodeByUserCode vrátí platné neschválené párování podle kódu z obrazovky.
func (s *Store) DeviceCodeByUserCode(ctx context.Context, userCode string, now time.Time) (*DeviceCode, error) {
	q := `SELECT ` + deviceCodeColumns + ` FROM device_codes
		WHERE user_code = ?1 AND user_id IS NULL AND expires_at > ?2`
	d, err := scanDeviceCode(s.db.QueryRowContext(ctx, q, userCode, now.Unix()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device code: %w", err)
	}
	return d, nil
}

// ApproveDeviceCode přiřadí platné neschválené párování uživateli.
func (s *Store) ApproveDeviceCode(ctx context.Context, userCode string, userID uuid.UUID, now time.Time) (*DeviceCode, error) {
	q := `UPDATE device_codes SET user_id = ?3
		WHERE user_code = ?1 AND user_id IS NULL AND expires_at > ?2
		RETURNING ` + deviceCodeColumns
	d, err := scanDeviceCode(s.db.QueryRowContext(ctx, q, userCode, now.Unix(), userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("approve device code: %w", err)
	}
	return d, nil
}

// PollDeviceCode zaznamená dotaz televize a vrátí párování ve stavu PŘED
// dotazem (s předchozím last_poll_at, ať jde poznat příliš častý dotaz).
func (s *Store) PollDeviceCode(ctx context.Context, deviceHash string, now time.Time) (*DeviceCode, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("poll device code: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	d, err := scanDeviceCode(tx.QueryRowContext(ctx, `SELECT `+deviceCodeColumns+` FROM device_codes WHERE device_hash = ?1`, deviceHash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("poll device code: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE device_codes SET last_poll_at = ?2 WHERE device_hash = ?1`, deviceHash, now.Unix()); err != nil {
		return nil, fmt.Errorf("poll device code: %w", err)
	}
	return d, tx.Commit()
}

// ConsumeDeviceCode smaže schválené párování. Vrátí ErrNotFound, když ho
// mezitím spotřeboval souběžný dotaz – token se tak vydá jen jednou.
func (s *Store) ConsumeDeviceCode(ctx context.Context, deviceHash string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM device_codes WHERE device_hash = ?1 AND user_id IS NOT NULL`, deviceHash)
	if err != nil {
		return fmt.Errorf("consume device code: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDeviceCode smaže párování (prošlé).
func (s *Store) DeleteDeviceCode(ctx context.Context, deviceHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM device_codes WHERE device_hash = ?1`, deviceHash); err != nil {
		return fmt.Errorf("delete device code: %w", err)
	}
	return nil
}
