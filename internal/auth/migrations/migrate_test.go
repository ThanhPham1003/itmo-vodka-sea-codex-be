package migrations_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"sea-codex-server/internal/auth/migrations"
	"sea-codex-server/internal/auth/models"
	base "sea-codex-server/internal/common/models"
)

func migrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	var dialect gorm.Dialector = sqlite.Open("file:" + uuid.NewString() + "?mode=memory&cache=shared&_foreign_keys=1")
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
		require.NoError(t, err)
		pool, err := root.DB()
		require.NoError(t, err)
		schema := "test_auth_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
		t.Cleanup(func() {
			require.NoError(t, root.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
			require.NoError(t, pool.Close())
		})
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		q := parsed.Query()
		q.Set("search_path", schema)
		parsed.RawQuery = q.Encode()
		dialect = postgres.Open(parsed.String())
	}
	db, err := gorm.Open(dialect, &gorm.Config{TranslateError: true, Logger: logger.Discard})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	return db
}

type legacyCompany struct {
	base.BaseModel
	Name     string `gorm:"not null;size:200"`
	Active   *bool
	IsActive *bool
}

func (legacyCompany) TableName() string { return "companies" }

type legacyUser struct {
	base.BaseModel
	CompanyID    uuid.UUID `gorm:"type:uuid;not null"`
	Username     string    `gorm:"not null;size:64"`
	Email        string    `gorm:"not null;size:254"`
	PasswordHash string    `gorm:"not null;size:100"`
	FullName     string    `gorm:"not null;size:200"`
	Role         string    `gorm:"not null;size:50"`
	Permissions  []string  `gorm:"serializer:json;type:jsonb"`
	Active       *bool
	IsActive     *bool
}

func (legacyUser) TableName() string { return "users" }

func TestUpgradeActiveColumns(t *testing.T) {
	for _, tc := range []struct {
		name            string
		missing, legacy bool
	}{
		{name: "existing nullable column"}, {name: "missing column", missing: true},
		{name: "nullable column with legacy status", legacy: true}, {name: "missing column with legacy status", missing: true, legacy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := migrationDB(t)
			ctx := context.Background()
			require.NoError(t, db.AutoMigrate(&legacyCompany{}, &legacyUser{}))
			yes, no := true, false
			companies := []legacyCompany{{Name: "Known enabled", Active: &yes, IsActive: &no}, {Name: "Known disabled", Active: &no, IsActive: &yes}, {Name: "Legacy enabled", IsActive: &yes}, {Name: "Legacy disabled", IsActive: &no}, {Name: "Unknown"}, {Name: "Soft deleted"}}
			users := make([]legacyUser, len(companies))
			for i := range companies {
				require.NoError(t, db.Create(&companies[i]).Error)
				users[i] = legacyUser{CompanyID: companies[i].ID, Username: "fixture-" + companies[i].ID.String(), Email: companies[i].ID.String() + "@example.test", PasswordHash: "synthetic-unused-hash", FullName: "Synthetic User", Role: "broker", Active: companies[i].Active, IsActive: companies[i].IsActive}
				require.NoError(t, db.Create(&users[i]).Error)
			}
			require.NoError(t, db.Delete(&companies[5]).Error)
			require.NoError(t, db.Delete(&users[5]).Error)
			for _, model := range []any{&legacyCompany{}, &legacyUser{}} {
				if tc.missing {
					require.NoError(t, db.Migrator().DropColumn(model, "active"))
				}
				if !tc.legacy {
					require.NoError(t, db.Migrator().DropColumn(model, "is_active"))
				}
			}
			require.NoError(t, migrations.MigrateAuthModels(ctx, db))
			for i := range companies {
				expected := false
				if !tc.missing && companies[i].Active != nil {
					expected = *companies[i].Active
				} else if tc.legacy && companies[i].IsActive != nil {
					expected = *companies[i].IsActive
				}
				for table, id := range map[string]uuid.UUID{"companies": companies[i].ID, "users": users[i].ID} {
					var active sql.NullBool
					require.NoError(t, db.Table(table).Select("active").Where("id = ?", id).Row().Scan(&active))
					assert.True(t, active.Valid)
					assert.Equal(t, expected, active.Bool, "%s row %d", table, i)
				}
			}
			// A repeated migration must not copy stale is_active over an authoritative active value.
			require.NoError(t, db.Table("companies").Where("id = ?", companies[2].ID).UpdateColumn("active", false).Error)
			require.NoError(t, migrations.MigrateAuthModels(ctx, db))
			var company models.Company
			require.NoError(t, db.Unscoped().First(&company, "id = ?", companies[2].ID).Error)
			assert.False(t, company.Active)
			// Database constraints also cover soft-deleted records.
			for table, id := range map[string]uuid.UUID{"companies": companies[5].ID, "users": users[5].ID} {
				require.Error(t, db.Table(table).Where("id = ?", id).UpdateColumn("active", nil).Error)
			}
		})
	}
}

func TestFreshMigrationPreservesExplicitDisabledInsert(t *testing.T) {
	db := migrationDB(t)
	require.NoError(t, migrations.MigrateAuthModels(context.Background(), db))
	company := models.Company{Name: "Disabled", Active: false}
	require.NoError(t, db.Create(&company).Error)
	user := models.User{CompanyID: company.ID, Username: "disabled-user", Email: "disabled@example.test", PasswordHash: "unused", FullName: "Disabled User", Role: "broker", Active: false}
	require.NoError(t, db.Create(&user).Error)
	var actualCompany models.Company
	var actualUser models.User
	require.NoError(t, db.First(&actualCompany, "id = ?", company.ID).Error)
	require.NoError(t, db.First(&actualUser, "id = ?", user.ID).Error)
	assert.False(t, actualCompany.Active)
	assert.False(t, actualUser.Active)
}

func TestMigrationRollsBackBackfillOnLaterFailure(t *testing.T) {
	db := migrationDB(t)
	require.NoError(t, db.AutoMigrate(&legacyCompany{}, &legacyUser{}))
	company := legacyCompany{Name: "Rollback fixture"}
	require.NoError(t, db.Create(&company).Error)
	for _, email := range []string{"one@example.test", "two@example.test"} {
		user := legacyUser{CompanyID: company.ID, Username: "duplicate", Email: email, PasswordHash: "unused", FullName: "Synthetic", Role: "broker"}
		require.NoError(t, db.Create(&user).Error)
	}
	// Creating the new unique username index must fail; the preceding backfill must roll back too.
	require.Error(t, migrations.MigrateAuthModels(context.Background(), db))
	var active sql.NullBool
	require.NoError(t, db.Table("companies").Select("active").Where("id = ?", company.ID).Row().Scan(&active))
	assert.False(t, active.Valid)
}

func TestPostgresRecoveryScript(t *testing.T) {
	if os.Getenv("TEST_POSTGRES_DSN") == "" {
		t.Skip("recovery SQL requires PostgreSQL; use a dedicated test database")
	}
	db := migrationDB(t)
	require.NoError(t, db.AutoMigrate(&legacyCompany{}, &legacyUser{}))
	yes, no := true, false
	company := legacyCompany{Name: "Legacy enabled", IsActive: &yes}
	require.NoError(t, db.Create(&company).Error)
	user := legacyUser{CompanyID: company.ID, Username: "recovery-user", Email: "recovery@example.test", FullName: "Synthetic", PasswordHash: "unused", Role: "broker", Active: &no, IsActive: &yes}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Migrator().DropColumn(&legacyCompany{}, "Active"))
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "db", "20261031_auth_active.sql"))
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		require.NoError(t, db.Exec(string(raw)).Error)
		for _, fixture := range []struct {
			table  string
			id     uuid.UUID
			active bool
		}{{"companies", company.ID, true}, {"users", user.ID, false}} {
			var active bool
			require.NoError(t, db.Table(fixture.table).Select("active").Where("id = ?", fixture.id).Row().Scan(&active))
			assert.Equal(t, fixture.active, active)
			assert.Error(t, db.Table(fixture.table).Where("id = ?", fixture.id).UpdateColumn("active", nil).Error)
		}
	}
}
