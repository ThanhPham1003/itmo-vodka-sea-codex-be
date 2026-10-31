// Package config loads and validates application configuration.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidConfiguration = errors.New("invalid configuration")

type Config struct {
	HTTPServer HTTPServerConfig `mapstructure:"http_server"`
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Auth       AuthConfig       `mapstructure:"auth"`
	Uploads    UploadsConfig    `mapstructure:"uploads"`
	Processing ProcessingConfig `mapstructure:"processing"`
	Assistant  AssistantConfig  `mapstructure:"assistant"`
}

type AssistantConfig struct {
	Enabled               bool          `mapstructure:"enabled"`
	BaseURL               string        `mapstructure:"base_url"`
	APIKey                string        `mapstructure:"api_key" json:"-"`
	Model                 string        `mapstructure:"model"`
	Timeout               time.Duration `mapstructure:"timeout"`
	MaxOutputTokens       int           `mapstructure:"max_output_tokens"`
	RequestLimit          int           `mapstructure:"request_limit"`
	RateWindow            time.Duration `mapstructure:"rate_window"`
	MaxConcurrentRequests int           `mapstructure:"max_concurrent_requests"`
}

func (c AssistantConfig) Validate() error {
	// Zero values are supported for programmatically constructed apps with AI off.
	if !c.Enabled {
		return nil
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme != "https" || u.Host != "api.openai.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/" && u.Path != "/v1" && u.Path != "/v1/") {
		return fmt.Errorf("%w: assistant.base_url must use the OpenAI HTTPS endpoint", ErrInvalidConfiguration)
	}
	if strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(c.Model) == "" || c.Timeout < time.Second || c.Timeout > 60*time.Second || c.MaxOutputTokens < 256 || c.MaxOutputTokens > 4096 || c.RequestLimit < 1 || c.RequestLimit > 100 || c.RateWindow < time.Second || c.RateWindow > time.Hour || c.MaxConcurrentRequests < 1 || c.MaxConcurrentRequests > 20 {
		return fmt.Errorf("%w: assistant credentials or limits are invalid", ErrInvalidConfiguration)
	}
	return nil
}

type HTTPServerConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	Mode              string        `mapstructure:"mode"`
	LogLevel          string        `mapstructure:"log_level"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
}

func (c HTTPServerConfig) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

type PostgresConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password" json:"-"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	AutoMigrate     bool          `mapstructure:"auto_migrate"`
}

type AuthConfig struct {
	PrivateKeyHex string        `mapstructure:"private_key_hex" json:"-"`
	PublicKeyHex  string        `mapstructure:"public_key_hex"`
	TokenDuration time.Duration `mapstructure:"token_duration"`
	RequestLimit  int           `mapstructure:"request_limit"`
	RateWindow    time.Duration `mapstructure:"rate_window"`
}

type UploadsConfig struct {
	Directory          string `mapstructure:"directory"`
	MaxFileSizeMB      int64  `mapstructure:"max_file_size_mb"`
	MaxFilesPerRequest int    `mapstructure:"max_files_per_request"`
	MaxPDFPages        int    `mapstructure:"max_pdf_pages"`
}
type ProcessingConfig struct {
	Enabled            bool          `mapstructure:"enabled"`
	Provider           string        `mapstructure:"provider"`
	BaseURL            string        `mapstructure:"base_url"`
	APIKey             string        `mapstructure:"api_key" json:"-"`
	Model              string        `mapstructure:"model"`
	PollInterval       time.Duration `mapstructure:"poll_interval"`
	MaxConcurrentFiles int           `mapstructure:"max_concurrent_files"`
	Timeout            time.Duration `mapstructure:"timeout"`
	LeaseDuration      time.Duration `mapstructure:"lease_duration"`
	MaxPageDimension   int           `mapstructure:"max_page_dimension"`
}

// Validate reports field names, never configuration values or secrets.
func (c Config) Validate() error {
	invalid := func(field string) error {
		return fmt.Errorf("%w: %s", ErrInvalidConfiguration, field)
	}
	if strings.TrimSpace(c.HTTPServer.Host) == "" {
		return invalid("http_server.host is required")
	}
	if c.HTTPServer.Port < 1 || c.HTTPServer.Port > 65535 {
		return invalid("http_server.port must be between 1 and 65535")
	}
	switch c.HTTPServer.Mode {
	case "debug", "release", "test":
	default:
		return invalid("http_server.mode is unsupported")
	}
	switch c.HTTPServer.LogLevel {
	case "trace", "debug", "info", "warn", "error", "fatal", "panic", "disabled":
	default:
		return invalid("http_server.log_level is unsupported")
	}
	if c.HTTPServer.ReadTimeout <= 0 || c.HTTPServer.ReadHeaderTimeout <= 0 ||
		c.HTTPServer.WriteTimeout <= 0 || c.HTTPServer.IdleTimeout <= 0 ||
		c.HTTPServer.ShutdownTimeout <= 0 {
		return invalid("http_server timeouts must be positive")
	}
	if strings.TrimSpace(c.Postgres.Host) == "" || strings.TrimSpace(c.Postgres.User) == "" ||
		strings.TrimSpace(c.Postgres.DBName) == "" || c.Postgres.Password == "" {
		return invalid("postgres host, user, dbname and password are required")
	}
	if c.Postgres.Port < 1 || c.Postgres.Port > 65535 {
		return invalid("postgres.port must be between 1 and 65535")
	}
	switch c.Postgres.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return invalid("postgres.sslmode is unsupported")
	}
	if c.Postgres.ConnectTimeout <= 0 || c.Postgres.ConnMaxLifetime <= 0 ||
		c.Postgres.MaxOpenConns < 1 || c.Postgres.MaxIdleConns < 0 ||
		c.Postgres.MaxIdleConns > c.Postgres.MaxOpenConns {
		return invalid("postgres connection timeout or pool settings are invalid")
	}
	if strings.TrimSpace(c.Auth.PrivateKeyHex) == "" || strings.TrimSpace(c.Auth.PublicKeyHex) == "" {
		return invalid("auth private/public key pair is required")
	}
	if c.Auth.TokenDuration < time.Second {
		return invalid("auth.token_duration must be at least one second")
	}
	if c.Auth.RequestLimit < 1 || c.Auth.RateWindow <= 0 {
		return invalid("auth rate limit must be positive")
	}
	if c.Uploads.Directory == "" || c.Uploads.MaxFileSizeMB < 1 || c.Uploads.MaxFileSizeMB > 100 || c.Uploads.MaxFilesPerRequest < 1 || c.Uploads.MaxFilesPerRequest > 50 || c.Uploads.MaxPDFPages < 1 || c.Uploads.MaxPDFPages > 500 {
		return invalid("uploads limits are invalid")
	}
	if c.Processing.PollInterval <= 0 || c.Processing.MaxConcurrentFiles < 1 || c.Processing.MaxConcurrentFiles > 10 || c.Processing.Timeout <= 0 || c.Processing.LeaseDuration <= c.Processing.Timeout || c.Processing.MaxPageDimension < 256 || c.Processing.MaxPageDimension > 4096 {
		return invalid("processing limits are invalid")
	}
	if c.Processing.Enabled && (c.Processing.APIKey == "" || c.Processing.Model == "" || c.Processing.BaseURL == "") {
		return invalid("processing provider/model/key are required when enabled")
	}
	if c.Processing.Provider != "anthropic" && c.Processing.Provider != "openai-compatible" {
		return invalid("processing.provider is unsupported")
	}
	if err := c.Assistant.Validate(); err != nil {
		return err
	}
	if c.Assistant.Enabled && c.HTTPServer.WriteTimeout <= c.Assistant.Timeout {
		return invalid("http_server.write_timeout must exceed assistant.timeout")
	}
	return nil
}
