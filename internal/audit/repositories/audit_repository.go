package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"sea-codex-server/internal/audit/dto"
	"sea-codex-server/internal/audit/models"
	"sea-codex-server/internal/common"
)

type IAuditRepository interface {
	Create(context.Context, *gorm.DB, *models.AuditLog) error
	List(context.Context, uuid.UUID, dto.Filter) (common.List[models.AuditLog], error)
}
type AuditRepository struct{ db *gorm.DB }

func NewAuditRepository(db *gorm.DB) *AuditRepository { return &AuditRepository{db: db} }

func (r *AuditRepository) Create(ctx context.Context, tx *gorm.DB, event *models.AuditLog) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.WithContext(ctx).Create(event).Error; err != nil {
		return fmt.Errorf("write audit: %w", err)
	}
	return nil
}
func (r *AuditRepository) List(ctx context.Context, company uuid.UUID, f dto.Filter) (common.List[models.AuditLog], error) {
	out := common.List[models.AuditLog]{Items: []models.AuditLog{}, Page: f.Page}
	if company == uuid.Nil {
		return out, common.ErrForbidden
	}
	q := r.db.WithContext(ctx).Model(&models.AuditLog{}).Where("company_id = ?", company)
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.DossierID != nil {
		q = q.Where("dossier_id = ?", *f.DossierID)
	}
	if f.ResourceID != nil {
		q = q.Where("resource_id = ?", *f.ResourceID)
	}
	if f.ActorID != nil {
		q = q.Where("actor_id = ?", *f.ActorID)
	}
	if f.From != nil {
		q = q.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		q = q.Where("created_at < ?", *f.To)
	}
	if err := q.Count(&out.Total).Error; err != nil {
		return out, fmt.Errorf("count audit: %w", err)
	}
	if err := q.Order("created_at DESC, id DESC").Limit(f.Limit).Offset(f.Offset).Find(&out.Items).Error; err != nil {
		return out, fmt.Errorf("list audit: %w", err)
	}
	return out, nil
}
