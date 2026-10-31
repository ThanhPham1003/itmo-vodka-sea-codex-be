package v1

import (
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/middlewares"
	"sea-codex-server/internal/dossier/presentation/handlers"
	"sea-codex-server/internal/dossier/services"
)

func RegisterRoutes(app *common.App, catalog services.ICatalogService, dossiers services.IDossierService) {
	c := handlers.NewCatalogHandler(catalog)
	h := handlers.NewDossierHandler(dossiers)
	g := app.Router.Group("/api/v1", middlewares.AuthMiddleware(app))
	g.GET("/document-types", c.ListDocuments)
	g.POST("/document-types", c.PutDocument)
	g.POST("/document-types/suggestions", c.SuggestDocument)
	g.GET("/document-types/:code", c.GetDocument)
	g.PUT("/document-types/:code", c.PutDocument)
	g.DELETE("/document-types/:code", c.ArchiveDocument)
	g.GET("/dossier-types", c.ListDossiers)
	g.POST("/dossier-types", c.PutDossier)
	g.POST("/dossier-types/suggestions", c.SuggestDossier)
	g.GET("/dossier-types/:code", c.GetDossier)
	g.PUT("/dossier-types/:code", c.PutDossier)
	g.DELETE("/dossier-types/:code", c.ArchiveDossier)
	g.POST("/dossier-types/:code/comparison-rules/suggestions", c.SuggestRules)
	g.POST("/dossiers", h.Create)
	g.GET("/dossiers", h.List(false))
	g.GET("/dossiers/review-queue", h.List(true))
	g.GET("/dossiers/:id", h.Detail)
	g.POST("/dossiers/:id/files", h.Upload(app))
	g.GET("/dossiers/:id/files/:file_id/content", h.Content)
	g.POST("/dossiers/:id/files/:file_id/retry", h.Retry)
	g.PATCH("/dossiers/:id/files/:file_id/classification", h.Classify)
	g.PATCH("/dossiers/:id/assignment", h.Assign)
	g.POST("/dossiers/:id/requirements", h.AddRequirement)
	g.DELETE("/dossiers/:id/requirements/:requirement_id", h.WaiveRequirement)
	g.PATCH("/dossiers/:id/fields/:field_id", h.Correct)
	g.POST("/dossiers/:id/issues/:issue_id/acknowledge", h.Acknowledge)
	g.POST("/dossiers/:id/decisions", h.Review)
	g.POST("/dossiers/:id/reopen", h.Reopen)
	g.POST("/dossiers/:id/pii/reveal", h.Reveal)
	g.GET("/dashboard/statuses", h.Dashboard)
}
