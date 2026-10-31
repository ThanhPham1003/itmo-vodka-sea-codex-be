package bootstrap

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"sea-codex-server/internal/audit"
	"sea-codex-server/internal/auth"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	"sea-codex-server/internal/common/middlewares"
	"sea-codex-server/internal/dossier"
	"sea-codex-server/internal/health"
)

func NewRouter(app *common.App) (*gin.Engine, error) {
	if app == nil || app.DB == nil || app.Config == nil || app.TokenManager == nil || app.Router != nil {
		return nil, fmt.Errorf("%w: application missing or router already initialized", common.ErrInvalidArgument)
	}
	router := gin.New()
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	router.HandleMethodNotAllowed = true
	router.Use(middlewares.RequestIDMiddleware(), middlewares.LoggerMiddleware(app.Logger), middlewares.RecoveryMiddleware(app.Logger))
	router.NoRoute(func(c *gin.Context) { dto.ErrorResponse(c, http.StatusNotFound, "Resource not found") })
	router.NoMethod(func(c *gin.Context) { dto.ErrorResponse(c, http.StatusMethodNotAllowed, "Method not allowed") })
	app.Router = router

	health.RegisterServices(app)
	audit.RegisterServices(app)
	auth.RegisterServices(app)
	dossier.RegisterServices(app)
	health.NewServer(app)
	auth.NewServer(app)
	audit.NewServer(app)
	dossier.NewServer(app)
	return router, nil
}
