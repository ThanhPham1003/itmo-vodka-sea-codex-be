// Package routes mounts module routes on the shared engine.
package routes

import (
	"github.com/gin-gonic/gin"

	v1 "sea-codex-server/internal/health/routes/v1"
)

func RegisterRoutes(router *gin.Engine) { v1.RegisterRoutes(router) }
