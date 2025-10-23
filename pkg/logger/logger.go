package logger

import (
	"io"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"mega-trainer-go/internal/config"
)

type Logger struct {
	zerolog.Logger
}

func New(cfg *config.LoggingConfig) *Logger {
	// Настройка ошибок
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	var output io.Writer
	var level zerolog.Level

	// Уровень логирования
	switch cfg.Level {
	case "debug":
		level = zerolog.DebugLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	default:
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Формат вывода
	if cfg.Format == "json" {
		output = os.Stdout
	} else {
		output = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
		}
	}

	// Файловый вывод если нужно
	if cfg.Output == "file" && cfg.FilePath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0755); err != nil {
			panic(err)
		}
		file, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}

		if cfg.Format == "json" {
			output = file
		} else {
			output = io.MultiWriter(output, file)
		}
	}

	logger := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Str("service", "telegram-bot").
		Logger()

	return &Logger{logger}
}

// Методы для удобства
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{l.With().Interface(key, value).Logger()}
}

func (l *Logger) WithError(err error) *Logger {
	return &Logger{l.With().Err(err).Logger()}
}
