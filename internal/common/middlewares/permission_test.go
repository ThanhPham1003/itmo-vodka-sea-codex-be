package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	"sea-codex-server/internal/common/middlewares"
)

func TestPermissionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, role, permission string
		granted                []string
		noActor, noCompany     bool
		status                 int
	}{
		{name: "granted", permission: "dossier:read", granted: []string{"dossier:read"}, status: 200},
		{name: "denied", permission: "dossier:review", granted: []string{"dossier:read"}, status: 403},
		{name: "missing identity", permission: "dossier:read", noActor: true, status: 401},
		{name: "missing tenant", permission: "dossier:read", noCompany: true, status: 401},
		{name: "admin default PII", role: "company_admin", permission: common.PermissionPIIView, status: 200},
		{name: "admin default dashboard", role: "company_admin", permission: common.PermissionDashboardRead, status: 200},
		{name: "admin has no blanket bypass", role: "company_admin", permission: "unknown:permission", status: 403},
		{name: "platform admin has no tenant PII bypass", role: "platform_admin", permission: common.PermissionPIIView, status: 403},
		{name: "empty required permission", permission: "", granted: []string{""}, status: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("is_admin", true) // Untrusted Gin keys must not confer permissions.
				if !tc.noActor {
					actor := common.Actor{UserID: uuid.New(), CompanyID: uuid.New(), Role: tc.role, Permissions: tc.granted}
					if tc.noCompany {
						actor.CompanyID = uuid.Nil
					}
					c.Request = c.Request.WithContext(common.WithActor(c.Request.Context(), actor))
				}
				c.Next()
			})
			router.GET("/protected", middlewares.RequirePermission(tc.permission), func(c *gin.Context) { dto.SuccessResponse(c, 200, "ok", nil) })
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/protected", nil))
			assert.Equal(t, tc.status, w.Code)
		})
	}
}
