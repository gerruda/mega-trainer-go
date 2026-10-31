package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"mega-trainer-go/internal/domain"
)

// memStateRepo is an in-memory domain.StateRepository for tests.
type memStateRepo struct {
	mu     sync.Mutex
	states map[int64]domain.UserState
}

func newMemStateRepo() *memStateRepo {
	return &memStateRepo{states: make(map[int64]domain.UserState)}
}

func (r *memStateRepo) GetState(_ context.Context, userID int64) (*domain.UserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[userID]
	if !ok {
		return nil, nil
	}
	return cloneState(&state), nil
}

func (r *memStateRepo) SetState(_ context.Context, state *domain.UserState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.states[state.UserID] = *cloneState(state)
	return nil
}

func (r *memStateRepo) ClearState(_ context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.states, userID)
	return nil
}

func cloneState(state *domain.UserState) *domain.UserState {
	clone := *state
	clone.Data = make(map[string]string, len(state.Data))
	for key, value := range state.Data {
		clone.Data[key] = value
	}
	return &clone
}

func TestStateServiceLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := NewStateService(newMemStateRepo())
	const userID int64 = 42

	state, err := svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get empty state: %v", err)
	}
	if state != nil {
		t.Fatalf("state = %+v, want nil", state)
	}

	if err := svc.Start(ctx, userID, "water"); err != nil {
		t.Fatalf("start: %v", err)
	}

	state, err = svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if state.Command != "water" || state.Step != 1 {
		t.Fatalf("state = %+v, want command=water step=1", state)
	}

	if err := svc.SetData(ctx, userID, "weight", "70.5"); err != nil {
		t.Fatalf("set data: %v", err)
	}
	if err := svc.SetStep(ctx, userID, 2); err != nil {
		t.Fatalf("set step: %v", err)
	}

	state, err = svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	// Advancing a step must never drop the collected answers.
	if state.Step != 2 || state.Data["weight"] != "70.5" {
		t.Fatalf("state = %+v, want step=2 with weight=70.5", state)
	}

	if err := svc.Finish(ctx, userID); err != nil {
		t.Fatalf("finish: %v", err)
	}

	state, err = svc.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get state after finish: %v", err)
	}
	if state != nil {
		t.Fatalf("state = %+v, want nil after finish", state)
	}
}

func TestStateServiceRequiresActiveDialog(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc := NewStateService(newMemStateRepo())

	if err := svc.SetStep(ctx, 7, 2); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("SetStep error = %v, want ErrNotFound", err)
	}
	if err := svc.SetData(ctx, 7, "weight", "70"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("SetData error = %v, want ErrNotFound", err)
	}
}
