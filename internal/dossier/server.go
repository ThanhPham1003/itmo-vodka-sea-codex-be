// Package dossier owns catalogs, dossiers, intake, extraction, reconciliation, review and dashboard.
package dossier

import (
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/logic"
	"sea-codex-server/internal/dossier/repositories"
	"sea-codex-server/internal/dossier/routes"
	"sea-codex-server/internal/dossier/services"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/storage"
)

// RegisterServices is the module composition point for future concrete services.
func RegisterServices(app *common.App) {
	r := repositories.NewDossierRepository(app.DB)
	client := ai.NewClient(app.Config.Processing)
	catalog := logic.NewCatalogLogic(r, app.AuditWriter, client)
	dossiers := logic.NewDossierLogic(r, app.AuditWriter, app.IdentityResolver, storage.NewLocal(app.Config.Uploads.Directory), catalog, client, app.Config, app.Logger)
	services.RegisterCatalogService(catalog)
	services.RegisterDossierService(dossiers)
	app.Workers = append(app.Workers, dossiers)
}

// NewServer will mount approved business routes on app.Router.
func NewServer(app *common.App) {
	routes.RegisterRoutes(app, services.CatalogService(), services.DossierService())
}
