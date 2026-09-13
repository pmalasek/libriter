package service

import (
	"context"
	"errors"
	"fmt"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	store *storage.Store
	// auth je volitelný – potřebuje ho jen server, aby po změně role nebo
	// smazání účtu zahodil zapamatovanou roli. CLI běží ve vlastním procesu
	// a předává nil; tam se změna projeví po vypršení authCacheTTL.
	auth *AuthService
}

func NewUser(store *storage.Store, auth *AuthService) *UserService {
	return &UserService{store: store, auth: auth}
}

// invalidate zahodí cache role, pokud je služba zapojená na AuthService.
func (u *UserService) invalidate(id uuid.UUID) {
	if u.auth != nil {
		u.auth.InvalidateUser(id)
	}
}

func (u *UserService) List(ctx context.Context) ([]model.User, error) {
	return u.store.ListUsers(ctx)
}

// Create vytvoří uživatele s explicitní rolí. Používá CLI (`libriter user add`);
// API registrace jde přes AuthService.Register, která vždy přiřadí roli reader.
func (u *UserService) Create(ctx context.Context, displayName, email, password, role string) (*model.User, error) {
	return createUser(ctx, u.store, displayName, email, password, role)
}

// GetByEmail vrátí uživatele podle emailu (používá CLI pro adresování uživatele).
func (u *UserService) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	usr, err := u.store.GetUserByEmail(ctx, email)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return usr, err
}

func (u *UserService) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	usr, err := u.store.GetUserByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return usr, err
}

func (u *UserService) Update(ctx context.Context, id uuid.UUID, displayName, email string) (*model.User, error) {
	usr, err := u.store.UpdateUser(ctx, id, displayName, email)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if errors.Is(err, storage.ErrConflict) {
		return nil, ErrEmailTaken
	}
	return usr, err
}

func (u *UserService) ChangePassword(ctx context.Context, id uuid.UUID, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err = u.store.UpdateUserPassword(ctx, id, string(hash)); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func (u *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := u.store.DeleteUser(ctx, id); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	u.invalidate(id)
	return nil
}

func (u *UserService) SetRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	if _, ok := model.RoleLevel[roleName]; !ok {
		return fmt.Errorf("neznámá role: %s", roleName)
	}
	if err := u.store.SetUserRole(ctx, userID, roleName); err != nil {
		return err
	}
	u.invalidate(userID)
	return nil
}
