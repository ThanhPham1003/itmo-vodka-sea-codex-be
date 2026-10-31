package bootstrap_test

import (
	paseto "aidanwoods.dev/go-paseto"
	"context"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	am "sea-codex-server/internal/audit/migrations"
	um "sea-codex-server/internal/auth/migrations"
	"sea-codex-server/internal/common"
	dm "sea-codex-server/internal/dossier/migrations"
	"sea-codex-server/pkg/config"
	"sea-codex-server/pkg/token"
	"strings"
	"testing"
	"time"
)

func newTestApp(t *testing.T) *common.App {
	t.Helper()
	var dialect gorm.Dialector = sqlite.Open("file:" + uuid.NewString() + "?mode=memory&cache=shared&_foreign_keys=1")
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		// Every test app receives its own schema; no existing application tables are touched.
		root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
		require.NoError(t, err)
		schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		require.NoError(t, root.Exec("CREATE SCHEMA "+schema).Error)
		pool, err := root.DB()
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, root.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
			require.NoError(t, pool.Close())
		})
		parsed, err := url.Parse(dsn)
		require.NoError(t, err)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		dialect = postgres.Open(parsed.String())
	}
	db, err := gorm.Open(dialect, &gorm.Config{TranslateError: true, Logger: logger.Discard})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	ctx := context.Background()
	require.NoError(t, um.MigrateAuthModels(ctx, db))
	require.NoError(t, am.MigrateAuditModels(ctx, db))
	require.NoError(t, dm.MigrateDossierModels(ctx, db))
	key := paseto.NewV4AsymmetricSecretKey()
	manager, err := token.NewManager(key.ExportHex(), key.Public().ExportHex(), time.Hour)
	require.NoError(t, err)
	cfg := &config.Config{HTTPServer: config.HTTPServerConfig{Mode: "test"}, Auth: config.AuthConfig{RequestLimit: 1000, RateWindow: time.Minute}, Uploads: config.UploadsConfig{Directory: t.TempDir(), MaxFileSizeMB: 20, MaxFilesPerRequest: 10, MaxPDFPages: 100}, Processing: config.ProcessingConfig{Provider: "openai-compatible", Timeout: time.Second, PollInterval: 10 * time.Millisecond, LeaseDuration: time.Minute, MaxConcurrentFiles: 2, MaxPageDimension: 800}}
	return &common.App{Config: cfg, DB: db, TokenManager: manager, Logger: zerolog.Nop()}
}
