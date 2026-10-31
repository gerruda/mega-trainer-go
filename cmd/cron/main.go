// Command cron runs the background jobs of the bot: currently the scheduled
// "log your weight" reminders, which are sent to every active user.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mega-trainer-go/internal/config"
	"mega-trainer-go/internal/cron"
	"mega-trainer-go/internal/logger"
	"mega-trainer-go/internal/storage/postgres"
	"mega-trainer-go/internal/telegramapi"
)

// logComponent identifies this package in log messages.
const logComponent = "cmd/cron"

// version is injected at build time.
var version = "dev"

func main() {
	once := flag.Bool("once", false, "отправить напоминания один раз и выйти")
	flag.Parse()

	if err := run(*once); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run(once bool) error {
	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.Logging)
	if err != nil {
		return err
	}

	if !cfg.Reminders.Enabled {
		log.Info().Msg(logger.Source(logComponent, "run") + "напоминания отключены, работа завершена")
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.Database.Postgres)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg(logger.Source(logComponent, "run") + "не удалось закрыть соединение с базой данных")
		}
	}()

	botAPI, err := telegramapi.New(cfg.Telegram)
	if err != nil {
		return err
	}

	runner := cron.New(
		postgres.NewUserRepo(db),
		&telegramSender{api: botAPI},
		log,
		time.Duration(cfg.Reminders.IntervalHours)*time.Hour,
	)

	if once {
		log.Info().Msg(logger.Source(logComponent, "run") + "разовая рассылка напоминаний")
		return runner.Once(ctx)
	}

	log.Info().
		Str("version", version).
		Int("interval_hours", cfg.Reminders.IntervalHours).
		Msg(logger.Source(logComponent, "run") + "планировщик запущен")

	return runner.Run(ctx)
}

// telegramSender adapts the Bot API client to the cron.MessageSender contract.
type telegramSender struct {
	api *tgbotapi.BotAPI
}

func (s *telegramSender) SendMessage(chatID int64, text string) error {
	if _, err := s.api.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		return fmt.Errorf("send message to %d: %w", chatID, err)
	}
	return nil
}

func configPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return "configs/config.yaml"
}
