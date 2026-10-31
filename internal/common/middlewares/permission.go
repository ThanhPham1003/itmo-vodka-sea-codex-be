package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
)

// RequirePermission must follow AuthMiddleware. Object-level tenant/assignment
// checks remain mandatory in the application service and repository.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := common.ActorFromContext(c.Request.Context())
		if !ok {
			dto.ErrorResponse(c, http.StatusUnauthorized, "Authentication required")
			return
		}
		if permission == "" || !actor.HasPermission(permission) {
			dto.ErrorResponse(c, http.StatusForbidden, "Permission denied")
			return
		}
		c.Next()
	}
}
