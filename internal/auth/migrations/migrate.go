package migrations

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"sea-codex-server/internal/auth/models"
)

func MigrateAuthModels(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&models.Company{}, &models.User{}); err != nil {
		return fmt.Errorf("migrate auth: %w", err)
	}
	return nil
}
