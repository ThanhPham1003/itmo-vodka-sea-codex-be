package v1

import (
	"sea-codex-server/internal/audit/presentation/handlers"
	"sea-codex-server/internal/audit/services"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/middlewares"
)

func RegisterRoutes(app *common.App, s services.IAuditService) {
	app.Router.Group("/api/v1", middlewares.AuthMiddleware(app)).GET("/audit-logs", middlewares.RequirePermission("audit:read"), handlers.List(s))
}
