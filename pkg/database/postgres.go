// Package database initializes PostgreSQL connections without running migrations.
package database

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"sea-codex-server/pkg/config"
)

func InitPostgresDB(ctx context.Context, cfg config.PostgresConfig) (*gorm.DB, error) {
	dsn := url.URL{
		Scheme: "postgres", User: url.UserPassword(cfg.User, cfg.Password),
		Host: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), Path: "/" + cfg.DBName,
	}
	query := dsn.Query()
	query.Set("sslmode", cfg.SSLMode)
	dsn.RawQuery = query.Encode()
	pgConfig, err := pgx.ParseConfig(dsn.String())
	if err != nil {
		// pgx parse errors may include the DSN; do not propagate credentials.
		return nil, fmt.Errorf("%w: PostgreSQL connection settings", config.ErrInvalidConfiguration)
	}
	pgConfig.ConnectTimeout = cfg.ConnectTimeout
	sqlDB := stdlib.OpenDB(*pgConfig)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing: true, TranslateError: true, Logger: gormlogger.Discard,
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("initialize GORM: %w", err)
	}
	return db, nil
}
