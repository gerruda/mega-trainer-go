// Package logger builds the application's structured logger (zerolog).
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"

	"mega-trainer-go/internal/config"
)

// New returns a configured logger. It never panics: a file that cannot be
// opened is reported as an error so the caller (main) can decide what to do.
func New(cfg config.LoggingConfig) (zerolog.Logger, error) {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	output, err := newWriter(cfg)
	if err != nil {
		return zerolog.Nop(), err
	}

	logger := zerolog.New(output).
		Level(parseLevel(cfg.Level)).
		With().
		Timestamp().
		Str("service", "mega-trainer").
		Logger()

	return logger, nil
}

func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func newWriter(cfg config.LoggingConfig) (io.Writer, error) {
	var base io.Writer
	if cfg.Format == "json" {
		base = os.Stdout
	} else {
		base = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		}
	}

	if cfg.Output == "stdout" || cfg.FilePath == "" {
		return base, nil
	}

	if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	if cfg.Format == "json" {
		return file, nil
	}
	return io.MultiWriter(base, file), nil
}
