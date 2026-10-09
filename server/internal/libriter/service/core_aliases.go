package service

import coreservice "mediater/internal/core/service"

// Služby identity žijí v jádře (mediater/internal/core/service); knižní kód je
// dál používá pod původními jmény.
type (
	UserService  = coreservice.UserService
	AuditEvent   = coreservice.AuditEvent
	AuditService = coreservice.AuditService
	Claims       = coreservice.Claims
	AuthService  = coreservice.AuthService
)

const (
	AuditUserCreate           = coreservice.AuditUserCreate
	AuditUserRoleChange       = coreservice.AuditUserRoleChange
	AuditUserDelete           = coreservice.AuditUserDelete
	AuditUserPasswordReset    = coreservice.AuditUserPasswordReset
	AuditSettingsMetadata     = coreservice.AuditSettingsMetadata
	AuditSettingsRegistration = coreservice.AuditSettingsRegistration
	AuditSettingsLibrary      = coreservice.AuditSettingsLibrary
	AuditBookDelete           = coreservice.AuditBookDelete
	AuditAuthorDelete         = coreservice.AuditAuthorDelete
	AuditSeriesDelete         = coreservice.AuditSeriesDelete
	AuditScannerRescan        = coreservice.AuditScannerRescan
	AuditLibraryRepair        = coreservice.AuditLibraryRepair
	AuditLibraryMerge         = coreservice.AuditLibraryMerge
	AuditDuplicateDismiss     = coreservice.AuditDuplicateDismiss
	AuditDuplicateRestore     = coreservice.AuditDuplicateRestore
	AuditLibraryImport        = coreservice.AuditLibraryImport
	AuditTargetUser           = coreservice.AuditTargetUser
	AuditTargetBook           = coreservice.AuditTargetBook
	AuditTargetAuthor         = coreservice.AuditTargetAuthor
	AuditTargetSeries         = coreservice.AuditTargetSeries
	AuditTargetSettings       = coreservice.AuditTargetSettings
	AuditTargetLibrary        = coreservice.AuditTargetLibrary
	ScopeStream               = coreservice.ScopeStream
	ScopeMobile               = coreservice.ScopeMobile
)

var (
	ErrEmailTaken         = coreservice.ErrEmailTaken
	ErrLoginTaken         = coreservice.ErrLoginTaken
	ErrInvalidLogin       = coreservice.ErrInvalidLogin
	ErrInvalidCredentials = coreservice.ErrInvalidCredentials
	ErrInvalidToken       = coreservice.ErrInvalidToken
	ErrForbidden          = coreservice.ErrForbidden
	ErrNotFound           = coreservice.ErrNotFound
	ErrConflict           = coreservice.ErrConflict
	ErrLastAdmin          = coreservice.ErrLastAdmin
	ErrInvalidLanguage    = coreservice.ErrInvalidLanguage
	NewUser               = coreservice.NewUser
	NormalizeLogin        = coreservice.NormalizeLogin
	NewAudit              = coreservice.NewAudit
	NewAuth               = coreservice.NewAuth
)
