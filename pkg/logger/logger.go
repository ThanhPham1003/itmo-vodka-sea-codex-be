// Package logger creates structured application loggers without global state.
package logger

import (
	"fmt"
	"io"

	"github.com/rs/zerolog"
)

func NewLogger(output io.Writer, level string) (zerolog.Logger, error) {
	parsed, err := zerolog.ParseLevel(level)
	if err != nil {
		return zerolog.Logger{}, fmt.Errorf("invalid log level")
	}
	return zerolog.New(output).Level(parsed).With().Timestamp().Str("service", "sea-codex-server").Logger(), nil
}
