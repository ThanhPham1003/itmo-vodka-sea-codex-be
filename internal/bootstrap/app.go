// Package bootstrap is the composition root for infrastructure and modules.
package bootstrap

import (
	"context"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"

	"sea-codex-server/internal/common"
	"sea-codex-server/pkg/config"
	"sea-codex-server/pkg/database"
	"sea-codex-server/pkg/logger"
	"sea-codex-server/pkg/token"
)

func NewApp(ctx context.Context, cfg *config.Config, output io.Writer) (*common.App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: configuration is required", config.ErrInvalidConfiguration)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("initialize application: %w", err)
	}
	log, err := logger.NewLogger(output, cfg.HTTPServer.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("initialize logger: %w", err)
	}
	// Reject invalid keys before opening a database connection.
	tokens, err := token.NewManager(cfg.Auth.PrivateKeyHex, cfg.Auth.PublicKeyHex, cfg.Auth.TokenDuration)
	if err != nil {
		return nil, fmt.Errorf("initialize token manager: %w", err)
	}
	db, err := database.InitPostgresDB(ctx, cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}
	gin.SetMode(cfg.HTTPServer.Mode)
	app := &common.App{Config: cfg, DB: db, Logger: log, TokenManager: tokens}
	if _, err := NewRouter(app); err != nil {
		_ = Close(app)
		return nil, fmt.Errorf("initialize router: %w", err)
	}
	return app, nil
}

func Close(app *common.App) error {
	if app == nil || app.DB == nil {
		return nil
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		return fmt.Errorf("get database pool: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close database pool: %w", err)
	}
	return nil
}
