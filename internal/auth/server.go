// Package auth owns company registration, users, authentication and membership.
package auth

import (
	"sea-codex-server/internal/auth/logic"
	"sea-codex-server/internal/auth/repositories"
	"sea-codex-server/internal/auth/routes"
	"sea-codex-server/internal/auth/services"
	"sea-codex-server/internal/common"
)

// RegisterServices is the module composition point for future concrete services.
func RegisterServices(app *common.App) {
	s := logic.NewAuthLogic(app.DB, repositories.NewUserRepository(app.DB), repositories.NewCompanyRepository(), app.TokenManager, app.AuditWriter)
	services.RegisterAuthService(s)
	app.IdentityResolver = s
}

// NewServer will mount approved business routes on app.Router.
func NewServer(app *common.App) { routes.RegisterRoutes(app, services.AuthService()) }
