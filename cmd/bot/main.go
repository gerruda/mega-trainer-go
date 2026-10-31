// Command bot runs the Telegram bot process: it loads the configuration,
// handles OS signals and delegates the wiring to internal/app.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"mega-trainer-go/internal/app"
	"mega-trainer-go/internal/config"
	"mega-trainer-go/internal/logger"
)

// logComponent identifies this package in log messages.
const logComponent = "cmd/bot"

// version is injected at build time:
//
//	go build -ldflags "-X main.version=v1.2.3" ./cmd/bot
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.Logging)
	if err != nil {
		return err
	}

	if version == "dev" {
		version = cfg.App.Version
	}

	// SIGTERM is what Docker and systemd send on stop, so both signals must
	// trigger the same graceful shutdown path.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info().
		Str("version", version).
		Str("environment", cfg.App.Environment).
		Msg(logger.Source(logComponent, "main") + "запуск бота")

	return app.Run(ctx, cfg, log)
}

func configPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return "configs/config.yaml"
}
