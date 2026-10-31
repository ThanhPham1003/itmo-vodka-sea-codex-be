package models_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sea-codex-server/internal/common/models"
)

type fixtureModel struct {
	models.BaseModel
	Name string
}

func TestBaseModelPersistence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&fixtureModel{}))
	for _, tc := range []struct {
		name string
		id   uuid.UUID
	}{
		{name: "generate UUID"}, {name: "preserve explicit UUID", id: uuid.New()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := fixtureModel{BaseModel: models.BaseModel{ID: tc.id}, Name: tc.name}
			require.NoError(t, db.WithContext(context.Background()).Create(&item).Error)
			assert.NotEqual(t, uuid.Nil, item.ID)
			if tc.id != uuid.Nil {
				assert.Equal(t, tc.id, item.ID)
			}
			assert.False(t, item.CreatedAt.IsZero())
			assert.False(t, item.UpdatedAt.IsZero())
			require.NoError(t, db.Delete(&item).Error)
			var found fixtureModel
			assert.ErrorIs(t, db.First(&found, "id = ?", item.ID).Error, gorm.ErrRecordNotFound)
			require.NoError(t, db.Unscoped().First(&found, "id = ?", item.ID).Error)
			assert.True(t, found.DeletedAt.Valid)
		})
	}
}
