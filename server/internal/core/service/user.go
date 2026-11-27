package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"mediater/internal/core/model"
	"mediater/internal/core/storage"

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

// Create vytvoří uživatele s explicitní rolí. Používá ji CLI
// (`libriter user add`) i administrace; veřejná registrace jde přes
// AuthService.Register, kde roli určuje nastavení registrace.
func (u *UserService) Create(ctx context.Context, displayName, email, login, password, role string) (*model.User, error) {
	return createUser(ctx, u.store, displayName, email, login, password, role)
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

// Update změní jméno, e-mail a login. Prázdný login uživateli login odebere.
func (u *UserService) Update(ctx context.Context, id uuid.UUID, displayName, email, login string) (*model.User, error) {
	login, err := NormalizeLogin(login)
	if err != nil {
		return nil, err
	}
	usr, err := u.store.UpdateUser(ctx, id, displayName, email, login)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if errors.Is(err, storage.ErrConflict) {
		return nil, ErrEmailTaken
	}
	if errors.Is(err, storage.ErrLoginConflict) {
		return nil, ErrLoginTaken
	}
	return usr, err
}

// UpdateAppearance uloží barevné téma produktu ("" nebo "libriter",
// "sceniter") a společný režim zobrazení.
func (u *UserService) UpdateAppearance(ctx context.Context, id uuid.UUID, product, colorScheme, themeMode string) (*model.User, error) {
	switch product {
	case "", model.AppearanceLibriter, model.AppearanceSceniter:
	default:
		return nil, fmt.Errorf("%w: neznámý produkt", ErrInvalidSetting)
	}
	colorScheme, ok := model.NormalizeColorScheme(colorScheme)
	if !ok {
		return nil, fmt.Errorf("%w: neznámé barevné schéma", ErrInvalidSetting)
	}
	switch themeMode {
	case "light", "dark", "system":
	default:
		return nil, fmt.Errorf("%w: neznámý režim zobrazení", ErrInvalidSetting)
	}
	usr, err := u.store.UpdateUserAppearance(ctx, id, product, colorScheme, themeMode)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return usr, err
}

// UpdateLanguage nastaví jazyk rozhraní uživatele.
func (u *UserService) UpdateLanguage(ctx context.Context, id uuid.UUID, uiLanguage string) (*model.User, error) {
	if !slices.Contains(model.UILanguages, uiLanguage) {
		return nil, ErrInvalidLanguage
	}
	usr, err := u.store.UpdateUserLanguage(ctx, id, uiLanguage)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return usr, err
}

// ChangePassword nastaví nové heslo. Zároveň zvýší verzi tokenů a odvolá
// relace zařízení, takže se uživatel odhlásí všude – heslo se obvykle mění
// právě proto, že ho mohl znát někdo další. Web si nový token vyžádá přes
// IssueWebToken.
func (u *UserService) ChangePassword(ctx context.Context, id uuid.UUID, newPassword string) error {
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err = u.store.UpdateUserPassword(ctx, id, string(hash), time.Now()); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	u.invalidate(id)
	return nil
}

// CheckPassword ověří současné heslo uživatele (před změnou hesla, e-mailu
// nebo loginu). Špatné heslo vrátí ErrInvalidCredentials.
func (u *UserService) CheckPassword(ctx context.Context, id uuid.UUID, password string) error {
	usr, err := u.store.GetUserByID(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(usr.PasswordHash), []byte(password)) != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// LogoutAll zneplatní všechny tokeny uživatele (web, mobil, televize i
// streamovací) zvýšením verze tokenů a odvolá jeho relace zařízení.
func (u *UserService) LogoutAll(ctx context.Context, id uuid.UUID) error {
	if _, err := u.store.LogoutAll(ctx, id, time.Now()); errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	u.invalidate(id)
	return nil
}

// ListDevices vrátí přihlášená zařízení uživatele (neodvolané relace).
func (u *UserService) ListDevices(ctx context.Context, id uuid.UUID) ([]model.DeviceSession, error) {
	return u.store.ListDeviceSessions(ctx, id)
}

// RevokeDevice odvolá jednu relaci zařízení; ostatní zařízení zůstanou
// přihlášená. Vrátí odvolanou relaci (pro záznam v auditu).
func (u *UserService) RevokeDevice(ctx context.Context, userID, sessionID uuid.UUID) (*model.DeviceSession, error) {
	d, err := u.store.RevokeDeviceSession(ctx, userID, sessionID, time.Now())
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.invalidate(userID)
	return d, nil
}

// IssueWebToken vydá uživateli nový přihlašovací token s aktuální verzí
// tokenů (po změně hesla nebo „Odhlásit všude“). Bez zapojené AuthService
// (CLI) token vydat nejde.
func (u *UserService) IssueWebToken(ctx context.Context, id uuid.UUID) (string, error) {
	if u.auth == nil {
		return "", errors.New("vydání tokenu: chybí AuthService")
	}
	return u.auth.IssueWebToken(ctx, id)
}

func (u *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	err := u.store.DeleteUser(ctx, id)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, storage.ErrLastAdmin):
		return ErrLastAdmin
	case err != nil:
		return err
	}
	u.invalidate(id)
	return nil
}

func (u *UserService) SetRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	if _, ok := model.RoleLevel[roleName]; !ok {
		return fmt.Errorf("neznámá role: %s", roleName)
	}
	if err := u.store.SetUserRole(ctx, userID, roleName); err != nil {
		if errors.Is(err, storage.ErrLastAdmin) {
			return ErrLastAdmin
		}
		return err
	}
	u.invalidate(userID)
	return nil
}
