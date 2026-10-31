package migrations

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"sea-codex-server/internal/audit/models"
)

func MigrateAuditModels(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&models.AuditLog{}); err != nil {
		return fmt.Errorf("migrate audit: %w", err)
	}
	return nil
}
