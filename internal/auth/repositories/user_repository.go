package repositories

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"sea-codex-server/internal/auth/models"
	"sea-codex-server/internal/common"
)

type IUserRepository interface {
	Create(context.Context, *gorm.DB, *models.User) error
	FindLogin(context.Context, string) (models.User, error)
	FindIdentity(context.Context, uuid.UUID) (models.User, error)
	FindInCompany(context.Context, *gorm.DB, uuid.UUID, uuid.UUID) (models.User, error)
	List(context.Context, uuid.UUID, common.Page) (common.List[models.User], error)
}
type UserRepository struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }
func (r *UserRepository) Create(ctx context.Context, tx *gorm.DB, u *models.User) error {
	if err := tx.WithContext(ctx).Omit("Company").Create(u).Error; err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// FindLogin/FindIdentity are authentication bootstrap lookups, never exposed as arbitrary user APIs.
func (r *UserRepository) FindLogin(ctx context.Context, identifier string) (models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).Preload("Company").Where("username = ? OR email = ?", identifier, identifier).First(&u).Error
	return u, err
}
func (r *UserRepository) FindIdentity(ctx context.Context, id uuid.UUID) (models.User, error) {
	var u models.User
	err := r.db.WithContext(ctx).Preload("Company").First(&u, "id = ?", id).Error
	return u, err
}
func (r *UserRepository) FindInCompany(ctx context.Context, tx *gorm.DB, company, id uuid.UUID) (models.User, error) {
	var u models.User
	if company == uuid.Nil {
		return u, common.ErrForbidden
	}
	if tx == nil {
		tx = r.db
	}
	err := tx.WithContext(ctx).Where("company_id = ? AND id = ?", company, id).First(&u).Error
	return u, err
}
func (r *UserRepository) List(ctx context.Context, company uuid.UUID, p common.Page) (common.List[models.User], error) {
	out := common.List[models.User]{Items: []models.User{}, Page: p}
	if company == uuid.Nil {
		return out, common.ErrForbidden
	}
	q := r.db.WithContext(ctx).Model(&models.User{}).Where("company_id = ?", company)
	if err := q.Count(&out.Total).Error; err != nil {
		return out, fmt.Errorf("count users: %w", err)
	}
	err := q.Order("created_at DESC, id DESC").Limit(p.Limit).Offset(p.Offset).Find(&out.Items).Error
	return out, err
}
