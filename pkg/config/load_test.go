package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sea-codex-server/pkg/config"
)

const fixture = `http_server:
  port: 8081
  mode: release
  log_level: info
postgres:
  host: 127.0.0.1
  user: fixture-user
  password: fixture-password-must-not-leak
  dbname: fixture-db
  sslmode: disable
auth:
  private_key_hex: fixture-private-key-must-not-leak
  public_key_hex: fixture-public-key
  token_duration: 2h
`

func clearConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"HTTP_SERVER_HOST", "HTTP_SERVER_PORT", "HTTP_SERVER_MODE", "HTTP_SERVER_LOG_LEVEL",
		"HTTP_SERVER_READ_TIMEOUT", "HTTP_SERVER_READ_HEADER_TIMEOUT", "HTTP_SERVER_WRITE_TIMEOUT",
		"HTTP_SERVER_IDLE_TIMEOUT", "HTTP_SERVER_SHUTDOWN_TIMEOUT",
		"POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DBNAME",
		"POSTGRES_SSLMODE", "POSTGRES_CONNECT_TIMEOUT", "POSTGRES_MAX_OPEN_CONNS",
		"POSTGRES_MAX_IDLE_CONNS", "POSTGRES_CONN_MAX_LIFETIME",
		"AUTH_PRIVATE_KEY_HEX", "AUTH_PUBLIC_KEY_HEX", "AUTH_TOKEN_DURATION",
	}
	for _, key := range keys {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestLoadConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		port int
	}{
		{name: "YAML and defaults", port: 8081},
		{name: "env overrides YAML", env: map[string]string{"HTTP_SERVER_PORT": "9090"}, port: 9090},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			cfg, err := config.LoadConfig(writeConfig(t, fixture))
			require.NoError(t, err)
			assert.Equal(t, tc.port, cfg.HTTPServer.Port)
			assert.Equal(t, "127.0.0.1", cfg.HTTPServer.Host)
			assert.Equal(t, "fixture-db", cfg.Postgres.DBName)
			assert.Equal(t, 2*time.Hour, cfg.Auth.TokenDuration)
			assert.Equal(t, 5*time.Second, cfg.Postgres.ConnectTimeout)
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		content string
	}{
		{name: "invalid port", env: map[string]string{"HTTP_SERVER_PORT": "0"}},
		{name: "port overflow", env: map[string]string{"HTTP_SERVER_PORT": "65536"}},
		{name: "invalid timeout", env: map[string]string{"HTTP_SERVER_READ_TIMEOUT": "-1s"}},
		{name: "invalid mode", env: map[string]string{"HTTP_SERVER_MODE": "unsafe"}},
		{name: "invalid level", env: map[string]string{"HTTP_SERVER_LOG_LEVEL": "fixture-password-must-not-leak"}},
		{name: "missing DB password", env: map[string]string{"POSTGRES_PASSWORD": ""}},
		{name: "invalid DB port", env: map[string]string{"POSTGRES_PORT": "0"}},
		{name: "invalid TLS", env: map[string]string{"POSTGRES_SSLMODE": "unknown"}},
		{name: "invalid pool", env: map[string]string{"POSTGRES_MAX_OPEN_CONNS": "1", "POSTGRES_MAX_IDLE_CONNS": "2"}},
		{name: "missing signing key", env: map[string]string{"AUTH_PRIVATE_KEY_HEX": ""}},
		{name: "invalid lifetime", env: map[string]string{"AUTH_TOKEN_DURATION": "0s"}},
		{name: "subsecond lifetime", env: map[string]string{"AUTH_TOKEN_DURATION": "10ms"}},
		{name: "invalid scalar type", env: map[string]string{"POSTGRES_PORT": "fixture-password-must-not-leak"}},
		{name: "invalid YAML", content: "auth: [fixture-private-key-must-not-leak"},
		{name: "unknown key", content: fixture + "unknown_setting: fixture-password-must-not-leak\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			content := tc.content
			if content == "" {
				content = fixture
			}
			_, err := config.LoadConfig(writeConfig(t, content))
			require.ErrorIs(t, err, config.ErrInvalidConfiguration)
			assert.NotContains(t, err.Error(), "fixture-password-must-not-leak")
			assert.NotContains(t, err.Error(), "fixture-private-key-must-not-leak")
		})
	}
}

func TestMissingConfigFile(t *testing.T) {
	clearConfigEnv(t)
	_, err := config.LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorIs(t, err, config.ErrInvalidConfiguration)
}

func TestHTTPAddress(t *testing.T) {
	assert.Equal(t, "[::1]:8080", (config.HTTPServerConfig{Host: "::1", Port: 8080}).Address())
}
