package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LoadConfig requires a YAML file; explicitly bound environment variables override it.
// It intentionally does not read .env files or use Viper's process-wide singleton.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AllowEmptyEnv(true)
	defaults := map[string]any{
		"http_server.host": "127.0.0.1", "http_server.port": 8080,
		"http_server.mode": "release", "http_server.log_level": "info",
		"http_server.read_timeout":        120 * time.Second,
		"http_server.read_header_timeout": 5 * time.Second,
		"http_server.write_timeout":       120 * time.Second,
		"http_server.idle_timeout":        60 * time.Second,
		"http_server.shutdown_timeout":    10 * time.Second,
		"postgres.host":                   "127.0.0.1", "postgres.port": 5432,
		"postgres.user": "", "postgres.password": "", "postgres.dbname": "",
		"postgres.sslmode": "require", "postgres.connect_timeout": 5 * time.Second,
		"postgres.max_open_conns": 10, "postgres.max_idle_conns": 5,
		"postgres.conn_max_lifetime": 30 * time.Minute,
		"auth.private_key_hex":       "", "auth.public_key_hex": "",
		"auth.token_duration": 24 * time.Hour,
		"auth.request_limit":  20, "auth.rate_window": time.Minute,
		"postgres.auto_migrate": false,
		"uploads.directory":     "./storage/uploads", "uploads.max_file_size_mb": 20, "uploads.max_files_per_request": 10, "uploads.max_pdf_pages": 100,
		"processing.enabled": false, "processing.provider": "openai-compatible", "processing.base_url": "https://api.openai.com", "processing.api_key": "", "processing.model": "",
		"processing.poll_interval": 2 * time.Second, "processing.max_concurrent_files": 3, "processing.timeout": 90 * time.Second, "processing.lease_duration": 5 * time.Minute, "processing.max_page_dimension": 1600,
		"assistant.enabled": false, "assistant.base_url": "https://api.openai.com", "assistant.api_key": "", "assistant.model": "",
		"assistant.timeout": 30 * time.Second, "assistant.max_output_tokens": 1500, "assistant.request_limit": 10, "assistant.rate_window": time.Minute, "assistant.max_concurrent_requests": 4,
	}
	for key, value := range defaults {
		v.SetDefault(key, value)
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind configuration environment: %w", err)
		}
	}
	if err := v.ReadInConfig(); err != nil {
		// Parser errors can contain YAML snippets, including secrets.
		return nil, fmt.Errorf("%w: cannot read configuration file or invalid YAML", ErrInvalidConfiguration)
	}
	var cfg Config
	if err := v.UnmarshalExact(&cfg); err != nil {
		return nil, fmt.Errorf("%w: unknown key or invalid field type", ErrInvalidConfiguration)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate configuration: %w", err)
	}
	return &cfg, nil
}
