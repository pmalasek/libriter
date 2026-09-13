package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"libriter/internal/config"
	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// authCacheTTL je doba, po kterou middleware věří jednou načtené roli, aniž by
// se znovu ptal databáze. Změny provedené přes API se projeví okamžitě (viz
// InvalidateUser), zásah zvenčí – například `libriter user set-role` na běžícím
// serveru – se projeví nejpozději po této době.
const authCacheTTL = 10 * time.Second

// Claims jsou data zakódovaná v JWT tokenu.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	store *storage.Store
	cfg   config.JWTConfig

	// cacheTTL je vytažená do pole kvůli testům; produkčně je to authCacheTTL.
	cacheTTL time.Duration
	cacheMu  sync.Mutex
	cache    map[uuid.UUID]cachedUser
}

// cachedUser je krátkodobě zapamatovaný výsledek ověření uživatele z tokenu.
type cachedUser struct {
	role      string
	expiresAt time.Time
}

func NewAuth(store *storage.Store, cfg config.JWTConfig) *AuthService {
	return &AuthService{
		store:    store,
		cfg:      cfg,
		cacheTTL: authCacheTTL,
		cache:    make(map[uuid.UUID]cachedUser),
	}
}

// ResolveRole ověří, že uživatel z tokenu v databázi stále existuje, a vrátí
// jeho aktuální roli. Middleware ji volá místo toho, aby věřil roli zapečené
// v tokenu – jinak by smazaný nebo přeřazený uživatel jel na stará práva až do
// expirace tokenu. Vrací ErrNotFound, pokud uživatel neexistuje.
func (a *AuthService) ResolveRole(ctx context.Context, userID uuid.UUID) (string, error) {
	if role, ok := a.cachedRole(userID); ok {
		return role, nil
	}

	u, err := a.store.GetUserByID(ctx, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get user: %w", err)
	}

	// Cachují se jen úspěšné odpovědi. Token smazaného uživatele tak sahá do
	// databáze při každém požadavku, což je přesně ta strana, kde chceme jistotu.
	a.cacheMu.Lock()
	a.cache[userID] = cachedUser{role: u.Role, expiresAt: time.Now().Add(a.cacheTTL)}
	a.cacheMu.Unlock()

	return u.Role, nil
}

// InvalidateUser zahodí zapamatovanou roli uživatele. Volá se po změně role
// nebo smazání účtu, aby zásah platil okamžitě.
func (a *AuthService) InvalidateUser(userID uuid.UUID) {
	a.cacheMu.Lock()
	delete(a.cache, userID)
	a.cacheMu.Unlock()
}

func (a *AuthService) cachedRole(userID uuid.UUID) (string, bool) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()

	entry, ok := a.cache[userID]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expiresAt) {
		delete(a.cache, userID)
		return "", false
	}
	return entry.role, true
}

// Register vytvoří nového uživatele s rolí reader (výchozí) a vrátí JWT token.
func (a *AuthService) Register(ctx context.Context, displayName, email, password string) (*model.User, string, error) {
	u, err := createUser(ctx, a.store, displayName, email, password, model.RoleReader)
	if err != nil {
		return nil, "", err
	}

	token, err := a.generateToken(u.ID, u.Role)
	if err != nil {
		return nil, "", err
	}

	return u, token, nil
}

// createUser je společná cesta vytvoření uživatele pro API registraci i CLI.
// Zahashuje heslo a uloží uživatele s danou rolí.
func createUser(ctx context.Context, store *storage.Store, displayName, email, password, role string) (*model.User, error) {
	if _, ok := model.RoleLevel[role]; !ok {
		return nil, fmt.Errorf("neznámá role: %s", role)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u, err := store.CreateUser(ctx, displayName, email, string(hash), role)
	if errors.Is(err, storage.ErrConflict) {
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return u, nil
}

// Login ověří přihlašovací údaje a vrátí JWT token.
func (a *AuthService) Login(ctx context.Context, email, password string) (*model.User, string, error) {
	u, err := a.store.GetUserByEmail(ctx, email)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return nil, "", fmt.Errorf("get user: %w", err)
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	token, err := a.generateToken(u.ID, u.Role)
	if err != nil {
		return nil, "", err
	}

	return u, token, nil
}

// ParseToken ověří a dekóduje JWT token.
func (a *AuthService) ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("neočekávaná podpisová metoda: %v", t.Header["alg"])
		}
		return []byte(a.cfg.Secret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func (a *AuthService) generateToken(userID uuid.UUID, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.cfg.ExpiryHours)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(a.cfg.Secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// Chybové typy service vrstvy
var (
	ErrEmailTaken         = errors.New("email je již použit")
	ErrInvalidCredentials = errors.New("neplatné přihlašovací údaje")
	ErrInvalidToken       = errors.New("neplatný token")
	ErrForbidden          = errors.New("nedostatečná oprávnění")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("záznam již existuje")
)
