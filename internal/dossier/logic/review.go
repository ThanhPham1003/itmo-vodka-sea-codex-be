package logic

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
)

func (l *DossierLogic) Review(ctx context.Context, id uuid.UUID, in dto.ReviewRequest) (models.Decision, error) {
	a, err := actor(ctx, "dossier:review")
	if err != nil {
		return models.Decision{}, err
	}
	if requireVersion(in.ExpectedVersion) != nil {
		return models.Decision{}, common.ErrInvalidArgument
	}
	if in.Action != "APPROVE" && in.Action != "REJECT" && in.Action != "REQUEST_CHANGES" {
		return models.Decision{}, common.ErrInvalidArgument
	}
	if in.Action != "APPROVE" && strings.TrimSpace(in.Reason) == "" {
		return models.Decision{}, common.ErrInvalidArgument
	}
	var decision models.Decision
	var acceptedFields []uuid.UUID
	err = l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		b, err := r.Bundle(ctx, id, true)
		if err != nil {
			return err
		}
		if !canRead(a, b.Dossier) {
			return common.ErrNotFound
		}
		if !editable(b.Dossier) {
			return l.reviewConflict(ctx, b.Dossier, in, "dossier_locked", 0)
		}
		if b.Dossier.Version != in.ExpectedVersion {
			return l.reviewConflict(ctx, b.Dossier, in, "version_mismatch", 0)
		}
		if a.Role != common.RoleCompanyAdmin && (b.Dossier.AssigneeID == nil || *b.Dossier.AssigneeID != a.UserID) {
			return common.ErrForbidden
		}
		if err = l.reconcile(ctx, r, &b, a.UserID); err != nil {
			return err
		}
		status := models.Rejected
		switch in.Action {
		case "APPROVE":
			if b.Dossier.Status != models.Valid {
				return l.reviewConflict(ctx, b.Dossier, in, "dossier_not_ready", 0)
			}
			invalidFields, accepted := requiredFieldReview(b)
			acceptedFields = accepted
			if invalidFields > 0 {
				return l.reviewConflict(ctx, b.Dossier, in, "invalid_required_fields", invalidFields)
			}
			for _, i := range b.Issues {
				if i.Status == "PENDING_DOCUMENTS" {
					return l.reviewConflict(ctx, b.Dossier, in, "pending_documents", 0)
				}
				if i.Status == "OPEN" && (i.Severity == "WARNING" || i.Severity == "BLOCKER") {
					return l.reviewConflict(ctx, b.Dossier, in, "unresolved_issues", 0)
				}
			}
			status = models.Approved
		case "REQUEST_CHANGES":
			b.Dossier.ChangeRequest = in.Reason
			status = models.Invalid
		case "REJECT":
			if b.Dossier.Status == models.Processing {
				return l.reviewConflict(ctx, b.Dossier, in, "processing_in_progress", 0)
			}
		}
		snapshot, err := json.Marshal(b)
		if err != nil {
			return err
		}
		decision = models.Decision{CompanyID: a.CompanyID, DossierID: id, ActorID: a.UserID, Action: in.Action, Reason: in.Reason, Version: b.Dossier.Version, Snapshot: string(snapshot)}
		if err = r.Create(ctx, &decision); err != nil {
			return err
		}
		if err = l.setStatus(ctx, r, &b.Dossier, status, a.UserID); err != nil {
			return err
		}
		b.Dossier.Version++
		if err = r.Save(ctx, &b.Dossier); err != nil {
			return err
		}
		metadata := map[string]any{"decision_id": decision.ID, "version": decision.Version}
		if len(acceptedFields) > 0 {
			metadata["accepted_required_fields"] = acceptedFields
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOSSIER_" + in.Action, Resource: "dossier", ResourceID: id, Metadata: metadata})
	})
	return decision, repositories.Error(err)
}

func (l *DossierLogic) reviewConflict(ctx context.Context, dossier models.Dossier, in dto.ReviewRequest, reason string, invalidFields int) error {
	l.log.Warn().Str("request_id", common.RequestIDFromContext(ctx)).
		Str("dossier_id", dossier.ID.String()).Str("decision_action", in.Action).
		Str("reason", reason).Str("dossier_status", dossier.Status).
		Uint64("expected_version", in.ExpectedVersion).Uint64("current_version", dossier.Version).
		Int("invalid_required_fields", invalidFields).Msg("dossier decision rejected")
	return common.ErrConflict
}

func (l *DossierLogic) Reopen(ctx context.Context, id uuid.UUID, in dto.ReasonRequest) error {
	a, err := actor(ctx, "dossier:reopen")
	if err != nil {
		return err
	}
	if in.ExpectedVersion == 0 || strings.TrimSpace(in.Reason) == "" {
		return common.ErrInvalidArgument
	}
	err = l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		b, err := r.Bundle(ctx, id, true)
		if err != nil {
			return err
		}
		if !canRead(a, b.Dossier) {
			return common.ErrNotFound
		}
		if b.Dossier.Status != models.Approved || b.Dossier.Version != in.ExpectedVersion {
			return common.ErrConflict
		}
		snapshot, err := json.Marshal(b)
		if err != nil {
			return err
		}
		decision := models.Decision{CompanyID: a.CompanyID, DossierID: id, ActorID: a.UserID, Action: "REOPEN", Reason: in.Reason, Version: b.Dossier.Version, Snapshot: string(snapshot)}
		if err = r.Create(ctx, &decision); err != nil {
			return err
		}
		b.Dossier.ChangeRequest = ""
		b.Dossier.Version++
		if err = l.setStatus(ctx, r, &b.Dossier, models.Processing, a.UserID); err != nil {
			return err
		}
		if err = l.reconcile(ctx, r, &b, a.UserID); err != nil {
			return err
		}
		if err = r.Save(ctx, &b.Dossier); err != nil {
			return err
		}
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOSSIER_REOPENED", Resource: "dossier", ResourceID: id, Metadata: map[string]any{"decision_id": decision.ID}})
	})
	return repositories.Error(err)
}
