// Package redis implements the dialog state repository on top of Redis.
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"mega-trainer-go/internal/config"
)

// NewClient creates a Redis client from the application configuration.
func NewClient(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
}

// Ping verifies that Redis is reachable.
func Ping(ctx context.Context, client *redis.Client) error {
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}
