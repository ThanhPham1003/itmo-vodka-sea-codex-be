package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/config"
)

func dossierSuggestionFixture(t *testing.T) (*repositories.DossierRepository, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, db.AutoMigrate(&models.DocumentType{}, &models.DocumentTypeVersion{}, &models.DossierType{}, &models.DossierTypeVersion{}))
	company := uuid.New()
	for _, entry := range []struct {
		code, name string
		company    uuid.UUID
		active     bool
	}{
		{"doc_selected", "Selected document", company, true},
		{"doc_unselected", "private-unselected", company, true},
		{"doc_inactive", "private-inactive", company, false},
		{"doc_foreign", "private-foreign", uuid.New(), true},
	} {
		doc := models.DocumentType{CompanyID: entry.company, Code: entry.code, Active: entry.active, CurrentVersion: 3, Definition: models.DocumentDefinition{Name: entry.name, Fields: []models.FieldSchema{{Key: "field_1", Label: "Field", Type: "text"}}}}
		require.NoError(t, db.Create(&doc).Error)
		require.NoError(t, db.Create(&models.DocumentTypeVersion{CompanyID: entry.company, DocumentTypeID: doc.ID, Version: 3, Definition: doc.Definition}).Error)
	}
	ctx := common.WithRequestID(common.WithActor(context.Background(), common.Actor{CompanyID: company, UserID: uuid.New(), Role: common.RoleCompanyAdmin}), "dossier-request-id")
	return repositories.NewDossierRepository(db), ctx
}

type dossierSuggestionAudit struct{ events []common.AuditEvent }

func (s *dossierSuggestionAudit) Write(_ context.Context, _ *gorm.DB, event common.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

func TestDossierSuggestionPinsCurrentVersionAndRemainsSaveable(t *testing.T) {
	repo, ctx := dossierSuggestionFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages       json.RawMessage `json:"messages"`
			ResponseFormat struct {
				Type       string `json:"type"`
				JSONSchema struct {
					Strict bool `json:"strict"`
					Schema struct {
						Properties map[string]json.RawMessage `json:"properties"`
					} `json:"schema"`
				} `json:"json_schema"`
			} `json:"response_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		assert.Equal(t, "json_schema", request.ResponseFormat.Type)
		assert.True(t, request.ResponseFormat.JSONSchema.Strict)
		var documents struct {
			Items struct {
				AdditionalProperties bool `json:"additionalProperties"`
				Properties           map[string]struct {
					Enum []string `json:"enum"`
				} `json:"properties"`
			} `json:"items"`
		}
		assert.NoError(t, json.Unmarshal(request.ResponseFormat.JSONSchema.Schema.Properties["documents"], &documents))
		assert.Equal(t, []string{"doc_selected"}, documents.Items.Properties["document_type"].Enum)
		assert.False(t, documents.Items.AdditionalProperties)
		for _, field := range []string{"key", "schema_version", "position", "condition"} {
			assert.NotContains(t, documents.Items.Properties, field)
		}
		assert.NotContains(t, request.ResponseFormat.JSONSchema.Schema.Properties, "rules")
		for _, value := range []string{"private-foreign", "private-unselected", "private-inactive"} {
			assert.NotContains(t, string(request.Messages), value)
		}
		content := `{"name":"Draft dossier","description":"Example","documents":[{"document_type":"doc_selected","role":"","required":true,"conditional":false,"min_count":1,"trigger_reason":""}]}`
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": content}}}})
	}))
	defer server.Close()
	audit := &dossierSuggestionAudit{}
	client := ai.NewClient(config.ProcessingConfig{Provider: "openai-compatible", BaseURL: server.URL, APIKey: "test", Model: "test", Timeout: time.Second})
	l := NewCatalogLogic(repo, audit, client, zerolog.Nop())
	raw, err := l.SuggestDossier(ctx, "An example dossier", []string{"doc_selected"})
	require.NoError(t, err)
	var draft struct {
		Draft      bool                     `json:"draft"`
		Definition models.DossierDefinition `json:"definition"`
	}
	require.NoError(t, json.Unmarshal(raw, &draft))
	assert.True(t, draft.Draft)
	require.Len(t, draft.Definition.Documents, 1)
	assert.Empty(t, draft.Definition.Documents[0].Key)
	assert.Equal(t, 3, draft.Definition.Documents[0].SchemaVersion, "version 1 does not exist in this fixture")
	assert.NotNil(t, draft.Definition.Rules)
	assert.Empty(t, draft.Definition.Rules)
	require.Len(t, audit.events, 1)
	assert.Equal(t, "DOSSIER_CONFIG_SUGGESTED", audit.events[0].Action)
	var count int64
	require.NoError(t, repo.DB().Model(&models.DossierType{}).Count(&count).Error)
	assert.Zero(t, count, "suggestions remain unsaved")
	saved, err := l.PutDossierType(ctx, "", dto.DossierTypeRequest{DossierDefinition: draft.Definition})
	require.NoError(t, err)
	assert.NotEmpty(t, saved.Definition.Documents[0].Key)
	assert.Equal(t, 3, saved.Definition.Documents[0].SchemaVersion)
}

func TestDossierSuggestionValidationLogs(t *testing.T) {
	repo, ctx := dossierSuggestionFixture(t)
	for _, tc := range []struct {
		name, rule, field string
		index             int
		mutate            func(*dossierSuggestion)
	}{
		{"duplicate role", "duplicate_document_role", "documents.role", 1, func(s *dossierSuggestion) { s.Documents = append(s.Documents, s.Documents[0]) }},
		{"conditional reason", "missing_trigger_reason", "documents.trigger_reason", 0, func(s *dossierSuggestion) { s.Documents[0].Conditional = true }},
		{"unselected code", "document_type_not_selected", "documents.document_type", 0, func(s *dossierSuggestion) { s.Documents[0].DocumentType = "doc_unselected" }},
		{"foreign code", "document_type_not_selected", "documents.document_type", 0, func(s *dossierSuggestion) { s.Documents[0].DocumentType = "doc_foreign" }},
		{"min count", "invalid_min_count", "documents.min_count", 0, func(s *dossierSuggestion) { s.Documents[0].MinCount = 101 }},
		{"empty name", "invalid_name", "name", -1, func(s *dossierSuggestion) { s.Name = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			suggestion := dossierSuggestion{Name: "private-name", Description: "private-description", Documents: []dossierDocumentSuggestion{{DocumentType: "doc_selected", Required: true, MinCount: 1}}}
			tc.mutate(&suggestion)
			raw, err := json.Marshal(suggestion)
			require.NoError(t, err)
			var buf bytes.Buffer
			audit := &dossierSuggestionAudit{}
			l := NewCatalogLogic(repo, audit, suggestionAI{raw: raw}, zerolog.New(&buf))
			_, err = l.SuggestDossier(ctx, "private-prompt", []string{"doc_selected"})
			require.ErrorIs(t, err, common.ErrUnavailable)
			assert.Empty(t, audit.events)
			var log map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &log))
			assert.Equal(t, "validation", log["stage"])
			assert.Equal(t, tc.rule, log["validation_error"])
			assert.Equal(t, tc.field, log["json_field"])
			assert.Equal(t, "dossier-request-id", log["request_id"])
			if tc.index >= 0 {
				assert.Equal(t, float64(tc.index), log["item_index"])
			} else {
				assert.NotContains(t, log, "item_index")
			}
			for _, private := range []string{"private-name", "private-description", "private-prompt", "doc_foreign"} {
				assert.NotContains(t, buf.String(), private)
			}
		})
	}
}

func TestDossierValidationRejectsInventedIdentifiersAndVersions(t *testing.T) {
	repo, ctx := dossierSuggestionFixture(t)
	actor, _ := common.ActorFromContext(ctx)
	for _, tc := range []struct {
		name, rule  string
		requirement models.DocumentRequirement
	}{
		{"invented key", "unknown_identifier", models.DocumentRequirement{Key: "invented", DocumentType: "doc_selected", SchemaVersion: 3}},
		{"version one unavailable", "schema_version_unavailable", models.DocumentRequirement{DocumentType: "doc_selected", SchemaVersion: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def := models.DossierDefinition{Name: "Draft", Documents: []models.DocumentRequirement{tc.requirement}}
			l := NewCatalogLogic(repo, suggestionAudit{}, nil, zerolog.Nop())
			_, err := l.validateDossier(ctx, repo.Scope(actor.CompanyID), &def, nil)
			require.ErrorIs(t, err, common.ErrInvalidArgument)
			var validation *catalogValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tc.rule, validation.Rule)
			status, _ := common.HTTPError(err)
			assert.Equal(t, 400, status, "saving invalid user input keeps its existing HTTP contract")
		})
	}
}
