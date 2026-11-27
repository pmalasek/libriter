package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"mediater/internal/core/config"
	"mediater/internal/core/model"
	"mediater/internal/core/storage"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// MaxPasswordBytes je nejdelší heslo, které bcrypt umí zpracovat. Delší heslo
// by bcrypt odmítl (ErrPasswordTooLong) a z registrace by se stala chyba 500.
// Měří se v bajtech, ne ve znacích – „ž“ zabere bajty dva.
const MaxPasswordBytes = 72

// authCacheTTL je doba, po kterou middleware věří jednou načtené roli, aniž by
// se znovu ptal databáze. Změny provedené přes API se projeví okamžitě (viz
// InvalidateUser), zásah zvenčí – například `libriter user set-role` na běžícím
// serveru – se projeví nejpozději po této době.
const authCacheTTL = 10 * time.Second

// sessionTouchInterval je nejkratší rozestup zápisů last_seen_at jedné relace
// zařízení. Častější zápis by z každého požadavku mobilu udělal zápis do DB.
const sessionTouchInterval = time.Hour

// ScopeStream označuje token, který smí jen streamovat audio. Prvek <audio>
// neumí poslat hlavičku Authorization, takže token putuje v adrese – a tam
// přihlašovací token patřit nemá (zůstává v historii i v logu proxy).
const ScopeStream = "stream"

// streamTokenTTL je platnost tokenu na audio. Delší poslech ho přežije,
// odcizená adresa zestárne do druhého dne.
const streamTokenTTL = 24 * time.Hour

// ScopeMobile označuje dlouhodobý token mobilní aplikace. Do API pouští
// jen s právy čtenáře (viz EffectiveRole), na audio ale nestačí – to má
// vlastní scope, aby se mobilní token nikdy neocitl v adrese souboru.
const ScopeMobile = "mobile"

// defaultMobileTokenTTL platí, když konfigurace mlčí (třeba v testech).
const defaultMobileTokenTTL = 365 * 24 * time.Hour

// Claims jsou data zakódovaná v JWT tokenu. Prázdný Scope má přihlašovací
// token; cokoliv jiného je token s vlastními pravidly (viz ScopeStream,
// ScopeMobile, ScopeTV).
//
// Ver je verze tokenů uživatele v okamžiku vydání (users.token_version);
// token bez claimu platí jako verze 0. Mobilní a televizní token nese v ID
// (jti) svou relaci zařízení, streamovací token vydaný zařízením ji nese v SID.
type Claims struct {
	Role  string `json:"role"`
	Scope string `json:"scope,omitempty"`
	Ver   int    `json:"ver"`
	SID   string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

// IsDevice říká, jestli token patří zařízení (mobil, televize), ne webové relaci.
func (c *Claims) IsDevice() bool {
	return c.Scope == ScopeMobile || c.Scope == ScopeTV
}

// sessionID vrátí ID relace zařízení, ke které token patří; prázdné u
// webového tokenu a u starších tokenů zařízení, které relaci ještě neměly.
func (c *Claims) sessionID() string {
	switch {
	case c.IsDevice():
		return c.ID
	case c.Scope == ScopeStream:
		return c.SID
	}
	return ""
}

type AuthService struct {
	store *storage.Store
	cfg   config.JWTConfig

	// cacheTTL je vytažená do pole kvůli testům; produkčně je to authCacheTTL.
	cacheTTL time.Duration
	cacheMu  sync.Mutex
	cache    map[uuid.UUID]cachedUser
	// cacheGen se zvýší při každém InvalidateUser. Načtení z databáze, které
	// běželo souběžně se zneplatněním, se pak do cache nezapíše – jinak by
	// stará verze tokenů mohla přežít změnu hesla o celou dobu cacheTTL.
	cacheGen uint64

	// now jsou hodiny služby (párování televizí, cache, relace zařízení);
	// testy je posouvají.
	now func() time.Time
}

// cachedUser je krátkodobě zapamatovaný výsledek ověření uživatele z tokenu.
type cachedUser struct {
	role      string
	version   int
	sessions  map[uuid.UUID]cachedSession
	expiresAt time.Time
}

// cachedSession je zapamatovaný stav relace zařízení.
type cachedSession struct {
	revoked  bool
	lastSeen time.Time
}

func NewAuth(store *storage.Store, cfg config.JWTConfig) *AuthService {
	// Hash pro neexistující účty se počítá předem, ať první neúspěšné
	// přihlášení netrvá déle než ostatní.
	go dummyPasswordHash()
	return &AuthService{
		store:    store,
		cfg:      cfg,
		cacheTTL: authCacheTTL,
		cache:    make(map[uuid.UUID]cachedUser),
		now:      time.Now,
	}
}

// dummyPasswordHash je bcrypt hash se stejnou cenou jako skutečná hesla.
// Login s ním porovná heslo k neexistujícímu účtu, aby délka odpovědi
// neprozradila, které účty existují.
var dummyPasswordHash = sync.OnceValue(func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("mediater: účet neexistuje"), bcryptCost)
	if err != nil {
		panic(fmt.Sprintf("dummy bcrypt hash: %v", err))
	}
	return h
})

// EffectiveRole vrátí roli, se kterou token smí do API. Tokeny zařízení
// (mobil, televize) dostanou nejvýš práva čtenáře – žijí rok, leží v zařízení
// a ke správě knihovny ani účtů nejsou potřeba.
func EffectiveRole(role, scope string) string {
	if scope != ScopeMobile && scope != ScopeTV {
		return role
	}
	if level, ok := model.RoleLevel[role]; ok && level >= model.RoleLevel[model.RoleReader] {
		return role
	}
	return model.RoleReader
}

// Authorize ověří token proti databázi a vrátí roli, se kterou smí volající
// do API. Platný podpis nestačí:
//   - uživatel musí stále existovat (jinak ErrNotFound),
//   - verze tokenu (claim ver) musí odpovídat users.token_version – změna
//     hesla nebo „Odhlásit všude“ ji zvýší (jinak ErrSessionRevoked),
//   - token zařízení s relací (jti, u streamovacího tokenu sid) nesmí mít
//     relaci odvolanou (jinak ErrSessionRevoked).
//
// Role se bere z databáze, ne z tokenu; tokeny zařízení omezí EffectiveRole.
// Výsledek se krátce cachuje, každé odvolání volá InvalidateUser.
func (a *AuthService) Authorize(ctx context.Context, claims *Claims) (string, error) {
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return "", ErrInvalidToken
	}
	u, err := a.userAuth(ctx, userID)
	if err != nil {
		return "", err
	}
	if claims.Ver != u.version {
		return "", ErrSessionRevoked
	}
	if sid := claims.sessionID(); sid != "" {
		id, err := uuid.Parse(sid)
		if err != nil {
			return "", ErrInvalidToken
		}
		if err := a.checkSession(ctx, userID, id); err != nil {
			return "", err
		}
	}
	return EffectiveRole(u.role, claims.Scope), nil
}

// userAuth vrátí roli a verzi tokenů uživatele, z cache nebo z databáze.
func (a *AuthService) userAuth(ctx context.Context, userID uuid.UUID) (cachedUser, error) {
	now := a.now()
	a.cacheMu.Lock()
	if entry, ok := a.cache[userID]; ok {
		if now.Before(entry.expiresAt) {
			a.cacheMu.Unlock()
			return entry, nil
		}
		delete(a.cache, userID)
	}
	gen := a.cacheGen
	a.cacheMu.Unlock()

	ua, err := a.store.GetUserAuth(ctx, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return cachedUser{}, ErrNotFound
	}
	if err != nil {
		return cachedUser{}, fmt.Errorf("get user: %w", err)
	}

	// Cachují se jen úspěšné odpovědi. Token smazaného uživatele tak sahá do
	// databáze při každém požadavku, což je přesně ta strana, kde chceme jistotu.
	entry := cachedUser{
		role:      ua.Role,
		version:   ua.TokenVersion,
		sessions:  make(map[uuid.UUID]cachedSession),
		expiresAt: now.Add(a.cacheTTL),
	}
	a.cacheMu.Lock()
	if a.cacheGen == gen {
		a.cache[userID] = entry
	}
	a.cacheMu.Unlock()
	return entry, nil
}

// checkSession ověří, že relace zařízení patří uživateli a není odvolaná.
// Mimochodem zapíše čas poslední aktivity, nejvýš jednou za hodinu.
func (a *AuthService) checkSession(ctx context.Context, userID, sid uuid.UUID) error {
	now := a.now()

	a.cacheMu.Lock()
	s, cached := a.cache[userID].sessions[sid]
	gen := a.cacheGen
	a.cacheMu.Unlock()

	if !cached {
		d, err := a.store.GetDeviceSession(ctx, sid)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			s = cachedSession{revoked: true}
		case err != nil:
			return fmt.Errorf("get device session: %w", err)
		default:
			s = cachedSession{revoked: d.RevokedAt != nil || d.UserID != userID, lastSeen: d.LastSeenAt}
		}
		a.cacheSession(userID, sid, s, gen)
	}
	if s.revoked {
		return ErrSessionRevoked
	}

	if now.Sub(s.lastSeen) >= sessionTouchInterval {
		// Chyba zápisu aktivity požadavek neshodí – jde jen o údaj v přehledu.
		if err := a.store.TouchDeviceSession(ctx, sid, now, now.Add(-sessionTouchInterval)); err != nil {
			slog.Warn("zápis aktivity zařízení", "session", sid, "err", err)
		} else {
			s.lastSeen = now
			a.cacheSession(userID, sid, s, gen)
		}
	}
	return nil
}

func (a *AuthService) cacheSession(userID, sid uuid.UUID, s cachedSession, gen uint64) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()
	if entry, ok := a.cache[userID]; ok && a.cacheGen == gen {
		entry.sessions[sid] = s
	}
}

// InvalidateUser zahodí zapamatovaný stav uživatele (roli, verzi tokenů,
// relace zařízení). Volá se po změně role, smazání účtu, změně hesla a
// odvolání zařízení, aby zásah platil okamžitě.
func (a *AuthService) InvalidateUser(userID uuid.UUID) {
	a.cacheMu.Lock()
	delete(a.cache, userID)
	a.cacheGen++
	a.cacheMu.Unlock()
}

// Register vytvoří nového uživatele s danou rolí a vrátí JWT token.
// Roli určuje nastavení registrace (viz SettingsService), ne klient.
// Prázdný uiLanguage ponechá výchozí jazyk rozhraní.
func (a *AuthService) Register(ctx context.Context, displayName, email, login, password, role, uiLanguage string) (*model.User, string, error) {
	if uiLanguage != "" && !slices.Contains(model.UILanguages, uiLanguage) {
		return nil, "", ErrInvalidLanguage
	}
	u, err := createUser(ctx, a.store, displayName, email, login, password, role)
	if err != nil {
		return nil, "", err
	}
	if uiLanguage != "" && uiLanguage != u.UILanguage {
		if u, err = a.store.UpdateUserLanguage(ctx, u.ID, uiLanguage); err != nil {
			return nil, "", fmt.Errorf("set ui language: %w", err)
		}
	}

	// Nový účet má verzi tokenů 0 (výchozí hodnota sloupce).
	token, err := a.generateToken(u.ID, u.Role, 0)
	if err != nil {
		return nil, "", err
	}

	return u, token, nil
}

// createUser je společná cesta vytvoření uživatele pro API registraci i CLI.
// Ověří login, zahashuje heslo a uloží uživatele s danou rolí.
func createUser(ctx context.Context, store *storage.Store, displayName, email, login, password, role string) (*model.User, error) {
	if _, ok := model.RoleLevel[role]; !ok {
		return nil, fmt.Errorf("neznámá role: %s", role)
	}
	login, err := NormalizeLogin(login)
	if err != nil {
		return nil, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	u, err := store.CreateUser(ctx, displayName, email, login, string(hash), role)
	if errors.Is(err, storage.ErrConflict) {
		return nil, ErrEmailTaken
	}
	if errors.Is(err, storage.ErrLoginConflict) {
		return nil, ErrLoginTaken
	}
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return u, nil
}

// hashPassword zahashuje heslo; příliš dlouhé vrátí ErrPasswordTooLong.
func hashPassword(password string) ([]byte, error) {
	if len(password) > MaxPasswordBytes {
		return nil, ErrPasswordTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}

// Login ověří přihlašovací údaje a vrátí JWT token. Parametr login se
// porovnává s přihlašovacím jménem i s e-mailem uživatele.
func (a *AuthService) Login(ctx context.Context, login, password string) (*model.User, string, error) {
	u, err := a.store.GetUserByIdentifier(ctx, login)
	if errors.Is(err, storage.ErrNotFound) {
		// Stejná práce jako u existujícího účtu – podle doby odpovědi se
		// nesmí dát poznat, který login existuje.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
		return nil, "", ErrInvalidCredentials
	}
	if err != nil {
		return nil, "", fmt.Errorf("get user: %w", err)
	}

	if err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	ua, err := a.store.GetUserAuth(ctx, u.ID)
	if err != nil {
		return nil, "", fmt.Errorf("get user auth: %w", err)
	}
	token, err := a.generateToken(u.ID, u.Role, ua.TokenVersion)
	if err != nil {
		return nil, "", err
	}

	return u, token, nil
}

// IssueWebToken vydá nový přihlašovací token s aktuální verzí tokenů.
// Po změně hesla nebo „Odhlásit všude“ ho dostane web, aby zůstal přihlášený,
// zatímco všechny dřívější tokeny přestaly platit.
func (a *AuthService) IssueWebToken(ctx context.Context, userID uuid.UUID) (string, error) {
	ua, err := a.store.GetUserAuth(ctx, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get user auth: %w", err)
	}
	return a.generateToken(userID, ua.Role, ua.TokenVersion)
}

// ParseToken ověří a dekóduje token, kterým se chodí do API: přihlašovací
// (prázdný scope), mobilní nebo televizní. Token na streamování odmítne – jinak by
// adresa audia posloužila jako přihlášení do celého API.
func (a *AuthService) ParseToken(tokenStr string) (*Claims, error) {
	claims, err := a.parseClaims(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.Scope != "" && claims.Scope != ScopeMobile && claims.Scope != ScopeTV {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// GenerateMobileToken vydá dlouhodobý token pro mobilní aplikaci a čas jeho
// vypršení. Mobil ho dostane výměnou za běžné přihlášení a uloží si ho do
// bezpečného úložiště; do API chodí místo přihlašovacího tokenu, který by mu
// vypršel dřív, než se telefon zase dostane na síť. Token dostane vlastní
// relaci zařízení, kterou jde v profilu odvolat.
func (a *AuthService) GenerateMobileToken(ctx context.Context, userID uuid.UUID, deviceName string) (string, time.Time, error) {
	return a.generateDeviceToken(ctx, userID, ScopeMobile, deviceName)
}

// generateDeviceToken založí relaci zařízení a vydá k ní dlouhodobý token
// (mobil, televize) s aktuální verzí tokenů uživatele.
func (a *AuthService) generateDeviceToken(ctx context.Context, userID uuid.UUID, scope, deviceName string) (string, time.Time, error) {
	ua, err := a.store.GetUserAuth(ctx, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return "", time.Time{}, ErrNotFound
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("get user auth: %w", err)
	}
	sess, err := a.store.CreateDeviceSession(ctx, userID, scope, CleanDeviceName(deviceName), a.now())
	if err != nil {
		return "", time.Time{}, err
	}

	ttl := a.cfg.MobileExpiry
	if ttl <= 0 {
		ttl = defaultMobileTokenTTL
	}
	now := time.Now()
	expiresAt := now.Add(ttl)
	token, err := a.signToken(Claims{
		Role:  ua.Role,
		Scope: scope,
		Ver:   ua.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sess.ID.String(),
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

// CleanDeviceName upraví název zařízení od klienta: bez řídicích znaků,
// oříznutý, nejvýš 80 znaků.
func CleanDeviceName(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(name, ""))
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 80 {
		name = strings.TrimSpace(string(r[:80]))
	}
	return name
}

// GenerateStreamToken vydá krátkodobý token pro přehrávání a čas jeho
// vypršení. Přehrávač si ho vyžádá běžným přihlášeným požadavkem a přidává ho
// do adresy souboru. Token zdědí verzi tokenů a relaci zařízení volajícího
// (parent), takže ho odvolání zařízení i „Odhlásit všude“ zneplatní také.
func (a *AuthService) GenerateStreamToken(parent *Claims) (string, time.Time, error) {
	if parent == nil {
		return "", time.Time{}, ErrInvalidToken
	}
	var sid string
	if parent.IsDevice() {
		sid = parent.ID
	}
	now := time.Now()
	expiresAt := now.Add(streamTokenTTL)
	token, err := a.signToken(Claims{
		Scope: ScopeStream,
		Ver:   parent.Ver,
		SID:   sid,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   parent.Subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	})
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// ParseStreamToken ověří podpis tokenu na streamování a vrátí jeho claims;
// platnost vůči databázi ověří až Authorize. Přihlašovací token tudy neprojde
// – ať se v adresách objevuje jen to, co do nich patří.
func (a *AuthService) ParseStreamToken(tokenStr string) (*Claims, error) {
	claims, err := a.parseClaims(tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.Scope != ScopeStream {
		return nil, ErrInvalidToken
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (a *AuthService) parseClaims(tokenStr string) (*Claims, error) {
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

func (a *AuthService) generateToken(userID uuid.UUID, role string, ver int) (string, error) {
	now := time.Now()
	return a.signToken(Claims{
		Role: role,
		Ver:  ver,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.cfg.ExpiryHours)),
		},
	})
}

func (a *AuthService) signToken(claims Claims) (string, error) {
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
	ErrLoginTaken         = errors.New("login je již použit")
	ErrInvalidLogin       = errors.New("neplatný login")
	ErrInvalidCredentials = errors.New("neplatné přihlašovací údaje")
	ErrInvalidToken       = errors.New("neplatný token")
	ErrSessionRevoked     = errors.New("přihlášení bylo odvoláno")
	ErrPasswordTooLong    = errors.New("heslo je delší než 72 bajtů")
	ErrForbidden          = errors.New("nedostatečná oprávnění")
	ErrNotFound           = errors.New("not found")
	ErrConflict           = errors.New("záznam již existuje")
	ErrLastAdmin          = errors.New("poslední administrátor")
	ErrInvalidLanguage    = errors.New("nepodporovaný jazyk rozhraní")
)
