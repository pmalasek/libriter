// Package product definuje rozhraní, kterým se do serveru zapojuje produkt
// (Libriter, Sceniter). Jádro dodá sdílené služby, produkt přidá vlastní
// schéma, routy a webové rozhraní.
package product

import (
	"context"
	"database/sql"
	"io/fs"
	"time"

	"github.com/go-chi/chi/v5"

	"mediater/internal/core/config"
	"mediater/internal/core/imgthumb"
	"mediater/internal/core/plugin"
	"mediater/internal/core/service"
)

// Deps jsou sdílené závislosti, které jádro předá produktu při startu.
type Deps struct {
	// Ctx žije po celou dobu běhu serveru a zruší se při ukončení – produkt
	// na něj váže scanner a další goroutiny.
	Ctx          context.Context
	Cfg          *config.Config
	DB           *sql.DB
	Auth         *service.AuthService
	Users        *service.UserService
	Registration *service.RegistrationService
	Audit        *service.AuditService
	// Plugins spouští zdroje metadat; PluginSettings drží jejich konfiguraci.
	Plugins        *plugin.Host
	PluginSettings *plugin.Settings
	// Thumbs vytváří zmenšeniny obrázků pro mřížky (?w= u obrázkových
	// endpointů). Nil = posílají se jen originály.
	Thumbs    *imgthumb.Cache
	StartedAt time.Time
}

// Product je jeden produkt nad společnými účty.
type Product interface {
	// Name je krátký identifikátor („libriter“, „sceniter“). Slouží jako klíč
	// migrací modulu, předpona proměnné <NAME>_HOST a výchozí produkt.
	Name() string
	// Migrations vrátí soubory NNN_popis.sql v kořeni FS; číslování je
	// vlastní řada modulu. Nil = produkt žádné migrace nemá.
	Migrations() fs.FS
	// Init sestaví služby produktu a spustí jeho běžící části (scanner).
	// Volá se po migracích.
	Init(d Deps) error
	// RegisterRoutes zaregistruje API produktu. api je router /api/v1;
	// produkt si sám zvolí prefix (Libriter z kompatibility žádný nemá).
	RegisterRoutes(api chi.Router)
	// Web vrátí sestavené webové rozhraní (obsah dist). Nil = produkt web nemá.
	Web() fs.FS
}
