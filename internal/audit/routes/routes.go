package routes

import (
	"sea-codex-server/internal/audit/routes/v1"
	"sea-codex-server/internal/audit/services"
	"sea-codex-server/internal/common"
)

func RegisterRoutes(app *common.App, s services.IAuditService) { v1.RegisterRoutes(app, s) }
