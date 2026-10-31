package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"mega-trainer-go/internal/domain"
)

// stateTTL is how long an abandoned dialog keeps its state.
const stateTTL = 24 * time.Hour

// StateRepo stores the FSM state of every user in Redis.
type StateRepo struct {
	client *redis.Client
	prefix string
}

var _ domain.StateRepository = (*StateRepo)(nil)

// NewStateRepo creates the dialog state repository.
func NewStateRepo(client *redis.Client) *StateRepo {
	return &StateRepo{
		client: client,
		prefix: "mega-trainer:user_state",
	}
}

// GetState returns the stored state or (nil, nil) when there is none.
func (r *StateRepo) GetState(ctx context.Context, userID int64) (*domain.UserState, error) {
	raw, err := r.client.Get(ctx, r.key(userID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get state for user %d: %w", userID, err)
	}

	var state domain.UserState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("decode state for user %d: %w", userID, err)
	}

	return &state, nil
}

// SetState writes the state with a TTL so abandoned dialogs do not leak.
func (r *StateRepo) SetState(ctx context.Context, state *domain.UserState) error {
	if state == nil {
		return errors.New("set state: nil state")
	}

	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode state for user %d: %w", state.UserID, err)
	}

	if err := r.client.Set(ctx, r.key(state.UserID), raw, stateTTL).Err(); err != nil {
		return fmt.Errorf("set state for user %d: %w", state.UserID, err)
	}

	return nil
}

// ClearState removes the state of the given user.
func (r *StateRepo) ClearState(ctx context.Context, userID int64) error {
	if err := r.client.Del(ctx, r.key(userID)).Err(); err != nil {
		return fmt.Errorf("clear state for user %d: %w", userID, err)
	}
	return nil
}

func (r *StateRepo) key(userID int64) string {
	return fmt.Sprintf("%s:%d", r.prefix, userID)
}
