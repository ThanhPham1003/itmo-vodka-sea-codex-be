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
	err = l.repo.Scope(a.CompanyID).Tx(ctx, func(r *repositories.DossierRepository) error {
		b, err := r.Bundle(ctx, id, true)
		if err != nil {
			return err
		}
		if !canRead(a, b.Dossier) {
			return common.ErrNotFound
		}
		if !editable(b.Dossier) || b.Dossier.Version != in.ExpectedVersion {
			return common.ErrConflict
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
				return common.ErrConflict
			}
			for _, f := range b.Fields {
				if f.Required && (f.Value == "" || f.Validation != "VALID") {
					return common.ErrConflict
				}
			}
			for _, i := range b.Issues {
				if i.Status == "PENDING_DOCUMENTS" || i.Status == "OPEN" && (i.Severity == "WARNING" || i.Severity == "BLOCKER") {
					return common.ErrConflict
				}
			}
			status = models.Approved
		case "REQUEST_CHANGES":
			b.Dossier.ChangeRequest = in.Reason
			status = models.Invalid
		case "REJECT":
			if b.Dossier.Status == models.Processing {
				return common.ErrConflict
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
		return l.audit.Write(ctx, r.DB(), common.AuditEvent{CompanyID: a.CompanyID, ActorID: a.UserID, Action: "DOSSIER_" + in.Action, Resource: "dossier", ResourceID: id, Metadata: map[string]any{"decision_id": decision.ID, "version": decision.Version}})
	})
	return decision, repositories.Error(err)
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
