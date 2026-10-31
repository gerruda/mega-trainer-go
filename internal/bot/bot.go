// Package bot runs the Telegram long-polling loop with a bounded worker pool.
package bot

import (
	"context"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"

	"mega-trainer-go/internal/delivery/telegram"
	"mega-trainer-go/internal/logger"
	"mega-trainer-go/internal/monitoring"
)

// logComponent identifies this package in log messages.
const logComponent = "bot"

// Bot wires the Telegram API client with the update handler.
type Bot struct {
	api     *tgbotapi.BotAPI
	handler *telegram.Handler
	log     zerolog.Logger
	metrics *monitoring.Metrics
	workers int
}

// New creates a Bot. workers bounds how many updates are processed at once.
func New(api *tgbotapi.BotAPI, handler *telegram.Handler, log zerolog.Logger, metrics *monitoring.Metrics, workers int) *Bot {
	if workers < 1 {
		workers = 1
	}
	return &Bot{
		api:     api,
		handler: handler,
		log:     log,
		metrics: metrics,
		workers: workers,
	}
}

// Run polls Telegram until the context is cancelled, then stops polling and
// waits for the in-flight updates to finish.
func (b *Bot) Run(ctx context.Context) error {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := b.api.GetUpdatesChan(updateConfig)

	sem := make(chan struct{}, b.workers)
	var wg sync.WaitGroup

	defer func() {
		// Closing the updates channel is the API's way of stopping the poller.
		b.api.StopReceivingUpdates()
		wg.Wait()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}

			// Acquire a worker slot or stop waiting when shutting down.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return nil
			}

			wg.Add(1)
			go func(update tgbotapi.Update) {
				defer wg.Done()
				defer func() { <-sem }()
				b.process(ctx, update)
			}(update)
		}
	}
}

func (b *Bot) process(ctx context.Context, update tgbotapi.Update) {
	start := time.Now()

	if err := b.handler.HandleUpdate(ctx, update); err != nil {
		b.metrics.UpdatesFailed.Inc()
		b.log.Error().
			Err(err).
			Int("update_id", update.UpdateID).
			Msg(logger.Source(logComponent, "process") + "не удалось обработать обновление Telegram")
		return
	}

	b.metrics.UpdatesProcessed.Inc()
	b.metrics.UpdateDuration.Observe(time.Since(start).Seconds())
}
