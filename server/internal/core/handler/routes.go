package handler

import (
	"github.com/go-chi/chi/v5"

	"mediater/internal/core/middleware"
	"mediater/internal/core/model"
	"mediater/internal/core/service"
)

// Handlers sdružuje handlery jádra pro registraci rout.
type Handlers struct {
	Auth  *AuthHandler
	Users *UserHandler
	Admin *AdminHandler
}

// RegisterRoutes zaregistruje API jádra na router /api/v1: přihlášení,
// tokeny, profil, správu uživatelů a společnou administraci.
//
// Administrace se registruje plnými cestami (ne r.Route("/admin")) – chi by
// jinak druhému produktu nedovolil přidat vlastní /admin/... cesty.
func RegisterRoutes(r chi.Router, auth *service.AuthService, h Handlers) {
	// --- bez přihlášení ---
	r.Get("/auth/config", h.Auth.Config) // je registrace zapnutá?
	r.Post("/auth/register", h.Auth.Register)
	r.Post("/auth/login", h.Auth.Login)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Authenticate(auth))

		// Token pro přehrávače (<audio>, <video> neumí poslat hlavičku)
		r.Get("/auth/stream-token", h.Auth.StreamToken)
		// Dlouhodobý token pro mobilní aplikaci (offline přehrávač)
		r.Post("/auth/mobile-token", h.Auth.MobileToken)

		// Uživatelé - vlastní profil
		r.Get("/users/{id}", h.Users.Get)
		r.Put("/users/{id}", h.Users.Update)
		r.Put("/users/{id}/password", h.Users.ChangePassword)
		r.Put("/users/{id}/appearance", h.Users.UpdateAppearance)
		r.Put("/users/{id}/language", h.Users.UpdateLanguage)

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
		})
	})
}
