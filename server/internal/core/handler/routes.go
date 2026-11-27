package handler

import (
	"github.com/go-chi/chi/v5"

	"mediater/internal/core/middleware"
	"mediater/internal/core/model"
	"mediater/internal/core/service"
)

// Handlers sdružuje handlery jádra pro registraci rout.
type Handlers struct {
	Auth    *AuthHandler
	Users   *UserHandler
	Admin   *AdminHandler
	Plugins *PluginsHandler
	// Limits jsou limity veřejných přihlašovacích endpointů; nulová hodnota
	// = bez limitů (viz DefaultRateLimits).
	Limits RateLimits
}

// RegisterRoutes zaregistruje API jádra na router /api/v1: přihlášení,
// tokeny, profil, správu uživatelů a společnou administraci.
//
// Administrace se registruje plnými cestami (ne r.Route("/admin")) – chi by
// jinak druhému produktu nedovolil přidat vlastní /admin/... cesty.
func RegisterRoutes(r chi.Router, auth *service.AuthService, h Handlers) {
	// --- bez přihlášení ---
	r.Get("/auth/config", h.Auth.Config) // je registrace zapnutá?
	r.Method("POST", "/auth/register", limited(h.Limits.Register, h.Auth.Register))
	r.Method("POST", "/auth/login", limited(h.Limits.Login, h.Auth.Login))
	// Párování televize (device flow): TV ukáže kód, uživatel ho schválí na webu
	r.Method("POST", "/auth/device/start", limited(h.Limits.DeviceStart, h.Auth.DeviceStart))
	r.Method("POST", "/auth/device/token", limited(h.Limits.DeviceToken, h.Auth.DeviceToken))

	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(auth))

		// Token pro přehrávače (<audio>, <video> neumí poslat hlavičku)
		r.Get("/auth/stream-token", h.Auth.StreamToken)

		// Uživatelé - vlastní profil (volá i mobil)
		r.Get("/users/{id}", h.Users.Get)
		r.Put("/users/{id}/appearance", h.Users.UpdateAppearance)
		r.Put("/users/{id}/language", h.Users.UpdateLanguage)

		// Jen z webového přihlášení: token zařízení (mobil, televize) nesmí
		// měnit přihlašovací údaje, vydávat další tokeny ani schvalovat
		// párování – uniklý token by se jinak sám prodlužoval nebo převzal účet.
		r.With(middleware.RequireWebSessionCode("auth.mobile_token_refresh",
			"mobilní token nelze obnovit mobilním tokenem – přihlaste se znovu")).
			Post("/auth/mobile-token", h.Auth.MobileToken)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireWebSession)

			// Schválení párování televize ze stránky /link
			r.Get("/auth/device/{code}", h.Auth.DeviceInfo)
			r.Post("/auth/device/approve", h.Auth.DeviceApprove)

			r.Put("/users/{id}", h.Users.Update)
			r.Put("/users/{id}/password", h.Users.ChangePassword)
			// Přihlášená zařízení a odhlášení všude
			r.Get("/users/{id}/devices", h.Users.ListDevices)
			r.Delete("/users/{id}/devices/{sid}", h.Users.RevokeDevice)
			r.Post("/users/{id}/logout-all", h.Users.LogoutAll)
		})

		// Uživatelé a společná administrace - pouze admin
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole(model.RoleAdmin))
			r.Get("/users", h.Users.List)
			r.Delete("/users/{id}", h.Users.Delete)
			r.Put("/users/{id}/role", h.Users.SetRole)

			r.Post("/admin/users", h.Admin.CreateUser)
			r.Get("/admin/settings/registration", h.Admin.RegistrationSettings)
			r.Put("/admin/settings/registration", h.Admin.SetRegistrationSettings)
			r.Get("/admin/audit", h.Admin.Audit)

			// Pluginy metadat – společné všem produktům
			r.Get("/admin/plugins", h.Plugins.List)
			r.Post("/admin/plugins/reload", h.Plugins.Reload)
			r.Put("/admin/plugins/{name}/config", h.Plugins.SetConfig)
		})
	})
}
