package v1

import (
	"sea-codex-server/internal/assistant/presentation/handlers"
	"sea-codex-server/internal/assistant/services"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/middlewares"
)

func RegisterRoutes(app *common.App, s services.IAssistantService) {
	h := handlers.NewAssistantHandler(s)
	g := app.Router.Group("/api/v1/assistant", middlewares.AuthMiddleware(app))
	g.GET("/context", h.Context)
	g.GET("/articles/:id", h.Article)
	g.POST("/chat", h.Chat)
}
