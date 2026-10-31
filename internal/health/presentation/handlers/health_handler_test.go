package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/internal/health/logic"
	"sea-codex-server/internal/health/routes"
	"sea-codex-server/internal/health/services"
)

func TestHealthHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if services.HealthService() == nil {
		require.True(t, services.RegisterHealthService(logic.NewHealthLogic()))
	}
	for _, tc := range []struct {
		name, method, path string
		cancelled          bool
		status             int
	}{
		{"running", http.MethodGet, "/health", false, 200},
		{"cancelled", http.MethodGet, "/health", true, 503},
		{"wrong method", http.MethodPost, "/health", false, 405},
		{"unknown route", http.MethodGet, "/missing", false, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.HandleMethodNotAllowed = true
			routes.RegisterRoutes(router)
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.cancelled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tc.status, w.Code)
			if tc.status == 200 {
				assert.JSONEq(t, `{"code":200,"message":"Service is running","data":{"status":"ok"}}`, w.Body.String())
			}
		})
	}
}
