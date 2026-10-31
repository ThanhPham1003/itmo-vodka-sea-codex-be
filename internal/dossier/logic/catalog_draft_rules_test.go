package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
)

func draftRulesInput() dto.DraftRuleSuggestionRequest {
	return dto.DraftRuleSuggestionRequest{Description: "Compare owner names", Definition: models.DossierDefinition{
		Name: "New sale", Description: "Draft configuration", Documents: []models.DocumentRequirement{
			{DocumentType: "doc_selected", SchemaVersion: 3, Role: "buyer", Required: true, MinCount: 1},
			{DocumentType: "doc_unselected", SchemaVersion: 3, Role: "seller", Required: true, MinCount: 1},
		},
	}}
}

func TestDraftRulesRemainUnsavedAndCanBeSavedOnce(t *testing.T) {
	repo, ctx := dossierSuggestionFixture(t)
	audit := &dossierSuggestionAudit{}
	rule := ruleSuggestionExample()
	rule.Left.Role, rule.Right.Role = "buyer", "seller"
	raw, err := json.Marshal(map[string]any{"rules": []models.ComparisonRule{rule}})
	require.NoError(t, err)
	client := &ruleStructuredAI{suggestionAI: suggestionAI{raw: raw}}
	l := NewCatalogLogic(repo, audit, client, zerolog.Nop())
	in := draftRulesInput()
	a, _ := common.ActorFromContext(ctx)
	// Current catalog changed after the UI loaded; use the selected schema version.
	doc, err := repo.Scope(a.CompanyID).DocumentType(ctx, "doc_selected", false)
	require.NoError(t, err)
	doc.CurrentVersion = 4
	doc.Definition.Fields = []models.FieldSchema{{Key: "new_field", Label: "New", Type: "date"}}
	require.NoError(t, repo.DB().Save(&doc).Error)
	out, err := l.SuggestDraftRules(ctx, in)
	require.NoError(t, err)
	var draft struct {
		Draft   bool                    `json:"draft"`
		Rules   []models.ComparisonRule `json:"rules"`
		Version *int                    `json:"configuration_version"`
	}
	require.NoError(t, json.Unmarshal(out, &draft))
	assert.True(t, draft.Draft)
	assert.Nil(t, draft.Version)
	require.Equal(t, []models.ComparisonRule{rule}, draft.Rules)
	assert.Contains(t, client.prompt, `"buyer"`)
	assert.NotContains(t, client.prompt, "new_field")
	assert.NotContains(t, client.prompt, "private-foreign")
	assert.Empty(t, in.Definition.Documents[0].Key, "input remains unchanged")
	require.Len(t, audit.events, 1)
	assert.Equal(t, uuid.Nil, audit.events[0].ResourceID)
	for _, model := range []any{&models.DossierType{}, &models.DossierTypeVersion{}} {
		var count int64
		require.NoError(t, repo.DB().Model(model).Count(&count).Error)
		assert.Zero(t, count, "AI must not create a type or version")
	}
	in.Definition.Rules = draft.Rules
	saved, err := l.PutDossierType(ctx, "", dto.DossierTypeRequest{DossierDefinition: in.Definition})
	require.NoError(t, err)
	assert.Equal(t, 1, saved.CurrentVersion)
	require.Len(t, saved.Definition.Rules, 1)
	assert.NotEmpty(t, saved.Definition.Rules[0].Code)
}

func TestDraftRulesValidateBeforeCallingAI(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*dto.DraftRuleSuggestionRequest)
	}{
		{"blank description", func(in *dto.DraftRuleSuggestionRequest) { in.Description = " " }},
		{"long description", func(in *dto.DraftRuleSuggestionRequest) { in.Description = strings.Repeat("x", 4001) }},
		{"no documents", func(in *dto.DraftRuleSuggestionRequest) { in.Definition.Documents = nil }},
		{"foreign document", func(in *dto.DraftRuleSuggestionRequest) { in.Definition.Documents[0].DocumentType = "doc_foreign" }},
		{"inactive document", func(in *dto.DraftRuleSuggestionRequest) { in.Definition.Documents[0].DocumentType = "doc_inactive" }},
		{"missing version", func(in *dto.DraftRuleSuggestionRequest) { in.Definition.Documents[0].SchemaVersion = 99 }},
		{"duplicate role", func(in *dto.DraftRuleSuggestionRequest) {
			in.Definition.Documents = append(in.Definition.Documents, in.Definition.Documents[0])
		}},
		{"unknown key", func(in *dto.DraftRuleSuggestionRequest) { in.Definition.Documents[0].Key = "invented" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, ctx := dossierSuggestionFixture(t)
			in := draftRulesInput()
			tc.mutate(&in)
			// A nil AI client would panic if validation reached the provider.
			l := NewCatalogLogic(repo, &dossierSuggestionAudit{}, nil, zerolog.Nop())
			_, err := l.SuggestDraftRules(ctx, in)
			require.ErrorIs(t, err, common.ErrInvalidArgument)
		})
	}
	for _, tc := range []struct {
		name     string
		ctx      context.Context
		expected error
	}{
		{"unauthenticated", context.Background(), common.ErrUnauthenticated},
		{"non admin", common.WithActor(context.Background(), common.Actor{UserID: uuid.New(), CompanyID: uuid.New(), Role: "manager"}), common.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := NewCatalogLogic(nil, nil, nil, zerolog.Nop())
			_, err := l.SuggestDraftRules(tc.ctx, draftRulesInput())
			require.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestDraftRulesDependencyAndProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		client   suggestionAI
		auditErr error
	}{
		{"provider", suggestionAI{err: errors.New("unavailable")}, nil},
		{"malformed output", suggestionAI{raw: json.RawMessage(`{"rules":{}}`)}, nil},
		{"foreign output", suggestionAI{raw: json.RawMessage(`{"rules":[{"name":"Mismatch","left":{"document_type":"doc_foreign","field_key":"field_1"},"right":{"document_type":"doc_selected","field_key":"field_1"},"operator":"TEXT_EQUALS","severity":"WARNING"}]}`)}, nil},
		{"unconfigured role", suggestionAI{raw: json.RawMessage(`{"rules":[{"name":"Mismatch","left":{"document_type":"doc_selected","field_key":"field_1","role":"invented"},"right":{"document_type":"doc_selected","field_key":"field_1"},"operator":"TEXT_EQUALS","severity":"WARNING"}]}`)}, nil},
		{"audit", suggestionAI{raw: json.RawMessage(`{"rules":[]}`)}, errors.New("audit unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, ctx := dossierSuggestionFixture(t)
			l := NewCatalogLogic(repo, suggestionAudit{tc.auditErr}, tc.client, zerolog.Nop())
			_, err := l.SuggestDraftRules(ctx, draftRulesInput())
			require.ErrorIs(t, err, common.ErrUnavailable)
		})
	}
	t.Run("database", func(t *testing.T) {
		repo, ctx := dossierSuggestionFixture(t)
		require.NoError(t, repo.DB().Migrator().DropTable(&models.DocumentType{}))
		l := NewCatalogLogic(repo, nil, nil, zerolog.Nop())
		_, err := l.SuggestDraftRules(ctx, draftRulesInput())
		require.ErrorIs(t, err, common.ErrUnavailable)
	})
}
