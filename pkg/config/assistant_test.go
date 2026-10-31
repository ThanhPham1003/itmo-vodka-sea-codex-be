package config_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"sea-codex-server/pkg/config"
	"testing"
	"time"
)

func TestAssistantConfigIsolationAndValidation(t *testing.T) {
	clearConfigEnv(t)
	for _, key := range []string{"ASSISTANT_ENABLED", "ASSISTANT_BASE_URL", "ASSISTANT_API_KEY", "ASSISTANT_MODEL", "ASSISTANT_TIMEOUT", "ASSISTANT_MAX_OUTPUT_TOKENS", "ASSISTANT_REQUEST_LIMIT", "ASSISTANT_RATE_WINDOW", "ASSISTANT_MAX_CONCURRENT_REQUESTS"} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}
	path := filepath.Join(t.TempDir(), "app.yaml")
	require.NoError(t, os.WriteFile(path, []byte(fixture), 0600))
	cfg, err := config.LoadConfig(path)
	require.NoError(t, err)
	assert.False(t, cfg.Assistant.Enabled)
	assert.Equal(t, 1500, cfg.Assistant.MaxOutputTokens)
	t.Setenv("ASSISTANT_ENABLED", "true")
	t.Setenv("ASSISTANT_API_KEY", "assistant-secret")
	t.Setenv("ASSISTANT_MODEL", "test-model")
	cfg, err = config.LoadConfig(path)
	require.NoError(t, err)
	assert.True(t, cfg.Assistant.Enabled)
	assert.False(t, cfg.Processing.Enabled)
	assert.Empty(t, cfg.Processing.APIKey)
	for _, tc := range []struct {
		name   string
		change func(*config.AssistantConfig)
	}{
		{"HTTP", func(c *config.AssistantConfig) { c.BaseURL = "http://api.openai.com" }},
		{"host", func(c *config.AssistantConfig) { c.BaseURL = "https://attacker.test" }},
		{"userinfo", func(c *config.AssistantConfig) { c.BaseURL = "https://user:secret@api.openai.com" }},
		{"path", func(c *config.AssistantConfig) { c.BaseURL = "https://api.openai.com/other" }},
		{"missing model", func(c *config.AssistantConfig) { c.Model = " " }},
		{"timeout", func(c *config.AssistantConfig) { c.Timeout = 61 * time.Second }},
		{"budget", func(c *config.AssistantConfig) { c.MaxOutputTokens = 100000 }},
		{"quota", func(c *config.AssistantConfig) { c.RequestLimit = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := cfg.Assistant
			tc.change(&copy)
			err := copy.Validate()
			require.ErrorIs(t, err, config.ErrInvalidConfiguration)
			assert.NotContains(t, err.Error(), "assistant-secret")
		})
	}
}
