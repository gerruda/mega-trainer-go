// Command bot runs the Telegram bot process: it wires the configuration,
// infrastructure clients, services and the Telegram delivery layer, then polls
// for updates until it receives SIGINT/SIGTERM.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mega-trainer-go/internal/bot"
	"mega-trainer-go/internal/config"
	"mega-trainer-go/internal/delivery/telegram"
	"mega-trainer-go/internal/logger"
	"mega-trainer-go/internal/monitoring"
	"mega-trainer-go/internal/service"
	"mega-trainer-go/internal/storage/postgres"
	redisstore "mega-trainer-go/internal/storage/redis"
	"mega-trainer-go/internal/telegramapi"
)

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

	db, err := postgres.Open(ctx, cfg.Database.Postgres)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close database")
		}
	}()

	redisClient := redisstore.NewClient(cfg.Redis)
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Error().Err(err).Msg("failed to close redis client")
		}
	}()

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	err = redisstore.Ping(pingCtx, redisClient)
	cancelPing()
	if err != nil {
		return err
	}

	metrics := monitoring.NewMetrics(cfg.App.Name)

	var monitor *monitoring.Server
	if cfg.Monitoring.Enabled {
		monitor = monitoring.NewServer(cfg.Monitoring, metrics, log,
			func(ctx context.Context) error { return db.PingContext(ctx) },
			func(ctx context.Context) error { return redisstore.Ping(ctx, redisClient) },
		)
		monitor.Start()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := monitor.Shutdown(shutdownCtx); err != nil {
				log.Error().Err(err).Msg("failed to shut down monitoring server")
			}
		}()
	}

	botAPI, err := telegramapi.New(cfg.Telegram)
	if err != nil {
		return err
	}

	if proxyHost := telegramapi.ProxyHost(cfg.Telegram.ProxyURL); proxyHost != "" {
		log.Info().Str("proxy", proxyHost).Msg("telegram requests go through a proxy")
	}

	userRepo := postgres.NewUserRepo(db)
	stateRepo := redisstore.NewStateRepo(redisClient)

	handler := telegram.NewHandler(
		botAPI,
		service.NewStateService(stateRepo),
		service.NewUserService(userRepo),
		service.NewMeasurementService(postgres.NewMeasurementRepo(db), userRepo),
		log,
	)

	log.Info().
		Str("username", botAPI.Self.UserName).
		Str("version", version).
		Str("environment", cfg.App.Environment).
		Int("workers", cfg.App.Workers).
		Msg("bot started")

	runner := bot.New(botAPI, handler, log, metrics, cfg.App.Workers)
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("run bot: %w", err)
	}

	log.Info().Msg("bot stopped")
	return nil
}

func configPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return "configs/config.yaml"
}
