package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	"sea-codex-server/pkg/token"
)

func AuthMiddleware(app *common.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Request.Header.Values("Authorization")
		if len(headers) != 1 {
			dto.ErrorResponse(c, http.StatusUnauthorized, "Authentication required")
			return
		}
		parts := strings.Fields(headers[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			dto.ErrorResponse(c, http.StatusUnauthorized, "Invalid authorization header")
			return
		}
		if app == nil || app.TokenManager == nil {
			dto.ErrorResponse(c, http.StatusServiceUnavailable, "Authentication temporarily unavailable")
			return
		}
		claims, err := app.TokenManager.VerifyToken(c.Request.Context(), parts[1])
		if err != nil {
			if errors.Is(err, token.ErrInvalidToken) {
				dto.ErrorResponse(c, http.StatusUnauthorized, "Invalid or expired token")
			} else {
				dto.ErrorResponse(c, http.StatusServiceUnavailable, "Authentication temporarily unavailable")
			}
			return
		}
		if app.IdentityResolver == nil {
			dto.ErrorResponse(c, http.StatusServiceUnavailable, "Authentication temporarily unavailable")
			return
		}
		actor, err := app.IdentityResolver.ResolveIdentity(c.Request.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, common.ErrUnauthenticated) || errors.Is(err, common.ErrNotFound) {
				dto.ErrorResponse(c, http.StatusUnauthorized, "Authentication required")
			} else {
				app.Logger.Error().Str("request_id", common.RequestIDFromContext(c.Request.Context())).Msg("identity resolution failed")
				dto.ErrorResponse(c, http.StatusServiceUnavailable, "Authentication temporarily unavailable")
			}
			return
		}
		if actor.UserID == uuid.Nil || actor.UserID != claims.UserID || actor.CompanyID == uuid.Nil {
			dto.ErrorResponse(c, http.StatusUnauthorized, "Authentication required")
			return
		}
		c.Request = c.Request.WithContext(common.WithActor(c.Request.Context(), actor))
		c.Set("user_id", actor.UserID.String())
		c.Set("company_id", actor.CompanyID.String())
		c.Set("user_email", actor.Email)
		c.Set("user_role", actor.Role)
		c.Set("is_admin", actor.Role == common.RoleCompanyAdmin)
		c.Set("claims", claims)
		c.Next()
	}
}
