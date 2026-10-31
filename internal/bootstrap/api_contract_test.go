package bootstrap_test

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/internal/bootstrap"
)

func TestOpenAPIContractMatchesRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, err := bootstrap.NewRouter(newTestApp(t))
	require.NoError(t, err)
	raw, err := os.ReadFile("../../docs/api/api_spec.json")
	require.NoError(t, err)
	var spec struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	require.NoError(t, json.Unmarshal(raw, &spec))
	public := map[string]bool{"/health": true, "/api/v1/auth/signup": true, "/api/v1/auth/login": true}
	ids := map[string]bool{}
	count := 0
	var walk func(any)
	var root any
	require.NoError(t, json.Unmarshal(raw, &root))
	walk = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			if ref, ok := v["$ref"].(string); ok {
				require.True(t, strings.HasPrefix(ref, "#/components/schemas/"))
				_, exists := spec.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
				assert.True(t, exists, "unresolved reference: %s", ref)
			}
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(root)
	params := regexp.MustCompile(`:([a-z_]+)`)
	for _, route := range router.Routes() {
		path := params.ReplaceAllString(route.Path, "{$1}")
		operation, ok := spec.Paths[path][strings.ToLower(route.Method)]
		require.True(t, ok, "undocumented route %s %s", route.Method, path)
		var op struct {
			OperationID string `json:"operationId"`
			Security    []any  `json:"security"`
		}
		require.NoError(t, json.Unmarshal(operation, &op))
		assert.NotEmpty(t, op.OperationID)
		assert.False(t, ids[op.OperationID], "duplicate operation ID")
		ids[op.OperationID] = true
		if public[route.Path] {
			assert.Empty(t, op.Security)
		} else {
			assert.NotEmpty(t, op.Security)
			path := params.ReplaceAllString(route.Path, "00000000-0000-4000-8000-000000000001")
			assert.Equal(t, 401, request(t, router, route.Method, path, "", nil).Code, "unprotected route %s %s", route.Method, path)
		}
		count++
	}
	documented := 0
	for _, path := range spec.Paths {
		documented += len(path)
	}
	assert.Equal(t, count, documented, "OpenAPI declares a route not implemented by Gin")
}
