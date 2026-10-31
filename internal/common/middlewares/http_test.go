package middlewares_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/internal/common/dto"
	"sea-codex-server/internal/common/middlewares"
)

func TestHTTPMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path, requestID string
		status                int
	}{
		{"normal", "/ok", uuid.NewString(), 200},
		{"replace invalid request ID", "/ok", "not-a-request-id", 200},
		{"recover panic", "/panic", "", 500},
		{"unmatched path not logged", "/private-secret-path", "", 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			log := zerolog.New(&logs)
			router := gin.New()
			router.Use(middlewares.RequestIDMiddleware(), middlewares.LoggerMiddleware(log), middlewares.RecoveryMiddleware(log))
			router.GET("/ok", func(c *gin.Context) { dto.SuccessResponse(c, 200, "ok", nil) })
			router.GET("/panic", func(c *gin.Context) { panic("panic-secret-do-not-log") })
			req := httptest.NewRequest(http.MethodGet, tc.path+"?password=query-secret", strings.NewReader(`{"password":"body-secret"}`))
			req.Header.Set("Authorization", "Bearer token-secret")
			req.Header.Set("Cookie", "session=cookie-secret")
			req.Header.Set(middlewares.RequestIDHeader, tc.requestID)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tc.status, w.Code)
			_, err := uuid.Parse(w.Header().Get(middlewares.RequestIDHeader))
			require.NoError(t, err)
			if tc.name == "normal" {
				assert.Equal(t, tc.requestID, w.Header().Get(middlewares.RequestIDHeader))
			}
			for _, secret := range []string{"query-secret", "body-secret", "token-secret", "cookie-secret", "panic-secret-do-not-log", "private-secret-path"} {
				assert.NotContains(t, logs.String(), secret)
				assert.NotContains(t, w.Body.String(), secret)
			}
			assert.Contains(t, logs.String(), w.Header().Get(middlewares.RequestIDHeader))
			if tc.status == 500 {
				assert.JSONEq(t, `{"code":500,"message":"Internal server error","data":null}`, w.Body.String())
			}
		})
	}
}
