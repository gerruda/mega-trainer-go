// Package app wires the bot's dependencies and runs the Telegram polling loop.
//
// It is the composition root shared by cmd/bot and the end-to-end tests: main
// only loads the configuration and handles signals, while all infrastructure is
// assembled here.
package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

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

// logComponent identifies this package in log messages.
const logComponent = "app"

// Run assembles the application and runs the bot until the context is cancelled.
func Run(ctx context.Context, cfg *config.Config, log zerolog.Logger) error {
	db, err := postgres.Open(ctx, cfg.Database.Postgres)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg(logger.Source(logComponent, "Run") + "не удалось закрыть соединение с базой данных (завершение работы)")
		}
	}()

	redisClient := redisstore.NewClient(cfg.Redis)
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Error().Err(err).Msg(logger.Source(logComponent, "Run") + "не удалось закрыть соединение с Redis (завершение работы)")
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
		monitor = monitoring.NewServer(cfg.Monitoring, log,
			func(ctx context.Context) error { return db.PingContext(ctx) },
			func(ctx context.Context) error { return redisstore.Ping(ctx, redisClient) },
		)
		monitor.Start()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := monitor.Shutdown(shutdownCtx); err != nil {
				log.Error().Err(err).Msg(logger.Source(logComponent, "Run") + "не удалось остановить сервер метрик (завершение работы)")
			}
		}()
	}

	botAPI, err := telegramapi.New(cfg.Telegram)
	if err != nil {
		return err
	}

	if proxyHost := telegramapi.ProxyHost(cfg.Telegram.ProxyURL); proxyHost != "" {
		log.Info().Str("proxy", proxyHost).Msg(logger.Source(logComponent, "Run") + "запросы к Telegram идут через прокси")
	}

	userRepo := postgres.NewUserRepo(db)
	handler := telegram.NewHandler(
		botAPI,
		service.NewStateService(redisstore.NewStateRepo(redisClient)),
		service.NewUserService(userRepo),
		service.NewMeasurementService(postgres.NewMeasurementRepo(db), userRepo),
		log,
	)

	log.Info().
		Str("username", botAPI.Self.UserName).
		Str("environment", cfg.App.Environment).
		Int("workers", cfg.App.Workers).
		Msg(logger.Source(logComponent, "Run") + "бот запущен")

	runner := bot.New(botAPI, handler, log, metrics, cfg.App.Workers)
	if err := runner.Run(ctx); err != nil {
		return fmt.Errorf("run bot: %w", err)
	}

	log.Info().Msg(logger.Source(logComponent, "Run") + "бот остановлен (контекст завершён)")
	return nil
}
