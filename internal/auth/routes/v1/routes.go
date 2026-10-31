package v1

import (
	"sea-codex-server/internal/auth/presentation/handlers"
	"sea-codex-server/internal/auth/services"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/middlewares"
	"time"
)

func RegisterRoutes(app *common.App, s services.IAuthService) {
	h := handlers.NewAuthHandler(s)
	limit := 20
	window := time.Minute
	if app.Config != nil {
		limit = app.Config.Auth.RequestLimit
		window = app.Config.Auth.RateWindow
	}
	limiter := middlewares.NewRateLimiter(limit, window)
	g := app.Router.Group("/api/v1/auth")
	g.POST("/signup", limiter.Middleware(), h.Signup)
	g.POST("/login", limiter.Middleware(), h.Login)
	g.Use(middlewares.AuthMiddleware(app))
	g.GET("/me", h.Me)
	g.GET("/users", h.ListUsers)
	g.POST("/users", h.CreateUser)
	g.PATCH("/users/:id/access", h.UpdateAccess)
}
