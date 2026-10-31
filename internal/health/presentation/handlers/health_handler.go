// Package handlers exposes the process liveness service over HTTP.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	"sea-codex-server/internal/health/services"
)

func Check(c *gin.Context) {
	service := services.HealthService()
	if service == nil {
		dto.ErrorResponse(c, http.StatusServiceUnavailable, "Liveness service unavailable")
		return
	}
	result, err := service.Check(c.Request.Context())
	if err != nil {
		status, message := common.HTTPError(common.ErrUnavailable)
		dto.ErrorResponse(c, status, message)
		return
	}
	dto.SuccessResponse(c, http.StatusOK, "Service is running", result)
}
