// Package config loads and validates application configuration.
package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidConfiguration = errors.New("invalid configuration")

type Config struct {
	HTTPServer HTTPServerConfig `mapstructure:"http_server"`
	Postgres   PostgresConfig   `mapstructure:"postgres"`
	Auth       AuthConfig       `mapstructure:"auth"`
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
}

type AuthConfig struct {
	PrivateKeyHex string        `mapstructure:"private_key_hex" json:"-"`
	PublicKeyHex  string        `mapstructure:"public_key_hex"`
	TokenDuration time.Duration `mapstructure:"token_duration"`
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
	return nil
}
