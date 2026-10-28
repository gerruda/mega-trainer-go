// Package cron contains the background jobs of the application. Currently it
// implements the scheduled "log your weight" reminder.
package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"mega-trainer-go/internal/domain"
)

// reminderText is sent to every active user by the reminder job.
const reminderText = "⚖️ Напоминание: запишите текущий вес — /measure\n📈 Динамика: /chart"

// usersBatch is the maximum number of users reminded in one run.
const usersBatch = 1000

// MessageSender delivers a plain text message to a Telegram chat.
type MessageSender interface {
	SendMessage(chatID int64, text string) error
}

// Runner periodically reminds users to record their weight.
type Runner struct {
	users    domain.UserRepository
	sender   MessageSender
	log      zerolog.Logger
	interval time.Duration
}

// New creates a reminder runner. A non-positive interval falls back to 24h.
func New(users domain.UserRepository, sender MessageSender, log zerolog.Logger, interval time.Duration) *Runner {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Runner{
		users:    users,
		sender:   sender,
		log:      log,
		interval: interval,
	}
}

// Run sends reminders every interval until the context is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	r.log.Info().Dur("interval", r.interval).Msg("reminder scheduler started")

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info().Msg("reminder scheduler stopped")
			return nil
		case <-ticker.C:
			if err := r.Once(ctx); err != nil {
				// A failed run must not kill the scheduler: the next tick retries.
				r.log.Error().Err(err).Msg("reminder run failed")
			}
		}
	}
}

// Once sends one round of reminders and returns only infrastructure errors.
// Delivery errors for individual users are counted and logged, not returned.
func (r *Runner) Once(ctx context.Context) error {
	users, err := r.users.ListActive(ctx, usersBatch)
	if err != nil {
		return fmt.Errorf("list active users: %w", err)
	}

	var sent, failed int
	for _, user := range users {
		if err := r.sender.SendMessage(user.TelegramID, reminderText); err != nil {
			failed++
			r.log.Warn().Err(err).Int64("telegram_id", user.TelegramID).Msg("failed to send reminder")
			continue
		}
		sent++
	}

	r.log.Info().Int("sent", sent).Int("failed", failed).Int("users", len(users)).Msg("reminders sent")
	return nil
}
