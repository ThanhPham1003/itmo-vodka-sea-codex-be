package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/pkg/config"
	"sea-codex-server/pkg/database"
)

func TestCancelledConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	db, err := database.InitPostgresDB(ctx, config.PostgresConfig{
		Host: "127.0.0.1", Port: 5432, User: "test", Password: "not-to-be-logged",
		DBName: "test", SSLMode: "disable", ConnectTimeout: time.Second,
		MaxOpenConns: 1, MaxIdleConns: 1, ConnMaxLifetime: time.Minute,
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, db)
	assert.NotContains(t, err.Error(), "not-to-be-logged")
}

func TestInvalidConnectionDoesNotLeakCredentials(t *testing.T) {
	db, err := database.InitPostgresDB(context.Background(), config.PostgresConfig{
		Host: "127.0.0.1", Port: 5432, User: "test", Password: "not-to-be-logged",
		DBName: "test", SSLMode: "not-a-mode", ConnectTimeout: time.Second,
	})
	require.ErrorIs(t, err, config.ErrInvalidConfiguration)
	assert.Nil(t, db)
	assert.NotContains(t, err.Error(), "not-to-be-logged")
}
