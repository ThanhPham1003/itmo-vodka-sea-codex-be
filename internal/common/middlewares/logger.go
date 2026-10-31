package middlewares

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"sea-codex-server/internal/common"
)

func LoggerMiddleware(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		// Use the route template, never the raw URL, query, headers or body.
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		log.Info().Str("request_id", common.RequestIDFromContext(c.Request.Context())).
			Str("method", c.Request.Method).Str("route", path).
			Int("status", c.Writer.Status()).Dur("duration", time.Since(start)).Msg("http request")
	}
}
