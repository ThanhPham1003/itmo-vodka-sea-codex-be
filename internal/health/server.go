// Package health wires the process liveness module.
package health

import (
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/health/logic"
	"sea-codex-server/internal/health/routes"
	"sea-codex-server/internal/health/services"
)

func RegisterServices(_ *common.App) {
	services.RegisterHealthService(logic.NewHealthLogic())
}

func NewServer(app *common.App) { routes.RegisterRoutes(app.Router) }
