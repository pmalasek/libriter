package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"mediater/internal/core/model"
	"mediater/internal/core/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ScopeTV označuje dlouhodobý token televizní aplikace. Do API pouští stejně
// jako mobilní token; televize ho získá párovacím kódem (device flow), ne
// heslem psaným ovladačem.
const ScopeTV = "tv"

// Párování televize: TV si vyžádá kód, uživatel ho schválí na webu a TV mezitím
// opakovaně zkouší, jestli už je schválený. Rozpracovaná párování leží
// v databázi (tabulka device_codes), takže přežijí restart serveru.
const (
	deviceCodeTTL      = 10 * time.Minute
	devicePollInterval = 5 * time.Second
	// deviceMaxPending omezuje počet rozpracovaných párování, ať anonymní
	// endpoint nejde použít k zaplnění databáze.
	deviceMaxPending = 1000
)

// userCodeAlphabet vynechává znaky, které se na televizi pletou (0/O, 1/I/L, 5/S, 2/Z, 8/B).
const userCodeAlphabet = "ACDEFGHJKMNPQRTUVWXY34679"

var (
	ErrDevicePending  = errors.New("párování čeká na schválení")
	ErrDeviceSlowDown = errors.New("párování se zkouší příliš často")
	ErrDeviceExpired  = errors.New("párovací kód vypršel")
	ErrDeviceBusy     = errors.New("příliš mnoho rozpracovaných párování")
)

// DeviceStart je odpověď na zahájení párování.
type DeviceStart struct {
	DeviceCode string `json:"device_code"`
	UserCode   string `json:"user_code"`
	ExpiresIn  int    `json:"expires_in"`
	Interval   int    `json:"interval"`
}

// DeviceInfo popisuje čekající párování pro stránku schválení.
type DeviceInfo struct {
	UserCode   string    `json:"user_code"`
	DeviceName string    `json:"device_name"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func hashDeviceCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// NormalizeUserCode převede ručně zadaný kód do kanonického tvaru ABCD-EFGH:
// velká písmena, bez mezer a pomlček.
func NormalizeUserCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if strings.ContainsRune(userCodeAlphabet, r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if len(s) != 8 {
		return s
	}
	return s[:4] + "-" + s[4:]
}

func randomUserCode() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 8)
	for i, v := range buf {
		out[i] = userCodeAlphabet[int(v)%len(userCodeAlphabet)]
	}
	return string(out[:4]) + "-" + string(out[4:]), nil
}

func deviceInfo(d *storage.DeviceCode) *DeviceInfo {
	return &DeviceInfo{UserCode: d.UserCode, DeviceName: d.DeviceName, ExpiresAt: d.ExpiresAt}
}

// StartDeviceAuth zahájí párování televize a vrátí kódy: device_code si TV
// nechá pro sebe, user_code ukáže na obrazovce.
func (a *AuthService) StartDeviceAuth(ctx context.Context, deviceName string) (*DeviceStart, error) {
	now := a.now()
	pending, err := a.store.PurgeDeviceCodes(ctx, now)
	if err != nil {
		return nil, err
	}
	if pending >= deviceMaxPending {
		return nil, ErrDeviceBusy
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("device code: %w", err)
	}
	deviceCode := base64.RawURLEncoding.EncodeToString(raw)

	name := strings.TrimSpace(deviceName)
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}

	// Kolize user_code je při 25^8 kombinacích vzácná; pár pokusů stačí.
	for range 5 {
		userCode, err := randomUserCode()
		if err != nil {
			return nil, fmt.Errorf("user code: %w", err)
		}
		err = a.store.InsertDeviceCode(ctx, storage.DeviceCode{
			DeviceHash: hashDeviceCode(deviceCode),
			UserCode:   userCode,
			DeviceName: name,
			ExpiresAt:  now.Add(deviceCodeTTL),
		})
		if errors.Is(err, storage.ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &DeviceStart{
			DeviceCode: deviceCode,
			UserCode:   userCode,
			ExpiresIn:  int(deviceCodeTTL.Seconds()),
			Interval:   int(devicePollInterval.Seconds()),
		}, nil
	}
	return nil, errors.New("user code: nepodařilo se najít volný kód")
}

// DeviceByUserCode vrátí čekající párování podle kódu z obrazovky, aby
// stránka schválení mohla ukázat, které zařízení se přihlašuje.
func (a *AuthService) DeviceByUserCode(ctx context.Context, userCode string) (*DeviceInfo, error) {
	d, err := a.store.DeviceCodeByUserCode(ctx, NormalizeUserCode(userCode), a.now())
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return deviceInfo(d), nil
}

// ApproveDevice přiřadí párování přihlášenému uživateli. Token si pak
// televize vyzvedne sama při dalším dotazu.
func (a *AuthService) ApproveDevice(ctx context.Context, userID uuid.UUID, userCode string) (*DeviceInfo, error) {
	d, err := a.store.ApproveDeviceCode(ctx, NormalizeUserCode(userCode), userID, a.now())
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return deviceInfo(d), nil
}

// PollDeviceToken vymění schválený device_code za dlouhodobý TV token.
// Kód jde vyměnit jen jednou; dokud není schválený, vrací ErrDevicePending,
// a kdo se ptá častěji než jednou za interval, dostane ErrDeviceSlowDown.
func (a *AuthService) PollDeviceToken(ctx context.Context, deviceCode string) (*model.User, string, time.Time, error) {
	now := a.now()
	hash := hashDeviceCode(deviceCode)

	d, err := a.store.PollDeviceCode(ctx, hash, now)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, "", time.Time{}, ErrDeviceExpired
	}
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if !now.Before(d.ExpiresAt) {
		_ = a.store.DeleteDeviceCode(ctx, hash)
		return nil, "", time.Time{}, ErrDeviceExpired
	}
	// Tolerance jedné sekundy na zpoždění sítě a časovačů klienta.
	if !d.LastPollAt.IsZero() && now.Sub(d.LastPollAt) < devicePollInterval-time.Second {
		return nil, "", time.Time{}, ErrDeviceSlowDown
	}
	if d.UserID == uuid.Nil {
		return nil, "", time.Time{}, ErrDevicePending
	}
	if err := a.store.ConsumeDeviceCode(ctx, hash); errors.Is(err, storage.ErrNotFound) {
		return nil, "", time.Time{}, ErrDeviceExpired
	} else if err != nil {
		return nil, "", time.Time{}, err
	}

	u, err := a.store.GetUserByID(ctx, d.UserID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, "", time.Time{}, ErrDeviceExpired
	}
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("get user: %w", err)
	}

	token, expiresAt, err := a.generateTVToken(u.ID, u.Role)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	return u, token, expiresAt, nil
}

// generateTVToken vydá dlouhodobý token televize; platí stejně dlouho jako
// mobilní token.
func (a *AuthService) generateTVToken(userID uuid.UUID, role string) (string, time.Time, error) {
	ttl := a.cfg.MobileExpiry
	if ttl <= 0 {
		ttl = defaultMobileTokenTTL
	}
	now := time.Now()
	expiresAt := now.Add(ttl)
	token, err := a.signToken(Claims{
		Role:  role,
		Scope: ScopeTV,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}
