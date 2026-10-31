package logic

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"sea-codex-server/pkg/ai"
	"strings"
)

func (l *CatalogLogic) SuggestDossier(ctx context.Context, description string, codes []string) (json.RawMessage, error) {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(description) == "" || len(description) > 4000 || len(codes) > 50 {
		return nil, common.ErrInvalidArgument
	}
	r := l.repo.Scope(a.CompanyID)
	available := []models.DocumentType{}
	if len(codes) == 0 {
		list, err := r.DocumentTypes(ctx, common.Page{Limit: 100})
		if err != nil {
			return nil, repositories.Error(err)
		}
		for _, doc := range list.Items {
			if doc.Active {
				available = append(available, doc)
			}
		}
	} else {
		for _, c := range codes {
			doc, err := r.DocumentType(ctx, c, false)
			if err != nil || !doc.Active {
				return nil, common.ErrInvalidArgument
			}
			available = append(available, doc)
		}
	}
	if len(available) == 0 {
		return nil, common.ErrInvalidArgument
	}
	type catalogEntry struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	catalog := make([]catalogEntry, 0, len(available))
	versions := make(map[string]int, len(available))
	for _, doc := range available {
		catalog = append(catalog, catalogEntry{doc.Code, doc.Definition.Name, doc.Definition.Description})
		versions[doc.Code] = doc.CurrentVersion
	}
	config, _ := json.Marshal(map[string]any{"description": description, "catalog": catalog})
	prompt := dossierSuggestionPrompt + string(config)
	schema := dossierSuggestionSchema(available)
	var raw json.RawMessage
	if client, ok := l.ai.(ai.StructuredClient); ok {
		raw, err = client.CompleteJSON(ctx, prompt, nil, schema)
	} else {
		raw, err = l.ai.Complete(ctx, prompt+"\nOutput JSON schema: "+string(schema), nil)
	}
	if err != nil {
		l.logSuggestionFailure(ctx, "dossier_type", "ai_request", err)
		return nil, common.ErrUnavailable
	}
	var suggestion dossierSuggestion
	if err = strictJSON(raw, &suggestion); err != nil {
		l.logSuggestionFailure(ctx, "dossier_type", "decode", err)
		return nil, common.ErrUnavailable
	}
	def := models.DossierDefinition{
		Name: suggestion.Name, Description: suggestion.Description,
		Documents: make([]models.DocumentRequirement, 0, len(suggestion.Documents)),
		Rules:     []models.ComparisonRule{},
	}
	for i, doc := range suggestion.Documents {
		version, ok := versions[doc.DocumentType]
		if !ok {
			l.logSuggestionFailure(ctx, "dossier_type", "validation", invalidCatalogDefinition("document_type_not_selected", "documents.document_type", i))
			return nil, common.ErrUnavailable
		}
		def.Documents = append(def.Documents, models.DocumentRequirement{
			Position: i, DocumentType: doc.DocumentType, SchemaVersion: version,
			Role: doc.Role, Required: doc.Required, Conditional: doc.Conditional,
			MinCount: doc.MinCount, TriggerReason: doc.TriggerReason,
		})
	}
	if _, err = l.validateDossier(ctx, r, &def, nil); err != nil {
		l.logSuggestionFailure(ctx, "dossier_type", "validation", err)
		return nil, common.ErrUnavailable
	}
	for i := range def.Documents {
		def.Documents[i].Key = ""
	}
	for i := range def.Rules {
		def.Rules[i].Code = ""
	}
	if err = l.audit.Write(ctx, nil, common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOSSIER_CONFIG_SUGGESTED", Resource: "catalog", ResourceID: uuid.Nil}); err != nil {
		l.logSuggestionFailure(ctx, "dossier_type", "audit", err)
		return nil, repositories.Error(err)
	}
	return json.Marshal(map[string]any{"draft": true, "definition": def})
}

// SuggestDraftRules validates the unsaved configuration without creating a catalog row.
func (l *CatalogLogic) SuggestDraftRules(ctx context.Context, in dto.DraftRuleSuggestionRequest) (json.RawMessage, error) {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Description) == "" || len(in.Description) > 4000 {
		return nil, common.ErrInvalidArgument
	}
	definition := in.Definition
	definition.Documents = append([]models.DocumentRequirement(nil), in.Definition.Documents...)
	definition.Rules = append([]models.ComparisonRule(nil), in.Definition.Rules...)
	r := l.repo.Scope(a.CompanyID)
	pinned, err := l.validateDossier(ctx, r, &definition, nil)
	if err != nil {
		return nil, err
	}
	docs := make(map[string]models.DocumentDefinition, len(pinned))
	for _, doc := range pinned {
		docs[doc.Code] = doc.Definition
	}
	return l.suggestRulesForDefinition(ctx, a, r, definition, docs, uuid.Nil, 0, in.Description)
}
