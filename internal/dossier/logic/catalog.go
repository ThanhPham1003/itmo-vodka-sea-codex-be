package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"sea-codex-server/pkg/ai"
)

type CatalogLogic struct {
	repo  *repositories.DossierRepository
	audit common.AuditWriter
	ai    ai.Client
	log   zerolog.Logger
}

func NewCatalogLogic(r *repositories.DossierRepository, a common.AuditWriter, c ai.Client, log zerolog.Logger) *CatalogLogic {
	return &CatalogLogic{repo: r, audit: a, ai: c, log: log}
}
func actor(ctx context.Context, permission string) (common.Actor, error) {
	a, ok := common.ActorFromContext(ctx)
	if !ok {
		return a, common.ErrUnauthenticated
	}
	if !a.HasPermission(permission) {
		return a, common.ErrForbidden
	}
	return a, nil
}
func catalogAdmin(ctx context.Context) (common.Actor, error) {
	a, ok := common.ActorFromContext(ctx)
	if !ok {
		return a, common.ErrUnauthenticated
	}
	if a.Role != common.RoleCompanyAdmin {
		return a, common.ErrForbidden
	}
	return a, nil
}
func code(prefix string) string { return prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "") }
func fieldType(t string) string {
	switch t {
	case "string":
		return "text"
	case "integer":
		return "number"
	default:
		return t
	}
}
func validateDocument(d *models.DocumentDefinition, previous *models.DocumentDefinition) error {
	if strings.TrimSpace(d.Name) == "" || utf8.RuneCountInString(d.Name) > 200 || len(d.Description) > 8000 || len(d.Fields) == 0 || len(d.Fields) > 100 || len(d.SpecialRequirements) > 20 {
		return common.ErrInvalidArgument
	}
	old := map[string]bool{}
	oldSpecial := map[string]bool{}
	if previous != nil {
		for _, f := range previous.Fields {
			old[f.Key] = true
		}
		for _, s := range previous.SpecialRequirements {
			oldSpecial[s.Code] = true
		}
	}
	seen := map[string]bool{}
	for i := range d.Fields {
		f := &d.Fields[i]
		f.Type = fieldType(f.Type)
		if f.Key == "" {
			f.Key = code("field")
		} else if !old[f.Key] {
			return common.ErrInvalidArgument
		}
		if seen[f.Key] || strings.TrimSpace(f.Label) == "" || len(f.Label) > 600 || len(f.Pattern) > 200 {
			return common.ErrInvalidArgument
		}
		seen[f.Key] = true
		switch f.Type {
		case "text", "date", "number", "money", "area", "boolean", "identifier":
		default:
			return common.ErrInvalidArgument
		}
		if f.Pattern != "" {
			if _, err := regexp.Compile(f.Pattern); err != nil {
				return common.ErrInvalidArgument
			}
		}
	}
	specialSeen := map[string]bool{}
	for i := range d.SpecialRequirements {
		s := &d.SpecialRequirements[i]
		if s.Code == "" {
			s.Code = code("check")
		} else if !oldSpecial[s.Code] {
			return common.ErrInvalidArgument
		}
		if specialSeen[s.Code] {
			return common.ErrInvalidArgument
		}
		specialSeen[s.Code] = true
		if strings.TrimSpace(s.Description) == "" || len(s.Description) > 4000 || len(s.ExternalSource) > 500 {
			return common.ErrInvalidArgument
		}
		if s.Severity == "" {
			s.Severity = "WARNING"
		}
		if s.Severity != "WARNING" && s.Severity != "BLOCKER" && s.Severity != "INFO" {
			return common.ErrInvalidArgument
		}
	}
	return nil
}
func (l *CatalogLogic) ListDocumentTypes(ctx context.Context, p common.Page) (common.List[models.DocumentType], error) {
	a, err := actor(ctx, "dossier:read")
	if err != nil {
		return common.List[models.DocumentType]{}, err
	}
	out, err := l.repo.Scope(a.CompanyID).DocumentTypes(ctx, p)
	return out, repositories.Error(err)
}
func (l *CatalogLogic) GetDocumentType(ctx context.Context, c string) (models.DocumentType, error) {
	a, err := actor(ctx, "dossier:read")
	if err != nil {
		return models.DocumentType{}, err
	}
	out, err := l.repo.Scope(a.CompanyID).DocumentType(ctx, c, false)
	return out, repositories.Error(err)
}
func (l *CatalogLogic) PutDocumentType(ctx context.Context, c string, in dto.DocumentTypeRequest) (models.DocumentType, error) {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return models.DocumentType{}, err
	}
	var out models.DocumentType
	err = l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		var previous *models.DocumentDefinition
		if c != "" {
			var err error
			out, err = r.DocumentType(ctx, c, true)
			if err != nil {
				return err
			}
			if in.ExpectedVersion != out.CurrentVersion {
				return common.ErrConflict
			}
			copy := out.Definition
			previous = &copy
		} else {
			out = models.DocumentType{CompanyID: a.CompanyID, Code: code("doc"), Active: true}
		}
		if err := validateDocument(&in.DocumentDefinition, previous); err != nil {
			return err
		}
		out.Definition = in.DocumentDefinition
		out.CurrentVersion++
		if in.Active != nil {
			out.Active = *in.Active
		}
		if c == "" {
			if err := r.Create(ctx, &out); err != nil {
				return err
			}
		} else {
			if err := r.Save(ctx, &out); err != nil {
				return err
			}
		}
		v := models.DocumentTypeVersion{CompanyID: a.CompanyID, DocumentTypeID: out.ID, Version: out.CurrentVersion, Definition: out.Definition}
		if err := r.Create(ctx, &v); err != nil {
			return err
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOCUMENT_TYPE_VERSIONED", Resource: "document_type", ResourceID: out.ID, Metadata: map[string]any{"version": out.CurrentVersion}})
	})
	return out, repositories.Error(err)
}
func (l *CatalogLogic) ArchiveDocumentType(ctx context.Context, c string) error {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return err
	}
	return repositories.Error(l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		d, err := r.DocumentType(ctx, c, true)
		if err != nil {
			return err
		}
		d.Active = false
		if err = r.Save(ctx, &d); err != nil {
			return err
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOCUMENT_TYPE_ARCHIVED", Resource: "document_type", ResourceID: d.ID})
	}))
}
func (l *CatalogLogic) ListDossierTypes(ctx context.Context, p common.Page) (common.List[models.DossierType], error) {
	a, err := actor(ctx, "dossier:read")
	if err != nil {
		return common.List[models.DossierType]{}, err
	}
	out, err := l.repo.Scope(a.CompanyID).DossierTypes(ctx, p)
	if err == nil {
		err = l.repo.Scope(a.CompanyID).ResolveCatalogSchemas(ctx, out.Items)
	}
	return out, repositories.Error(err)
}
func (l *CatalogLogic) GetDossierType(ctx context.Context, c string) (models.DossierType, error) {
	a, err := actor(ctx, "dossier:read")
	if err != nil {
		return models.DossierType{}, err
	}
	out, err := l.repo.Scope(a.CompanyID).DossierType(ctx, c, false)
	if err == nil {
		items := []models.DossierType{out}
		err = l.repo.Scope(a.CompanyID).ResolveCatalogSchemas(ctx, items)
		out = items[0]
	}
	return out, repositories.Error(err)
}
func lookupField(def models.DocumentDefinition, key string) (models.FieldSchema, bool) {
	for _, f := range def.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return models.FieldSchema{}, false
}
func compatibleRule(rule models.ComparisonRule, docs map[string]models.DocumentDefinition) bool {
	ld, lok := docs[rule.Left.DocumentType]
	rd, rok := docs[rule.Right.DocumentType]
	if !lok || !rok {
		return false
	}
	lf, lok := lookupField(ld, rule.Left.FieldKey)
	rf, rok := lookupField(rd, rule.Right.FieldKey)
	if !lok || !rok {
		return false
	}
	if (rule.Left.SubjectField == "") != (rule.Right.SubjectField == "") {
		return false
	}
	for _, ref := range []models.FieldReference{rule.Left, rule.Right} {
		if ref.SubjectField != "" {
			f, ok := lookupField(docs[ref.DocumentType], ref.SubjectField)
			if !ok || (f.Type != "text" && f.Type != "identifier") {
				return false
			}
		}
	}
	if rule.Severity != "WARNING" && rule.Severity != "BLOCKER" && rule.Severity != "INFO" {
		return false
	}
	switch rule.Operator {
	case "TEXT_EQUALS":
		return (lf.Type == "text" || lf.Type == "identifier") && (rf.Type == "text" || rf.Type == "identifier")
	case "DIGITS_EQUAL":
		return (lf.Type == "text" || lf.Type == "identifier") && (rf.Type == "text" || rf.Type == "identifier")
	case "DATE_EQUAL":
		return lf.Type == "date" && rf.Type == "date"
	case "NUMBER_EQUAL":
		return lf.Type == rf.Type && (lf.Type == "number" || lf.Type == "money" || lf.Type == "area")
	}
	return false
}
func (l *CatalogLogic) validateDossier(ctx context.Context, r *repositories.DossierRepository, d *models.DossierDefinition, previous *models.DossierDefinition) ([]models.PinnedDocument, error) {
	if strings.TrimSpace(d.Name) == "" || len(d.Name) > 600 {
		return nil, invalidCatalogDefinition("invalid_name", "name", -1)
	}
	if len(d.Description) > 8000 {
		return nil, invalidCatalogDefinition("description_too_long", "description", -1)
	}
	if len(d.Documents) == 0 || len(d.Documents) > 50 {
		return nil, invalidCatalogDefinition("invalid_document_count", "documents", -1)
	}
	if len(d.Rules) > 200 {
		return nil, invalidCatalogDefinition("too_many_rules", "rules", -1)
	}
	oldReq := map[string]bool{}
	oldRule := map[string]bool{}
	if previous != nil {
		for _, v := range previous.Documents {
			oldReq[v.Key] = true
		}
		for _, v := range previous.Rules {
			oldRule[v.Code] = true
		}
	}
	docs := map[string]models.DocumentDefinition{}
	versions := map[string]int{}
	pinned := []models.PinnedDocument{}
	keys := map[string]bool{}
	requirementPairs := map[string]bool{}
	for i := range d.Documents {
		req := &d.Documents[i]
		if req.Position < 0 || req.Position > 1000 {
			return nil, invalidCatalogDefinition("invalid_position", "documents.position", i)
		}
		if req.Position == 0 && i > 0 {
			req.Position = i
		}
		if req.Key == "" {
			req.Key = code("requirement")
		} else if !oldReq[req.Key] {
			return nil, invalidCatalogDefinition("unknown_identifier", "documents.key", i)
		}
		if keys[req.Key] {
			return nil, invalidCatalogDefinition("duplicate_identifier", "documents.key", i)
		}
		keys[req.Key] = true
		pair := req.DocumentType + "\x00" + req.Role
		if requirementPairs[pair] {
			return nil, invalidCatalogDefinition("duplicate_document_role", "documents.role", i)
		}
		requirementPairs[pair] = true
		if req.MinCount == 0 {
			req.MinCount = 1
		}
		if req.MinCount < 1 || req.MinCount > 100 {
			return nil, invalidCatalogDefinition("invalid_min_count", "documents.min_count", i)
		}
		if len(req.Role) > 80 {
			return nil, invalidCatalogDefinition("role_too_long", "documents.role", i)
		}
		if req.Conditional && strings.TrimSpace(req.TriggerReason) == "" {
			return nil, invalidCatalogDefinition("missing_trigger_reason", "documents.trigger_reason", i)
		}
		doc, err := r.DocumentType(ctx, req.DocumentType, false)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repositories.Error(err)
		}
		if err != nil || !doc.Active {
			return nil, invalidCatalogDefinition("document_type_unavailable", "documents.document_type", i)
		}
		if req.SchemaVersion == 0 {
			req.SchemaVersion = doc.CurrentVersion
		}
		v, err := r.DocumentVersion(ctx, doc.ID, req.SchemaVersion)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repositories.Error(err)
		}
		if err != nil {
			return nil, invalidCatalogDefinition("schema_version_unavailable", "documents.schema_version", i)
		}
		if old, ok := versions[doc.Code]; ok && old != req.SchemaVersion {
			return nil, invalidCatalogDefinition("inconsistent_schema_version", "documents.schema_version", i)
		}
		if _, ok := docs[doc.Code]; !ok {
			pinned = append(pinned, models.PinnedDocument{Code: doc.Code, Version: req.SchemaVersion, Definition: v.Definition})
		}
		docs[doc.Code] = v.Definition
		versions[doc.Code] = req.SchemaVersion
	}
	for i, req := range d.Documents {
		if req.Condition != nil {
			if !req.Conditional {
				return nil, invalidCatalogDefinition("condition_without_flag", "documents.condition", i)
			}
			c := req.Condition
			def, ok := docs[c.DocumentType]
			if !ok {
				return nil, invalidCatalogDefinition("condition_document_not_configured", "documents.condition.document_type", i)
			}
			field, found := lookupField(def, c.FieldKey)
			if !found || (c.Operator != "equals" && c.Operator != "exists") || !configuredReference(d.Documents, c.DocumentType, c.Role) {
				return nil, invalidCatalogDefinition("invalid_condition_reference", "documents.condition", i)
			}
			if c.Operator == "equals" {
				if _, err := normalizeField(c.Value, field.Type); err != nil {
					return nil, invalidCatalogDefinition("invalid_condition_value", "documents.condition.value", i)
				}
			}
			if c.DocumentType == req.DocumentType {
				return nil, invalidCatalogDefinition("self_referencing_condition", "documents.condition.document_type", i)
			}
		}
	}
	edges := map[string][]string{}
	for _, req := range d.Documents {
		if req.Condition != nil {
			edges[req.DocumentType] = append(edges[req.DocumentType], req.Condition.DocumentType)
		}
	}
	state := map[string]int{}
	var cycle func(string) bool
	cycle = func(node string) bool {
		if state[node] == 1 {
			return true
		}
		if state[node] == 2 {
			return false
		}
		state[node] = 1
		for _, next := range edges[node] {
			if cycle(next) {
				return true
			}
		}
		state[node] = 2
		return false
	}
	for node := range edges {
		if cycle(node) {
			return nil, invalidCatalogDefinition("condition_cycle", "documents.condition", -1)
		}
	}
	seen := map[string]bool{}
	for i := range d.Rules {
		rule := &d.Rules[i]
		if rule.Code == "" {
			rule.Code = code("rule")
		} else if !oldRule[rule.Code] {
			return nil, invalidCatalogDefinition("unknown_identifier", "rules.code", i)
		}
		if seen[rule.Code] {
			return nil, invalidCatalogDefinition("duplicate_identifier", "rules.code", i)
		}
		seen[rule.Code] = true
		if rule.Severity == "" {
			rule.Severity = "WARNING"
		}
		if strings.TrimSpace(rule.Name) == "" || len(rule.Name) > 300 || len(rule.Message) > 2000 || len(rule.Remediation) > 2000 {
			return nil, invalidCatalogDefinition("invalid_rule_text", "rules", i)
		}
		if !compatibleRule(*rule, docs) {
			return nil, invalidCatalogDefinition("incompatible_rule", "rules", i)
		}
		if !configuredReference(d.Documents, rule.Left.DocumentType, rule.Left.Role) || !configuredReference(d.Documents, rule.Right.DocumentType, rule.Right.Role) {
			return nil, invalidCatalogDefinition("invalid_rule_role", "rules", i)
		}
	}
	return pinned, nil
}
func (l *CatalogLogic) PutDossierType(ctx context.Context, c string, in dto.DossierTypeRequest) (models.DossierType, error) {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return models.DossierType{}, err
	}
	var out models.DossierType
	err = l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		var previous *models.DossierDefinition
		if c != "" {
			var err error
			out, err = r.DossierType(ctx, c, true)
			if err != nil {
				return err
			}
			if in.ExpectedVersion != out.CurrentVersion {
				return common.ErrConflict
			}
			copy := out.Definition
			previous = &copy
		} else {
			out = models.DossierType{CompanyID: a.CompanyID, Code: code("dossier"), Active: true}
		}
		if _, err := l.validateDossier(ctx, r, &in.DossierDefinition, previous); err != nil {
			l.logCatalogValidationFailure(ctx, err)
			return err
		}
		out.Definition = in.DossierDefinition
		out.CurrentVersion++
		if in.Active != nil {
			out.Active = *in.Active
		}
		if c == "" {
			if err := r.Create(ctx, &out); err != nil {
				return err
			}
		} else {
			if err := r.Save(ctx, &out); err != nil {
				return err
			}
		}
		if err := r.Create(ctx, &models.DossierTypeVersion{CompanyID: a.CompanyID, DossierTypeID: out.ID, Version: out.CurrentVersion, Definition: out.Definition}); err != nil {
			return err
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOSSIER_TYPE_VERSIONED", Resource: "dossier_type", ResourceID: out.ID, Metadata: map[string]any{"version": out.CurrentVersion}})
	})
	return out, repositories.Error(err)
}
func (l *CatalogLogic) ArchiveDossierType(ctx context.Context, c string) error {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return err
	}
	return repositories.Error(l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		d, err := r.DossierType(ctx, c, true)
		if err != nil {
			return err
		}
		d.Active = false
		if err = r.Save(ctx, &d); err != nil {
			return err
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOSSIER_TYPE_ARCHIVED", Resource: "dossier_type", ResourceID: d.ID})
	}))
}
func strictJSON(raw []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("multiple JSON values")
	}
	return nil
}
func (l *CatalogLogic) SuggestDocument(ctx context.Context, description string) (json.RawMessage, error) {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(description) == "" || len(description) > 4000 {
		return nil, common.ErrInvalidArgument
	}
	prompt := documentSuggestionPrompt + description
	var raw json.RawMessage
	if client, ok := l.ai.(ai.StructuredClient); ok {
		raw, err = client.CompleteJSON(ctx, prompt, nil, json.RawMessage(documentSuggestionSchema))
	} else {
		raw, err = l.ai.Complete(ctx, prompt+"\nOutput JSON schema: "+documentSuggestionSchema, nil)
	}
	if err != nil {
		l.logSuggestionFailure(ctx, "document_type", "ai_request", err)
		return nil, common.ErrUnavailable
	}
	var def models.DocumentDefinition
	if err = strictJSON(raw, &def); err != nil {
		l.logSuggestionFailure(ctx, "document_type", "decode", err)
		return nil, common.ErrUnavailable
	}
	if err = validateDocument(&def, nil); err != nil {
		l.logSuggestionFailure(ctx, "document_type", "validation", err)
		return nil, common.ErrUnavailable
	}
	// Generated identifiers here are proposal identifiers; saving a draft regenerates them server-side.
	for i := range def.Fields {
		def.Fields[i].Key = ""
	}
	for i := range def.SpecialRequirements {
		def.SpecialRequirements[i].Code = ""
	}
	if err = l.audit.Write(ctx, nil, common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOCUMENT_SCHEMA_SUGGESTED", Resource: "catalog", ResourceID: uuid.Nil}); err != nil {
		l.logSuggestionFailure(ctx, "document_type", "audit", err)
		return nil, repositories.Error(err)
	}
	return json.Marshal(map[string]any{"draft": true, "definition": def})
}
func (l *CatalogLogic) SuggestRules(ctx context.Context, c, description string) (json.RawMessage, error) {
	a, err := catalogAdmin(ctx)
	if err != nil {
		return nil, err
	}
	if len(description) > 4000 {
		return nil, common.ErrInvalidArgument
	}
	r := l.repo.Scope(a.CompanyID)
	d, err := r.DossierType(ctx, c, false)
	if err != nil {
		return nil, repositories.Error(err)
	}
	docs := map[string]models.DocumentDefinition{}
	for _, req := range d.Definition.Documents {
		doc, err := r.DocumentType(ctx, req.DocumentType, false)
		if err != nil {
			return nil, repositories.Error(err)
		}
		v, err := r.DocumentVersion(ctx, doc.ID, req.SchemaVersion)
		if err != nil {
			return nil, repositories.Error(err)
		}
		docs[doc.Code] = v.Definition
	}
	return l.suggestRulesForDefinition(ctx, a, r, d.Definition, docs, d.ID, d.CurrentVersion, description)
}

func (l *CatalogLogic) suggestRulesForDefinition(ctx context.Context, a common.Actor, r *repositories.DossierRepository, definition models.DossierDefinition, docs map[string]models.DocumentDefinition, resourceID uuid.UUID, version int, description string) (json.RawMessage, error) {
	var err error
	config, _ := json.Marshal(map[string]any{"definition": definition, "document_schemas": docs, "description": description})
	prompt := comparisonSuggestionPrompt + string(config)
	schema := comparisonSuggestionSchema(definition.Documents, docs)
	var raw json.RawMessage
	if client, ok := l.ai.(ai.StructuredClient); ok {
		raw, err = client.CompleteJSON(ctx, prompt, nil, schema)
	} else {
		raw, err = l.ai.Complete(ctx, prompt+"\nOutput JSON schema: "+string(schema), nil)
	}
	if err != nil {
		l.logSuggestionFailure(ctx, "comparison_rules", "ai_request", err)
		return nil, common.ErrUnavailable
	}
	var out struct {
		Rules []models.ComparisonRule `json:"rules"`
	}
	if err = strictJSON(raw, &out); err != nil {
		l.logSuggestionFailure(ctx, "comparison_rules", "decode", err)
		return nil, common.ErrUnavailable
	}
	for i, rule := range out.Rules {
		if rule.Code != "" {
			l.logSuggestionFailure(ctx, "comparison_rules", "validation", invalidCatalogDefinition("unknown_identifier", "rules.code", i))
			return nil, common.ErrUnavailable
		}
	}
	// Validate drafts with the same rules as PUT, including configured roles,
	// text limits and pinned field types. Work on a copy; suggestions never save.
	candidate := definition
	candidate.Documents = append([]models.DocumentRequirement(nil), definition.Documents...)
	candidate.Rules = out.Rules
	if _, err = l.validateDossier(ctx, r, &candidate, &definition); err != nil {
		l.logSuggestionFailure(ctx, "comparison_rules", "validation", err)
		return nil, common.ErrUnavailable
	}
	for i := range out.Rules {
		out.Rules[i].Code = ""
	}
	if err = l.audit.Write(ctx, nil, common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "COMPARISON_RULES_SUGGESTED", Resource: "dossier_type", ResourceID: resourceID, Metadata: map[string]any{"version": version, "draft": version == 0}}); err != nil {
		l.logSuggestionFailure(ctx, "comparison_rules", "audit", err)
		return nil, repositories.Error(err)
	}
	response := map[string]any{"draft": true, "rules": out.Rules}
	if version > 0 {
		response["configuration_version"] = version
	}
	return json.Marshal(response)
}

func configuredReference(requirements []models.DocumentRequirement, typ, role string) bool {
	for _, req := range requirements {
		if req.DocumentType == typ && (role == "" || req.Role == role) {
			return true
		}
	}
	return false
}
