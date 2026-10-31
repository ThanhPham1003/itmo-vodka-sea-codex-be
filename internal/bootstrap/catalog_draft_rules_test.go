package bootstrap_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ad "sea-codex-server/internal/auth/dto"
	"sea-codex-server/internal/bootstrap"
	dd "sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
)

func TestDraftRuleSuggestionHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": `{"rules":[]}`}}}})
	}))
	defer provider.Close()
	app := newTestApp(t)
	app.Config.Processing.BaseURL = provider.URL
	app.Config.Processing.APIKey = "fixture-key"
	app.Config.Processing.Model = "fixture-model"
	router, err := bootstrap.NewRouter(app)
	require.NoError(t, err)
	_, token := signupAndLogin(t, router, "draft-admin")
	doc := data[models.DocumentType](t, request(t, router, "POST", "/api/v1/document-types", token, dd.DocumentTypeRequest{DocumentDefinition: models.DocumentDefinition{
		Name: "Identity", Fields: []models.FieldSchema{{Label: "Name", Type: "text"}},
	}}), 201)
	member := ad.CreateUserRequest{Username: "draft-member", Email: "member@example.test", Password: "a-valid-test-password", FullName: "Member", Role: "manager"}
	require.Equal(t, 201, request(t, router, "POST", "/api/v1/auth/users", token, member).Code)
	login := data[ad.LoginResponse](t, request(t, router, "POST", "/api/v1/auth/login", "", ad.LoginRequest{UsernameOrEmail: member.Username, Password: member.Password}), 200)
	in := dd.DraftRuleSuggestionRequest{Description: "Compare names", Definition: models.DossierDefinition{Name: "New type", Documents: []models.DocumentRequirement{{DocumentType: doc.Code, SchemaVersion: 1, Required: true}}}}
	const path = "/api/v1/dossier-types/comparison-rules/suggestions"
	for _, tc := range []struct {
		name, token string
		body        any
		status      int
	}{
		{"unauthenticated", "", in, 401},
		{"non admin", login.AccessToken, in, 403},
		{"missing description", token, map[string]any{"definition": in.Definition}, 400},
		{"unknown property", token, map[string]any{"description": "Compare", "definition": in.Definition, "save": true}, 400},
		{"empty configuration", token, dd.DraftRuleSuggestionRequest{Description: "Compare"}, 400},
		{"draft", token, in, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := request(t, router, "POST", path, tc.token, tc.body)
			require.Equal(t, tc.status, res.Code, res.Body.String())
			assert.NotEmpty(t, res.Header().Get("X-Request-ID"))
			if tc.status == 200 {
				out := data[struct {
					Draft bool                    `json:"draft"`
					Rules []models.ComparisonRule `json:"rules"`
				}](t, res, 200)
				assert.True(t, out.Draft)
				assert.Empty(t, out.Rules)
			}
		})
	}
	var count int64
	require.NoError(t, app.DB.Model(&models.DossierType{}).Count(&count).Error)
	assert.Zero(t, count)
}
