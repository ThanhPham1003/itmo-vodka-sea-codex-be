// Package audit owns tenant-scoped append-only audit events and authorized queries.
package audit

import (
	"sea-codex-server/internal/audit/logic"
	"sea-codex-server/internal/audit/repositories"
	"sea-codex-server/internal/audit/routes"
	"sea-codex-server/internal/audit/services"
	"sea-codex-server/internal/common"
)

// RegisterServices is the module composition point for future concrete services.
func RegisterServices(app *common.App) {
	s := logic.NewAuditLogic(repositories.NewAuditRepository(app.DB))
	services.RegisterAuditService(s)
	app.AuditWriter = s
}

// NewServer will mount approved business routes on app.Router.
func NewServer(app *common.App) { routes.RegisterRoutes(app, services.AuditService()) }
