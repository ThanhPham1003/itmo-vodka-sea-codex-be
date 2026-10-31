package migrations

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"sea-codex-server/internal/auth/models"
)

func MigrateAuthModels(ctx context.Context, db *gorm.DB) error {
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range []string{"companies", "users"} {
			if err := backfillActive(tx, table); err != nil {
				return err
			}
		}
		for _, model := range []any{&models.Company{}, &models.User{}} {
			if err := enforceActiveNotNull(tx, model); err != nil {
				return err
			}
		}
		return tx.AutoMigrate(&models.Company{}, &models.User{})
	})
	if err != nil {
		return fmt.Errorf("migrate auth: %w", err)
	}
	return nil
}
