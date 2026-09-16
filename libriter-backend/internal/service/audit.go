package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// Akce zapisované do audit logu.
const (
	AuditUserCreate           = "user.create"
	AuditUserRoleChange       = "user.role_change"
	AuditUserDelete           = "user.delete"
	AuditUserPasswordReset    = "user.password_reset"
	AuditSettingsMetadata     = "settings.metadata_update"
	AuditSettingsRegistration = "settings.registration_update"
	AuditBookDelete           = "book.delete"
	AuditAuthorDelete         = "author.delete"
	AuditSeriesDelete         = "series.delete"
	AuditScannerRescan        = "scanner.rescan"
	AuditLibraryRepair        = "library.repair_apply"
	AuditLibraryMerge         = "library.merge_books"
)

// Typy cílů akce (pro rozhraní, ne pro logiku).
const (
	AuditTargetUser     = "user"
	AuditTargetBook     = "book"
	AuditTargetAuthor   = "author"
	AuditTargetSeries   = "series"
	AuditTargetSettings = "settings"
	AuditTargetLibrary  = "library"
)

// AuditEvent popisuje jednu zaznamenávanou akci.
type AuditEvent struct {
	Action      string
	TargetType  string
	TargetID    string
	TargetLabel string
	Details     any
}

// AuditService zapisuje administrativní akce a vrací jejich výpis.
type AuditService struct {
	store *storage.Store
	log   *slog.Logger
}

func NewAudit(store *storage.Store) *AuditService {
	return &AuditService{store: store, log: slog.Default().With("component", "audit")}
}

// Výchozí a maximální počet záznamů ve výpisu.
const (
	auditDefaultLimit = 50
	auditMaxLimit     = 200
)

// Record zapíše akci. Selhání zápisu nesmí shodit operaci, kterou audituje –
// chyba se jen zaloguje. Nil příjemce je v pořádku (CLI audit nevede).
func (a *AuditService) Record(ctx context.Context, actorID uuid.UUID, ev AuditEvent) {
	if a == nil {
		return
	}

	in := storage.AuditInput{
		Action:      ev.Action,
		TargetType:  ev.TargetType,
		TargetID:    ev.TargetID,
		TargetLabel: ev.TargetLabel,
	}

	if actorID != uuid.Nil {
		id := actorID
		in.ActorID = &id
		if actor, err := a.store.GetUserByID(ctx, actorID); err == nil {
			in.ActorEmail = actor.Email
		}
	}

	if ev.Details != nil {
		details, err := json.Marshal(ev.Details)
		if err != nil {
			a.log.Warn("detaily auditu nelze serializovat", "akce", ev.Action, "err", err)
		} else {
			in.Details = details
		}
	}

	if _, err := a.store.InsertAudit(ctx, in); err != nil {
		a.log.Error("zápis do audit logu selhal", "akce", ev.Action, "err", err)
	}
}

// List vrátí záznamy od nejnovějšího. before > 0 pokračuje pod daným id.
func (a *AuditService) List(ctx context.Context, limit int, before int64) ([]model.AuditEntry, error) {
	switch {
	case limit <= 0:
		limit = auditDefaultLimit
	case limit > auditMaxLimit:
		limit = auditMaxLimit
	}
	if before < 0 {
		before = 0
	}
	return a.store.ListAudit(ctx, limit, before)
}
