package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/pkg/ai"
)

type ruleStructuredAI struct {
	suggestionAI
	prompt string
	schema json.RawMessage
}

func (s *ruleStructuredAI) CompleteJSON(_ context.Context, prompt string, _ []ai.Image, schema json.RawMessage) (json.RawMessage, error) {
	s.prompt, s.schema = prompt, schema
	return s.raw, s.err
}

func ruleSuggestionExample() models.ComparisonRule {
	return models.ComparisonRule{
		Name: "private-rule-name", Operator: "TEXT_EQUALS", Severity: "WARNING",
		Left:    models.FieldReference{DocumentType: "doc_selected", FieldKey: "field_1", SubjectField: "field_1"},
		Right:   models.FieldReference{DocumentType: "doc_unselected", FieldKey: "field_1", SubjectField: "field_1"},
		Message: "private-message", Remediation: "private-remediation",
	}
}

func TestRuleSuggestionsUsePinnedSchemasAndSaveValidation(t *testing.T) {
	for _, role := range []string{"", "bên mua"} {
		t.Run("role="+role, func(t *testing.T) {
			repo, ctx := dossierSuggestionFixture(t)
			audit := &dossierSuggestionAudit{}
			client := &ruleStructuredAI{}
			l := NewCatalogLogic(repo, audit, client, zerolog.Nop())
			saved, err := l.PutDossierType(ctx, "", dto.DossierTypeRequest{DossierDefinition: models.DossierDefinition{
				Name: "Sale", Documents: []models.DocumentRequirement{
					{DocumentType: "doc_selected", SchemaVersion: 3, Role: role, Required: true},
					{DocumentType: "doc_unselected", SchemaVersion: 3, Required: true},
				},
			}})
			require.NoError(t, err)
			// Catalog has moved on. Suggestions must still use the pinned version.
			a, _ := common.ActorFromContext(ctx)
			doc, err := repo.Scope(a.CompanyID).DocumentType(ctx, "doc_selected", false)
			require.NoError(t, err)
			doc.CurrentVersion = 4
			doc.Definition.Fields = []models.FieldSchema{{Key: "new_field", Label: "New", Type: "date"}}
			require.NoError(t, repo.DB().Save(&doc).Error)
			rule := ruleSuggestionExample()
			rule.Left.Role = role
			client.raw, err = json.Marshal(map[string]any{"rules": []models.ComparisonRule{rule}})
			require.NoError(t, err)
			raw, err := l.SuggestRules(ctx, saved.Code, "private-prompt")
			require.NoError(t, err)
			assert.NotContains(t, client.prompt, "new_field")
			assert.Contains(t, client.prompt, `always use role=""`)
			var schema struct {
				Properties struct {
					Rules struct {
						Items struct {
							Properties map[string]struct {
								AnyOf []struct {
									Properties map[string]struct {
										Enum []string `json:"enum"`
									} `json:"properties"`
								} `json:"anyOf"`
							} `json:"properties"`
						} `json:"items"`
					} `json:"rules"`
				} `json:"properties"`
			}
			require.NoError(t, json.Unmarshal(client.schema, &schema))
			for _, side := range []string{"left", "right"} {
				branches := schema.Properties.Rules.Items.Properties[side].AnyOf
				require.Len(t, branches, 2)
				assert.Equal(t, []string{"doc_selected"}, branches[0].Properties["document_type"].Enum)
				assert.Equal(t, []string{"field_1"}, branches[0].Properties["field_key"].Enum)
				expectedRoles := []string{""}
				if role != "" {
					expectedRoles = append(expectedRoles, role)
				}
				assert.Equal(t, expectedRoles, branches[0].Properties["role"].Enum)
				assert.Equal(t, []string{""}, branches[1].Properties["role"].Enum, "roles are scoped per document")
			}
			var draft struct {
				Draft   bool                    `json:"draft"`
				Rules   []models.ComparisonRule `json:"rules"`
				Version int                     `json:"configuration_version"`
			}
			require.NoError(t, json.Unmarshal(raw, &draft))
			assert.True(t, draft.Draft)
			assert.Equal(t, saved.CurrentVersion, draft.Version)
			require.Len(t, draft.Rules, 1)
			assert.Equal(t, rule, draft.Rules[0], "preserve roles, subject joins, messages and blank code")
			unchanged, err := repo.Scope(a.CompanyID).DossierType(ctx, saved.Code, false)
			require.NoError(t, err)
			assert.Equal(t, saved.CurrentVersion, unchanged.CurrentVersion)
			assert.Empty(t, unchanged.Definition.Rules, "no automatic save")
			saved.Definition.Rules = draft.Rules
			updated, err := l.PutDossierType(ctx, saved.Code, dto.DossierTypeRequest{DossierDefinition: saved.Definition, ExpectedVersion: draft.Version})
			require.NoError(t, err)
			assert.Equal(t, 2, updated.CurrentVersion)
			assert.NotEmpty(t, updated.Definition.Rules[0].Code)
		})
	}
}

func TestRuleSuggestionsRejectUnsavableDrafts(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		mutate       func(*models.ComparisonRule)
	}{
		{"buyer role from field name", "invalid_rule_role", func(r *models.ComparisonRule) { r.Left.Role = "bên mua" }},
		{"seller role from field name", "invalid_rule_role", func(r *models.ComparisonRule) { r.Right.Role = "bên bán" }},
		{"invented code", "unknown_identifier", func(r *models.ComparisonRule) { r.Code = "private-code" }},
		{"empty name", "invalid_rule_text", func(r *models.ComparisonRule) { r.Name = " " }},
		{"long message", "invalid_rule_text", func(r *models.ComparisonRule) { r.Message = strings.Repeat("x", 2001) }},
		{"unpaired join", "incompatible_rule", func(r *models.ComparisonRule) { r.Right.SubjectField = "" }},
		{"foreign document", "incompatible_rule", func(r *models.ComparisonRule) { r.Left.DocumentType = "doc_foreign" }},
		{"wrong type", "incompatible_rule", func(r *models.ComparisonRule) { r.Operator = "DATE_EQUAL" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, ctx := dossierSuggestionFixture(t)
			var buf bytes.Buffer
			audit := &dossierSuggestionAudit{}
			l := NewCatalogLogic(repo, audit, nil, zerolog.New(&buf))
			saved, err := l.PutDossierType(ctx, "", dto.DossierTypeRequest{DossierDefinition: models.DossierDefinition{
				Name: "Sale", Documents: []models.DocumentRequirement{{DocumentType: "doc_selected", SchemaVersion: 3}, {DocumentType: "doc_unselected", SchemaVersion: 3}},
			}})
			require.NoError(t, err)
			audit.events = nil
			rule := ruleSuggestionExample()
			tc.mutate(&rule)
			raw, err := json.Marshal(map[string]any{"rules": []models.ComparisonRule{rule}})
			require.NoError(t, err)
			l.ai = suggestionAI{raw: raw} // Legacy providers must also pass business validation.
			_, err = l.SuggestRules(ctx, saved.Code, "private-prompt")
			require.ErrorIs(t, err, common.ErrUnavailable)
			assert.Empty(t, audit.events)
			var entry map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
			assert.Equal(t, "comparison_rules", entry["operation"])
			assert.Equal(t, tc.reason, entry["validation_error"])
			assert.EqualValues(t, 0, entry["item_index"])
			buf.Reset()
			saved.Definition.Rules = []models.ComparisonRule{rule}
			_, err = l.PutDossierType(ctx, saved.Code, dto.DossierTypeRequest{DossierDefinition: saved.Definition, ExpectedVersion: saved.CurrentVersion})
			require.ErrorIs(t, err, common.ErrInvalidArgument)
			require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
			assert.Equal(t, "catalog save rejected", entry["message"])
			assert.Equal(t, tc.reason, entry["validation_error"])
			assert.Equal(t, "dossier-request-id", entry["request_id"])
			for _, private := range []string{"private-prompt", "private-rule-name", "private-message", "private-remediation", "bên mua", "bên bán"} {
				assert.NotContains(t, buf.String(), private)
			}
		})
	}
}
