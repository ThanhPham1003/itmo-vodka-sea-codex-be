// Package v1 defines health transport routes. Liveness is unversioned by design.
package v1

import (
	"github.com/gin-gonic/gin"

	"sea-codex-server/internal/health/presentation/handlers"
)

func RegisterRoutes(router *gin.Engine) {
	router.GET("/health", handlers.Check)
}
