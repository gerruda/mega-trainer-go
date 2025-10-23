package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	// tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	// "github.com/golang-migrate/migrate/v4/database/postgres"
	// "mega-trainer-go/internal/bot"
	"mega-trainer-go/internal/config"
	"mega-trainer-go/internal/storage/redis"

	// "mega-trainer-go/internal/storage/postgres"
	// "mega-trainer-go/internal/storage/redis"
	"mega-trainer-go/pkg/database"
	"mega-trainer-go/pkg/logger"
	"mega-trainer-go/pkg/monitoring"
)

func main() {
	// Загрузка конфигурации
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}

	// Инициализация логгера
	log := logger.New(&cfg.Logging)

	// Инициализация метрик
	metrics := monitoring.NewMetrics(cfg.App.Name)
	if cfg.Monitoring.PrometheusEnabled {
		metrics.StartServer(cfg.Monitoring.PrometheusPort)
	}

	// Подключение к БД
	pg, err := database.NewPostgres(&cfg.Database.Postgres)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer pg.Close()

	// Инициализация Redis
	redisStore := redis.NewFSMStore(&cfg.Redis)

	// // Инициализация репозиториев
	// userRepo := postgres.NewUserRepository(pg.DB)
	//
	// // Создание бота
	// botAPI, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	// if err != nil {
	// 	log.Fatal().Err(err).Msg("Failed to create bot")
	// }
	//
	// botAPI.Debug = cfg.Telegram.Debug
	//
	// log.Info().
	// 	Str("username", botAPI.Self.UserName).
	// 	Msg("Bot authorized successfully")
	//
	// // Создание обработчика
	// handler := bot.NewHandler(botAPI, redisStore, userRepo, metrics, log, cfg)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Запуск бота
	// if cfg.Telegram.WebhookURL != "" {
	// 	handler.StartWebhook(ctx, cfg.Telegram.WebhookURL)
	// } else {
	// 	handler.StartPolling(ctx)
	// }

	<-ctx.Done()
	log.Info().Msg("Shutting down bot...")
}
