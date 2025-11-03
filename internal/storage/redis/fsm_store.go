package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"mega-trainer-go/internal/domain"
)

type redisStateRepository struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewRedisStateRepository(client *redis.Client) domain.StateRepository {
	return &redisStateRepository{
		client: client,
		prefix: "user_state",
		ttl:    24 * time.Hour,
	}
}

func (r *redisStateRepository) GetState(ctx context.Context, userID int64) (*domain.UserState, error) {
	key := fmt.Sprintf("%s:%d", r.prefix, userID)

	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get state: %w", err)
	}

	var state domain.UserState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &state, nil
}

func (r *redisStateRepository) SetState(ctx context.Context, state *domain.UserState) error {
	key := fmt.Sprintf("%s:%d", r.prefix, state.UserID)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := r.client.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set state: %w", err)
	}

	return nil
}

func (r *redisStateRepository) ClearState(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("%s:%d", r.prefix, userID)
	return r.client.Del(ctx, key).Err()
}
