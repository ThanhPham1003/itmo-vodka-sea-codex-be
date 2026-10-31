package logic

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"sea-codex-server/pkg/ai"
	"sea-codex-server/pkg/config"
	"sea-codex-server/pkg/storage"
)

type DossierLogic struct {
	repo       *repositories.DossierRepository
	audit      common.AuditWriter
	identities common.IdentityResolver
	storage    *storage.Local
	catalog    *CatalogLogic
	ai         ai.Client
	cfg        *config.Config
	log        zerolog.Logger
}

func NewDossierLogic(r *repositories.DossierRepository, a common.AuditWriter, ids common.IdentityResolver, s *storage.Local, c *CatalogLogic, client ai.Client, cfg *config.Config, log zerolog.Logger) *DossierLogic {
	return &DossierLogic{repo: r, audit: a, identities: ids, storage: s, catalog: c, ai: client, cfg: cfg, log: log}
}
func canRead(a common.Actor, d models.Dossier) bool {
	return a.CompanyID == d.CompanyID && a.HasPermission("dossier:read") && (a.HasPermission("dossier:read_all") || a.UserID == d.CreatorID || d.AssigneeID != nil && *d.AssigneeID == a.UserID)
}
func editable(d models.Dossier) bool {
	return d.Status != models.Approved && d.Status != models.Rejected
}
func (l *DossierLogic) Create(ctx context.Context, in dto.CreateDossierRequest) (models.Dossier, error) {
	a, err := actor(ctx, "dossier:create")
	if err != nil {
		return models.Dossier{}, err
	}
	if strings.TrimSpace(in.Title) == "" || utf8.RuneCountInString(in.Title) > 300 {
		return models.Dossier{}, common.ErrInvalidArgument
	}
	var out models.Dossier
	err = l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		typ, err := r.DossierType(ctx, in.DossierType, true)
		if err != nil {
			return err
		}
		if !typ.Active {
			return common.ErrInvalidArgument
		}
		def := typ.Definition
		pinned, err := l.catalog.validateDossier(ctx, r, &def, &typ.Definition)
		if err != nil {
			return err
		}
		out = models.Dossier{CompanyID: a.CompanyID, CreatorID: a.UserID, Title: strings.TrimSpace(in.Title), Priority: in.Priority, TypeCode: typ.Code, TypeVersion: typ.CurrentVersion, Version: 1, Snapshot: models.DossierSnapshot{Code: typ.Code, Version: typ.CurrentVersion, Definition: def, Documents: pinned}}
		if err = r.Create(ctx, &out); err != nil {
			return err
		}
		b := repositories.Bundle{Dossier: out}
		for _, req := range def.Documents {
			name := req.DocumentType
			for _, d := range pinned {
				if d.Code == req.DocumentType {
					name = d.Definition.Name
					break
				}
			}
			entry := models.Requirement{CompanyID: a.CompanyID, DossierID: out.ID, Key: req.Key, Position: req.Position, DocumentType: req.DocumentType, Role: req.Role, Name: name, Required: req.Required, Conditional: req.Conditional, MinCount: req.MinCount, Condition: req.Condition, Reason: req.TriggerReason, Status: models.Missing}
			if err = r.Create(ctx, &entry); err != nil {
				return err
			}
			b.Requirements = append(b.Requirements, entry)
		}
		if err = l.reconcile(ctx, r, &b, a.UserID); err != nil {
			return err
		}
		out = b.Dossier
		if err = r.Save(ctx, &out); err != nil {
			return err
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOSSIER_CREATED", Resource: "dossier", ResourceID: out.ID, Metadata: map[string]any{"configuration_version": out.TypeVersion}})
	})
	return out, repositories.Error(err)
}
func (l *DossierLogic) List(ctx context.Context, f dto.DossierFilter, review bool) (common.List[models.Dossier], error) {
	permission := "dossier:read"
	if review {
		permission = "dossier:review"
	}
	a, err := actor(ctx, permission)
	if err != nil {
		return common.List[models.Dossier]{}, err
	}
	if len(f.Search) > 300 {
		return common.List[models.Dossier]{}, common.ErrInvalidArgument
	}
	if f.Status != "" {
		valid := false
		for _, status := range []string{models.Processing, models.Missing, models.Invalid, models.Valid, models.Approved, models.Rejected} {
			valid = valid || status == f.Status
		}
		if !valid {
			return common.List[models.Dossier]{}, common.ErrInvalidArgument
		}
	}
	if review {
		f.Review = true
		f.Status = models.Valid
		if a.Role != common.RoleCompanyAdmin {
			f.AssigneeID = &a.UserID
		}
	}
	out, err := l.repo.Scope(a.CompanyID).List(ctx, a, f)
	return out, repositories.Error(err)
}
func (l *DossierLogic) Detail(ctx context.Context, id uuid.UUID) (dto.Detail, error) {
	a, err := actor(ctx, "dossier:read")
	if err != nil {
		return dto.Detail{}, err
	}
	b, err := l.repo.Scope(a.CompanyID).Bundle(ctx, id, false)
	if err != nil {
		return dto.Detail{}, repositories.Error(err)
	}
	if !canRead(a, b.Dossier) {
		return dto.Detail{}, common.ErrNotFound
	}
	return redacted(b), nil
}
func (l *DossierLogic) mutate(ctx context.Context, id uuid.UUID, permission string, version uint64, action string, metadata map[string]any, fn func(*repositories.DossierRepository, *repositories.Bundle, common.Actor) error) error {
	if version == 0 && action != "FILE_UPLOADED" {
		return common.ErrInvalidArgument
	}
	a, err := actor(ctx, permission)
	if err != nil {
		return err
	}
	err = l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		b, err := r.Bundle(ctx, id, true)
		if err != nil {
			return err
		}
		if !canRead(a, b.Dossier) {
			return common.ErrNotFound
		}
		if !editable(b.Dossier) || (version != 0 && version != b.Dossier.Version) {
			return common.ErrConflict
		}
		if err = fn(r, &b, a); err != nil {
			return err
		}
		d := b.Dossier
		d.Version++
		if action != "DOSSIER_ASSIGNED" && action != "WARNING_ACKNOWLEDGED" {
			d.ChangeRequest = ""
		}
		updated, err := r.Bundle(ctx, id, false)
		if err != nil {
			return err
		}
		updated.Dossier = d
		if err = l.reconcile(ctx, r, &updated, a.UserID); err != nil {
			return err
		}
		if err = r.Save(ctx, &updated.Dossier); err != nil {
			return err
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: action, Resource: "dossier", ResourceID: id, Metadata: mutationMetadata(metadata, updated.Dossier.Version)})
	})
	return repositories.Error(err)
}
func pinnedDocument(d models.Dossier, c string) (models.PinnedDocument, bool) {
	for _, doc := range d.Snapshot.Documents {
		if doc.Code == c {
			return doc, true
		}
	}
	return models.PinnedDocument{}, false
}
func validRole(d models.Dossier, typ, role string) bool {
	if role == "" {
		return true
	}
	for _, r := range d.Snapshot.Definition.Documents {
		if r.DocumentType == typ && r.Role == role {
			return true
		}
	}
	return false
}
func (l *DossierLogic) Assign(ctx context.Context, id uuid.UUID, in dto.AssignmentRequest) error {
	a, err := actor(ctx, "dossier:assign")
	if err != nil {
		return err
	}
	// The resolver uses its own connection; perform this read before taking the dossier lock.
	if in.AssigneeID != nil {
		user, err := l.identities.ResolveIdentity(ctx, *in.AssigneeID)
		if err != nil || user.CompanyID != a.CompanyID || !user.HasPermission("dossier:read") || user.Role == "broker" {
			return common.ErrInvalidArgument
		}
	}
	return l.mutate(ctx, id, "dossier:assign", in.ExpectedVersion, "DOSSIER_ASSIGNED", map[string]any{"assignee_id": in.AssigneeID}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
		b.Dossier.AssigneeID = in.AssigneeID
		if in.Priority != nil {
			if *in.Priority < 0 || *in.Priority > 5 {
				return common.ErrInvalidArgument
			}
			b.Dossier.Priority = *in.Priority
		}
		return nil
	})
}
func (l *DossierLogic) AddRequirement(ctx context.Context, id uuid.UUID, in dto.RequirementRequest) error {
	if strings.TrimSpace(in.Reason) == "" || in.MinCount < 0 || in.MinCount > 100 {
		return common.ErrInvalidArgument
	}
	if in.MinCount == 0 {
		in.MinCount = 1
	}
	return l.mutate(ctx, id, "dossier:upload", in.ExpectedVersion, "CONDITIONAL_REQUIREMENT_ADDED", map[string]any{"document_type": in.DocumentType, "role": in.Role}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
		doc, ok := pinnedDocument(b.Dossier, in.DocumentType)
		if !ok || !validRole(b.Dossier, in.DocumentType, in.Role) {
			return common.ErrInvalidArgument
		}
		yes := true
		for _, req := range b.Requirements {
			if req.DocumentType == in.DocumentType && req.Role == in.Role {
				if !req.Conditional {
					return common.ErrConflict
				}
				req.Override = &yes
				req.Reason = in.Reason
				req.MinCount = in.MinCount
				return r.Save(ctx, &req)
			}
		}
		return r.Create(ctx, &models.Requirement{CompanyID: a.CompanyID, DossierID: id, Key: code("requirement"), Position: len(b.Requirements), DocumentType: doc.Code, Role: in.Role, Name: doc.Definition.Name, Required: true, Conditional: true, MinCount: in.MinCount, Override: &yes, Reason: in.Reason, Status: models.Missing})
	})
}
func (l *DossierLogic) WaiveRequirement(ctx context.Context, id, reqID uuid.UUID, in dto.ReasonRequest) error {
	if strings.TrimSpace(in.Reason) == "" {
		return common.ErrInvalidArgument
	}
	return l.mutate(ctx, id, "dossier:upload", in.ExpectedVersion, "CONDITIONAL_REQUIREMENT_NOT_REQUIRED", map[string]any{"requirement_id": reqID}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
		for _, req := range b.Requirements {
			if req.ID == reqID {
				if !req.Conditional {
					return common.ErrConflict
				}
				no := false
				req.Override = &no
				req.Reason = in.Reason
				return r.Save(ctx, &req)
			}
		}
		return common.ErrNotFound
	})
}
func (l *DossierLogic) Correct(ctx context.Context, id, fieldID uuid.UUID, in dto.CorrectionRequest) error {
	if in.Reason != "OCR_ERROR" && in.Reason != "SOURCE_ERROR" && in.Reason != "ADMIN_UNIT_CHANGE" && in.Reason != "OTHER" || in.Reason == "OTHER" && strings.TrimSpace(in.Note) == "" {
		return common.ErrInvalidArgument
	}
	return l.mutate(ctx, id, "dossier:correct", in.ExpectedVersion, "FIELD_CORRECTED", map[string]any{"field_id": fieldID, "reason_code": in.Reason}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
		for _, f := range b.Fields {
			if f.ID == fieldID {
				var schema models.FieldSchema
				found := false
				for _, g := range b.Groups {
					if g.ID == f.GroupID {
						doc, ok := pinnedDocument(b.Dossier, g.DocumentType)
						if ok {
							schema, found = lookupField(doc.Definition, f.Key)
						}
					}
				}
				if !found {
					return common.ErrConflict
				}
				before := f.Value
				f.Value = in.Value
				f.Corrected = true
				f.Confidence = 1
				validateField(&f, schema)
				if f.Validation != "VALID" && schema.Required {
					return common.ErrInvalidArgument
				}
				if err := r.Save(ctx, &f); err != nil {
					return err
				}
				for _, g := range b.Groups {
					if g.ID == f.GroupID {
						for _, file := range b.Files {
							if file.ID == g.FileID {
								source := file
								if source.SourceFileID != nil {
									for _, candidate := range b.Files {
										if candidate.ID == *source.SourceFileID {
											source = candidate
											break
										}
									}
								}
								entry := models.PreservedCorrection{DocumentType: g.DocumentType, Role: g.Role, Pages: g.Pages, Key: f.Key, Value: f.Value}
								replaced := false
								for index, old := range source.PreservedCorrections {
									if old.DocumentType == entry.DocumentType && old.Role == entry.Role && old.Key == entry.Key && samePages(old.Pages, entry.Pages) {
										source.PreservedCorrections[index] = entry
										replaced = true
									}
								}
								if !replaced {
									source.PreservedCorrections = append(source.PreservedCorrections, entry)
								}
								if err := r.Save(ctx, &source); err != nil {
									return err
								}
								break
							}
						}
						break
					}
				}
				return r.Create(ctx, &models.FieldCorrection{CompanyID: a.CompanyID, DossierID: id, FieldID: f.ID, ActorID: a.UserID, Before: before, After: f.Value, Reason: in.Reason, Note: in.Note})
			}
		}
		return common.ErrNotFound
	})
}
func (l *DossierLogic) Acknowledge(ctx context.Context, id, issueID uuid.UUID, in dto.ReasonRequest) error {
	if strings.TrimSpace(in.Reason) == "" {
		return common.ErrInvalidArgument
	}
	return l.mutate(ctx, id, "dossier:review", in.ExpectedVersion, "WARNING_ACKNOWLEDGED", map[string]any{"issue_id": issueID}, func(r *repositories.DossierRepository, b *repositories.Bundle, a common.Actor) error {
		for _, i := range b.Issues {
			if i.ID == issueID {
				if i.Severity != "WARNING" || i.Status != "OPEN" {
					return common.ErrConflict
				}
				i.Status = "ACKNOWLEDGED"
				i.AcknowledgedBy = &a.UserID
				i.Acknowledgement = in.Reason
				return r.Save(ctx, &i)
			}
		}
		return common.ErrNotFound
	})
}
func requireVersion(version uint64) error {
	if version == 0 {
		return fmt.Errorf("%w: expected_version is required", common.ErrInvalidArgument)
	}
	return nil
}

func mutationMetadata(metadata map[string]any, version uint64) map[string]any {
	out := map[string]any{"version": version}
	for k, v := range metadata {
		out[k] = v
	}
	return out
}
