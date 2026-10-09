package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"libriter/internal/model"

	"github.com/google/uuid"
)

// AuditInput je jeden záznam k zapsání do audit logu.
type AuditInput struct {
	ActorID     *uuid.UUID
	ActorEmail  string
	Action      string
	TargetType  string
	TargetID    string
	TargetLabel string
	Details     json.RawMessage
}

const auditColumns = `id, actor_id, actor_email, action, target_type, target_id,
	       target_label, details, created_at`

// InsertAudit zapíše záznam o administrativní akci.
func (s *Store) InsertAudit(ctx context.Context, in AuditInput) (*model.AuditEntry, error) {
	details := in.Details
	if len(details) == 0 {
		details = json.RawMessage("{}")
	}

	const q = `
		INSERT INTO audit_log (actor_id, actor_email, action, target_type, target_id,
		                       target_label, details)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)
		RETURNING ` + auditColumns

	row := s.db.QueryRowContext(ctx, q, in.ActorID, in.ActorEmail, in.Action,
		in.TargetType, in.TargetID, in.TargetLabel, string(details))
	return scanAudit(row)
}

// ListAudit vrátí záznamy od nejnovějšího. beforeID > 0 pokračuje ve výpisu
// pod daným id (stránkování kurzorem, ne offsetem – log mezitím roste).
func (s *Store) ListAudit(ctx context.Context, limit int, beforeID int64) ([]model.AuditEntry, error) {
	const q = `
		SELECT ` + auditColumns + `
		FROM audit_log
		WHERE ?2 = 0 OR id < ?2
		ORDER BY id DESC
		LIMIT ?1`

	rows, err := s.db.QueryContext(ctx, q, limit, beforeID)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()

	entries := []model.AuditEntry{}
	for rows.Next() {
		e, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, *e)
	}
	return entries, rows.Err()
}

func scanAudit(row scanner) (*model.AuditEntry, error) {
	var (
		e       model.AuditEntry
		actorID sql.NullString
		details string
	)
	err := row.Scan(&e.ID, &actorID, &e.ActorEmail, &e.Action, &e.TargetType,
		&e.TargetID, &e.TargetLabel, &details, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	if actorID.Valid {
		if id, perr := uuid.Parse(actorID.String); perr == nil {
			e.ActorID = &id
		}
	}
	e.Details = json.RawMessage(details)
	return &e, nil
}
