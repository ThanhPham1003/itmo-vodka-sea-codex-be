package middlewares_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	paseto "aidanwoods.dev/go-paseto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/internal/common"
	"sea-codex-server/internal/common/dto"
	"sea-codex-server/internal/common/middlewares"
	"sea-codex-server/pkg/token"
)

type resolverFunc func(context.Context, uuid.UUID) (common.Actor, error)

func (f resolverFunc) ResolveIdentity(ctx context.Context, userID uuid.UUID) (common.Actor, error) {
	return f(ctx, userID)
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := paseto.NewV4AsymmetricSecretKey()
	manager, err := token.NewManager(key.ExportHex(), key.Public().ExportHex(), time.Hour)
	require.NoError(t, err)
	userID, companyID := uuid.New(), uuid.New()
	encoded, _, err := manager.CreateToken(context.Background(), userID)
	require.NoError(t, err)
	actor := common.Actor{UserID: userID, CompanyID: companyID, Email: "current@example.test", Role: "broker", Permissions: []string{"dossier:read"}}
	for _, tc := range []struct {
		name    string
		headers []string
		change  func(*common.App)
		status  int
	}{
		{name: "missing header", status: 401},
		{name: "wrong scheme", headers: []string{"Basic " + encoded}, status: 401},
		{name: "missing value", headers: []string{"Bearer"}, status: 401},
		{name: "bad token", headers: []string{"Bearer invalid"}, status: 401},
		{name: "multiple headers", headers: []string{"Bearer " + encoded, "Bearer " + encoded}, status: 401},
		{name: "no manager", headers: []string{"Bearer " + encoded}, change: func(app *common.App) { app.TokenManager = nil }, status: 503},
		{name: "no resolver", headers: []string{"Bearer " + encoded}, change: func(app *common.App) { app.IdentityResolver = nil }, status: 503},
		{name: "inactive user", headers: []string{"Bearer " + encoded}, change: func(app *common.App) {
			app.IdentityResolver = resolverFunc(func(context.Context, uuid.UUID) (common.Actor, error) {
				return common.Actor{}, fmt.Errorf("inactive: %w", common.ErrUnauthenticated)
			})
		}, status: 401},
		{name: "deleted user", headers: []string{"Bearer " + encoded}, change: func(app *common.App) {
			app.IdentityResolver = resolverFunc(func(context.Context, uuid.UUID) (common.Actor, error) { return common.Actor{}, common.ErrNotFound })
		}, status: 401},
		{name: "DB failure", headers: []string{"Bearer " + encoded}, change: func(app *common.App) {
			app.IdentityResolver = resolverFunc(func(context.Context, uuid.UUID) (common.Actor, error) {
				return common.Actor{}, errors.New("DB secret must not leak")
			})
		}, status: 503},
		{name: "company absent", headers: []string{"Bearer " + encoded}, change: func(app *common.App) {
			app.IdentityResolver = resolverFunc(func(context.Context, uuid.UUID) (common.Actor, error) {
				copy := actor
				copy.CompanyID = uuid.Nil
				return copy, nil
			})
		}, status: 401},
		{name: "subject mismatch", headers: []string{"Bearer " + encoded}, change: func(app *common.App) {
			app.IdentityResolver = resolverFunc(func(context.Context, uuid.UUID) (common.Actor, error) {
				copy := actor
				copy.UserID = uuid.New()
				return copy, nil
			})
		}, status: 401},
		{name: "current identity", headers: []string{"Bearer " + encoded}, status: 200},
		{name: "case insensitive scheme", headers: []string{"bearer " + encoded}, status: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &common.App{TokenManager: manager, Logger: zerolog.Nop(), IdentityResolver: resolverFunc(func(ctx context.Context, got uuid.UUID) (common.Actor, error) {
				assert.Equal(t, userID, got)
				return actor, nil
			})}
			if tc.change != nil {
				tc.change(app)
			}
			router := gin.New()
			reached := false
			router.GET("/protected", middlewares.AuthMiddleware(app), func(c *gin.Context) {
				reached = true
				current, ok := common.ActorFromContext(c.Request.Context())
				require.True(t, ok)
				assert.Equal(t, actor, current)
				assert.Equal(t, companyID.String(), c.GetString("company_id"))
				assert.Equal(t, userID.String(), c.GetString("user_id"))
				assert.Equal(t, actor.Email, c.GetString("user_email"))
				assert.Equal(t, "broker", c.GetString("user_role"))
				assert.False(t, c.GetBool("is_admin"))
				_, exists := c.Get("claims")
				assert.True(t, exists)
				dto.SuccessResponse(c, 200, "ok", gin.H{"company_id": current.CompanyID})
			})
			req := httptest.NewRequest(http.MethodGet, "/protected?company_id="+uuid.NewString(), strings.NewReader(`{"role":"company_admin"}`))
			req.Header.Set("X-Company-ID", uuid.NewString())
			for _, header := range tc.headers {
				req.Header.Add("Authorization", header)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tc.status, w.Code)
			assert.Equal(t, tc.status == 200, reached)
			assert.NotContains(t, w.Body.String(), "DB secret")
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, float64(tc.status), body["code"])
		})
	}
}

func TestExpiredTokenRejectedByMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := paseto.NewV4AsymmetricSecretKey()
	manager, err := token.NewManager(key.ExportHex(), key.Public().ExportHex(), time.Hour)
	require.NoError(t, err)
	tok := paseto.NewToken()
	tok.SetIssuedAt(time.Now().Add(-time.Hour))
	tok.SetNotBefore(time.Now().Add(-time.Hour))
	tok.SetExpiration(time.Now().Add(-time.Minute))
	tok.SetIssuer("sea-codex-server")
	tok.SetAudience("sea-codex-api")
	app := &common.App{TokenManager: manager, Logger: zerolog.Nop(), IdentityResolver: resolverFunc(func(context.Context, uuid.UUID) (common.Actor, error) {
		t.Error("expired token must not call resolver")
		return common.Actor{}, nil
	})}
	router := gin.New()
	router.GET("/protected", middlewares.AuthMiddleware(app), func(c *gin.Context) { t.Error("handler reached") })
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok.V4Sign(key, nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
