package routes

import (
	"sea-codex-server/internal/assistant/routes/v1"
	"sea-codex-server/internal/assistant/services"
	"sea-codex-server/internal/common"
)

func RegisterRoutes(app *common.App, s services.IAssistantService) { v1.RegisterRoutes(app, s) }
