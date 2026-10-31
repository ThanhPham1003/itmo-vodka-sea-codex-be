package routes

import (
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/routes/v1"
	"sea-codex-server/internal/dossier/services"
)

func RegisterRoutes(app *common.App, c services.ICatalogService, s services.IDossierService) {
	v1.RegisterRoutes(app, c, s)
}
