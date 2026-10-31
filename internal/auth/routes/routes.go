package routes

import (
	"sea-codex-server/internal/auth/routes/v1"
	"sea-codex-server/internal/auth/services"
	"sea-codex-server/internal/common"
)

func RegisterRoutes(app *common.App, s services.IAuthService) { v1.RegisterRoutes(app, s) }
