package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sea-codex-server/internal/auth/models"
)

type ICompanyRepository interface {
	Create(context.Context, *gorm.DB, *models.Company) error
	Lock(context.Context, *gorm.DB, uuid.UUID) error
}
type CompanyRepository struct{}

func NewCompanyRepository() *CompanyRepository { return &CompanyRepository{} }
func (*CompanyRepository) Create(ctx context.Context, tx *gorm.DB, c *models.Company) error {
	if err := tx.WithContext(ctx).Create(c).Error; err != nil {
		return fmt.Errorf("create company: %w", err)
	}
	return nil
}
func (*CompanyRepository) Lock(ctx context.Context, tx *gorm.DB, id uuid.UUID) error {
	var c models.Company
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&c, "id = ?", id).Error; err != nil {
		return fmt.Errorf("lock company: %w", err)
	}
	return nil
}
