package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
)

func RecoveryMiddleware(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				// Panic values and request dumps may contain credentials or PII.
				log.Error().Str("request_id", common.RequestIDFromContext(c.Request.Context())).Msg("http handler panic")
				if !c.Writer.Written() {
					dto.ErrorResponse(c, http.StatusInternalServerError, "Internal server error")
				} else {
					c.Abort()
				}
			}
		}()
		c.Next()
	}
}
