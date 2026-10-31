package migrations

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"sea-codex-server/internal/dossier/models"
)

func MigrateDossierModels(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&models.DocumentType{}, &models.DocumentTypeVersion{}, &models.DossierType{}, &models.DossierTypeVersion{}, &models.Dossier{}, &models.Requirement{}, &models.File{}, &models.Group{}, &models.ExtractedField{}, &models.FieldCorrection{}, &models.SpecialCheck{}, &models.Issue{}, &models.Decision{}, &models.StatusHistory{}); err != nil {
		return fmt.Errorf("migrate dossier: %w", err)
	}
	return nil
}
