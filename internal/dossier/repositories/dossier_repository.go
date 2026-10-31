package repositories

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/dto"
	"sea-codex-server/internal/dossier/models"
)

type Bundle struct {
	Dossier      models.Dossier
	Requirements []models.Requirement
	Files        []models.File
	Groups       []models.Group
	Fields       []models.ExtractedField
	Issues       []models.Issue
	Decisions    []models.Decision
	Checks       []models.SpecialCheck
}
type DossierRepository struct {
	db      *gorm.DB
	company uuid.UUID
}

func NewDossierRepository(db *gorm.DB) *DossierRepository { return &DossierRepository{db: db} }
func (r *DossierRepository) Scope(company uuid.UUID) *DossierRepository {
	return &DossierRepository{db: r.db, company: company}
}
func (r *DossierRepository) Tx(ctx context.Context, fn func(*DossierRepository) error) error {
	if r.company == uuid.Nil {
		return common.ErrForbidden
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&DossierRepository{db: tx, company: r.company}) })
}
func (r *DossierRepository) DB() *gorm.DB { return r.db } // Only passed to the shared audit writer to join the active transaction.
func (r *DossierRepository) query(ctx context.Context, model any) *gorm.DB {
	q := r.db.WithContext(ctx).Model(model).Where("company_id = ?", r.company)
	if r.company == uuid.Nil {
		q = q.Where("1 = 0")
	}
	return q
}
func (r *DossierRepository) Create(ctx context.Context, entity any) error {
	v := reflect.Indirect(reflect.ValueOf(entity))
	field := v.FieldByName("CompanyID")
	if r.company == uuid.Nil || !field.IsValid() || field.Interface() != r.company {
		return common.ErrForbidden
	}
	return r.db.WithContext(ctx).Create(entity).Error
}
func (r *DossierRepository) Save(ctx context.Context, entity any) error {
	v := reflect.Indirect(reflect.ValueOf(entity))
	field := v.FieldByName("CompanyID")
	id := v.FieldByName("ID")
	if r.company == uuid.Nil || !field.IsValid() || field.Interface() != r.company || !id.IsValid() {
		return common.ErrForbidden
	}
	result := r.query(ctx, entity).Where("id = ?", id.Interface()).Select("*").Omit("id", "created_at").Updates(entity)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *DossierRepository) DocumentType(ctx context.Context, code string, lock bool) (models.DocumentType, error) {
	var out models.DocumentType
	q := r.query(ctx, &out).Where("code = ?", code)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	err := q.First(&out).Error
	return out, err
}
func (r *DossierRepository) DocumentTypes(ctx context.Context, p common.Page) (common.List[models.DocumentType], error) {
	out := common.List[models.DocumentType]{Items: []models.DocumentType{}, Page: p}
	q := r.query(ctx, &models.DocumentType{})
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	err := q.Order("created_at DESC, id DESC").Limit(p.Limit).Offset(p.Offset).Find(&out.Items).Error
	return out, err
}
func (r *DossierRepository) DocumentVersion(ctx context.Context, id uuid.UUID, version int) (models.DocumentTypeVersion, error) {
	var out models.DocumentTypeVersion
	err := r.query(ctx, &out).Where("document_type_id = ? AND version = ?", id, version).First(&out).Error
	return out, err
}
func (r *DossierRepository) DossierType(ctx context.Context, code string, lock bool) (models.DossierType, error) {
	var out models.DossierType
	q := r.query(ctx, &out).Where("code = ?", code)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.First(&out).Error
	return out, err
}
func (r *DossierRepository) DossierTypes(ctx context.Context, p common.Page) (common.List[models.DossierType], error) {
	out := common.List[models.DossierType]{Items: []models.DossierType{}, Page: p}
	q := r.query(ctx, &models.DossierType{})
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	err := q.Order("created_at DESC, id DESC").Limit(p.Limit).Offset(p.Offset).Find(&out.Items).Error
	return out, err
}
func (r *DossierRepository) Find(ctx context.Context, id uuid.UUID, lock bool) (models.Dossier, error) {
	var d models.Dossier
	q := r.query(ctx, &d).Where("id = ?", id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := q.First(&d).Error
	return d, err
}
func (r *DossierRepository) Bundle(ctx context.Context, id uuid.UUID, lock bool) (Bundle, error) {
	d, err := r.Find(ctx, id, lock)
	out := Bundle{Dossier: d, Requirements: []models.Requirement{}, Files: []models.File{}, Groups: []models.Group{}, Fields: []models.ExtractedField{}, Issues: []models.Issue{}, Decisions: []models.Decision{}}
	if err != nil {
		return out, err
	}
	for _, target := range []any{&out.Requirements, &out.Files, &out.Groups, &out.Fields, &out.Issues, &out.Decisions, &out.Checks} {
		if err = r.query(ctx, target).Where("dossier_id = ?", id).Order("created_at ASC, id ASC").Find(target).Error; err != nil {
			return out, err
		}
	}
	return out, nil
}
func (r *DossierRepository) List(ctx context.Context, a common.Actor, f dto.DossierFilter) (common.List[models.Dossier], error) {
	out := common.List[models.Dossier]{Items: []models.Dossier{}, Page: f.Page}
	q := r.query(ctx, &models.Dossier{})
	if !a.HasPermission("dossier:read_all") {
		q = q.Where("creator_id = ? OR assignee_id = ?", a.UserID, a.UserID)
	}
	if f.Status != "" && f.HistoryFrom == nil && f.HistoryTo == nil {
		q = q.Where("status = ?", f.Status)
	}
	if f.Type != "" {
		q = q.Where("type_code = ?", f.Type)
	}
	if f.Search != "" {
		q = q.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(f.Search)+"%")
	}
	if f.CreatorID != nil {
		q = q.Where("creator_id = ?", *f.CreatorID)
	}
	if f.AssigneeID != nil {
		q = q.Where("assignee_id = ?", *f.AssigneeID)
	}
	if f.HistoryFrom != nil || f.HistoryTo != nil {
		sub := r.query(ctx, &models.StatusHistory{}).Select("dossier_id")
		if f.Status != "" {
			sub = sub.Where("\"to\" = ?", f.Status)
		}
		if f.HistoryFrom != nil {
			sub = sub.Where("created_at >= ?", f.HistoryFrom)
		}
		if f.HistoryTo != nil {
			sub = sub.Where("created_at < ?", f.HistoryTo)
		}
		// Historical drill-down should not additionally filter by the current status.
		q = q.Where("id IN (?)", sub)
		if !a.HasPermission("dossier:read_all") {
			q = q.Where("creator_id = ? OR assignee_id = ?", a.UserID, a.UserID)
		}
		if f.Type != "" {
			q = q.Where("type_code = ?", f.Type)
		}
	}
	if err := q.Count(&out.Total).Error; err != nil {
		return out, err
	}
	order := "created_at DESC, id DESC"
	if f.Review {
		order = "priority DESC, created_at ASC, id ASC"
	}
	err := q.Order(order).Limit(f.Limit).Offset(f.Offset).Find(&out.Items).Error
	return out, err
}
func (r *DossierRepository) Duplicate(ctx context.Context, id uuid.UUID, sha string) (models.File, error) {
	var f models.File
	err := r.query(ctx, &f).Where("dossier_id = ? AND sha256 = ? AND source_file_id IS NULL", id, sha).First(&f).Error
	return f, err
}
func (r *DossierRepository) RemoveProcessingData(ctx context.Context, id, file uuid.UUID) error {
	var derived []models.File
	if err := r.query(ctx, &models.File{}).Where("dossier_id = ? AND source_file_id = ?", id, file).Find(&derived).Error; err != nil {
		return err
	}
	ids := []uuid.UUID{file}
	for _, f := range derived {
		ids = append(ids, f.ID)
	}
	for _, model := range []any{&models.Group{}, &models.ExtractedField{}, &models.SpecialCheck{}} {
		if err := r.query(ctx, model).Where("dossier_id = ? AND file_id IN ?", id, ids).Delete(model).Error; err != nil {
			return err
		}
	}
	return r.query(ctx, &models.File{}).Where("dossier_id = ? AND source_file_id = ?", id, file).Delete(&models.File{}).Error
}
func (r *DossierRepository) RetireIssue(ctx context.Context, id uuid.UUID) error {
	return r.query(ctx, &models.Issue{}).Where("id = ?", id).Delete(&models.Issue{}).Error
}
func (r *DossierRepository) History(ctx context.Context, from, to time.Time, typ string) ([]models.StatusHistory, error) {
	out := []models.StatusHistory{}
	q := r.query(ctx, &models.StatusHistory{}).Where("created_at >= ? AND created_at < ?", from, to)
	if typ != "" {
		sub := r.query(ctx, &models.Dossier{}).Select("id").Where("type_code = ?", typ)
		q = q.Where("dossier_id IN (?)", sub)
	}
	err := q.Order("created_at ASC, id ASC").Find(&out).Error
	return out, err
}
func (r *DossierRepository) CurrentStatuses(ctx context.Context, typ string) ([]models.Dossier, error) {
	out := []models.Dossier{}
	q := r.query(ctx, &models.Dossier{})
	if typ != "" {
		q = q.Where("type_code = ?", typ)
	}
	err := q.Select("id", "status").Find(&out).Error
	return out, err
}
func (r *DossierRepository) Claim(ctx context.Context, lease time.Duration) (models.File, error) {
	var file models.File
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		q := tx.Model(&models.File{}).Where("source_file_id IS NULL AND superseded_by IS NULL AND (status = ? OR (status = ? AND lease_until < ?))", "UPLOADED", "PROCESSING", now)
		q = q.Where("dossier_id IN (?)", tx.Model(&models.Dossier{}).Select("id").Where("status NOT IN ?", []string{models.Approved, models.Rejected}))
		if err := q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Order("created_at ASC").First(&file).Error; err != nil {
			return err
		}
		claim := uuid.New()
		until := now.Add(lease)
		updated := tx.Model(&models.File{}).Where("id = ? AND company_id = ? AND (status = ? OR (status = ? AND lease_until < ?))", file.ID, file.CompanyID, "UPLOADED", "PROCESSING", now).Updates(map[string]any{"status": "PROCESSING", "claim_id": claim, "lease_until": until, "attempt": gorm.Expr("attempt + 1"), "error_code": "", "error_message": ""})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		file.Status = "PROCESSING"
		file.ClaimID = &claim
		file.LeaseUntil = &until
		file.Attempt++
		return nil
	})
	return file, err
}
func (r *DossierRepository) Renew(ctx context.Context, file models.File, lease time.Duration) error {
	if file.ClaimID == nil {
		return common.ErrConflict
	}
	result := r.db.WithContext(ctx).Model(&models.File{}).Where("id = ? AND company_id = ? AND status = ? AND claim_id = ? AND superseded_by IS NULL", file.ID, file.CompanyID, "PROCESSING", *file.ClaimID).Update("lease_until", time.Now().UTC().Add(lease))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return common.ErrConflict
	}
	return nil
}
func Error(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.ErrNotFound
	}
	for _, e := range []error{common.ErrInvalidArgument, common.ErrUnauthenticated, common.ErrForbidden, common.ErrConflict, common.ErrNotFound, common.ErrUnavailable} {
		if errors.Is(err, e) {
			return err
		}
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return common.ErrConflict
	}
	return fmt.Errorf("%w: %w", common.ErrUnavailable, err)
}
