package logic

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"sea-codex-server/internal/audit/dto"
	"sea-codex-server/internal/audit/models"
	"sea-codex-server/internal/audit/repositories"
	"sea-codex-server/internal/common"
)

type AuditLogic struct{ repo repositories.IAuditRepository }

func NewAuditLogic(repo repositories.IAuditRepository) *AuditLogic { return &AuditLogic{repo: repo} }
func (l *AuditLogic) Write(ctx context.Context, tx *gorm.DB, e common.AuditEvent) error {
	if e.DossierID == nil && e.Resource == "dossier" {
		e.DossierID = &e.ResourceID
	}
	if e.DossierID == nil && e.Metadata != nil {
		if id, ok := e.Metadata["dossier_id"].(uuid.UUID); ok {
			e.DossierID = &id
		}
	}
	return l.repo.Create(ctx, tx, &models.AuditLog{CompanyID: e.CompanyID, ActorID: e.ActorID, Action: e.Action, Resource: e.Resource, ResourceID: e.ResourceID, DossierID: e.DossierID, Metadata: e.Metadata})
}
func (l *AuditLogic) List(ctx context.Context, f dto.Filter) (common.List[models.AuditLog], error) {
	a, ok := common.ActorFromContext(ctx)
	if !ok {
		return common.List[models.AuditLog]{}, common.ErrUnauthenticated
	}
	if !a.HasPermission("audit:read") {
		return common.List[models.AuditLog]{}, common.ErrForbidden
	}
	result, err := l.repo.List(ctx, a.CompanyID, f)
	if err != nil {
		return result, fmt.Errorf("%w: audit query: %w", common.ErrUnavailable, err)
	}
	return result, nil
}
