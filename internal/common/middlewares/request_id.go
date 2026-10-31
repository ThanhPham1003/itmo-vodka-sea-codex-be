// Package middlewares implements shared transport concerns.
package middlewares

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sea-codex-server/internal/common"
)

const RequestIDHeader = "X-Request-ID"

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.GetHeader(RequestIDHeader))
		if err != nil || id == uuid.Nil {
			id = uuid.New()
		}
		requestID := id.String()
		c.Request = c.Request.WithContext(common.WithRequestID(c.Request.Context(), requestID))
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}
