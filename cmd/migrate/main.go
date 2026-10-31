// Command migrate applies, rolls back or reports the SQL schema migrations
// embedded in the binary. It needs database settings only, so it can run before
// the bot is configured with a Telegram token.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"mega-trainer-go/internal/config"
	"mega-trainer-go/internal/logger"
	"mega-trainer-go/internal/storage/postgres"
	"mega-trainer-go/migrations"
)

// logComponent identifies this package in log messages.
const logComponent = "cmd/migrate"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	cfg, err := config.LoadForMigration(configPath())
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.Logging)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := postgres.Open(ctx, cfg.Database.Postgres)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	migrator := postgres.NewMigrator(db, migrations.FS)

	switch command {
	case "up":
		applied, err := migrator.Up(ctx)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			log.Info().Msg(logger.Source(logComponent, "run") + "неприменённых миграций нет")
			return nil
		}
		for _, migration := range applied {
			log.Info().Str("migration", migration).Msg(logger.Source(logComponent, "run") + "миграция применена")
		}
		return nil

	case "down":
		version, err := migrator.Down(ctx)
		if err != nil {
			return err
		}
		log.Info().Str("migration", version).Msg(logger.Source(logComponent, "run") + "миграция откачена")
		return nil

	case "status":
		statuses, err := migrator.Status(ctx)
		if err != nil {
			return err
		}
		for _, status := range statuses {
			if status.Applied {
				log.Info().Str("migration", status.Version).Msg(logger.Source(logComponent, "run") + "миграция применена (статус)")
				continue
			}
			log.Warn().Str("migration", status.Version).Msg(logger.Source(logComponent, "run") + "миграция не применена (статус)")
		}
		return nil

	default:
		return fmt.Errorf("unknown command %q: expected up, down or status", command)
	}
}

func configPath() string {
	if path := os.Getenv("CONFIG_PATH"); path != "" {
		return path
	}
	return "configs/config.yaml"
}
