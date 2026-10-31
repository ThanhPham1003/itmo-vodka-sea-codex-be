package bootstrap_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/internal/bootstrap"
	"sea-codex-server/internal/common"
	"sea-codex-server/pkg/config"
)

func TestRouterWiring(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app := newTestApp(t)
	router, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	assert.Same(t, router, app.Router)
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/health", 200},
		{http.MethodPost, "/health", 405},
		{http.MethodPost, "/api/v1/auth/signup", 400},
		{http.MethodGet, "/api/v1/auth/me", 401},
		{http.MethodGet, "/api/v1/dossiers", 401},
		{http.MethodGet, "/api/v1/audit-logs", 401},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			assert.Equal(t, tc.status, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
			assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
		})
	}
	_, err = bootstrap.NewRouter(app)
	assert.ErrorIs(t, err, common.ErrInvalidArgument)
	_, err = bootstrap.NewRouter(nil)
	assert.ErrorIs(t, err, common.ErrInvalidArgument)
}

func TestAppRejectsMissingConfiguration(t *testing.T) {
	_, err := bootstrap.NewApp(context.Background(), nil, io.Discard)
	assert.ErrorIs(t, err, config.ErrInvalidConfiguration)
}
