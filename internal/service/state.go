package service

import (
	"context"
	"fmt"

	"mega-trainer-go/internal/domain"
)

// StateService provides higher-level operations over the dialog state
// repository: starting a dialog, advancing a step and collecting answers.
type StateService struct {
	repo domain.StateRepository
}

// NewStateService creates a StateService.
func NewStateService(repo domain.StateRepository) *StateService {
	return &StateService{repo: repo}
}

// Get returns the current dialog state or (nil, nil) when the user has none.
func (s *StateService) Get(ctx context.Context, userID int64) (*domain.UserState, error) {
	state, err := s.repo.GetState(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user state: %w", err)
	}
	return state, nil
}

// Start begins a new dialog on step 1 and drops any previous answers.
func (s *StateService) Start(ctx context.Context, userID int64, command string) error {
	return s.save(ctx, &domain.UserState{
		UserID:  userID,
		Command: command,
		Step:    1,
		Data:    make(map[string]string),
	})
}

// SetStep moves the dialog to the next step and keeps the collected answers.
func (s *StateService) SetStep(ctx context.Context, userID int64, step int) error {
	state, err := s.Get(ctx, userID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("advance state: %w", domain.ErrNotFound)
	}

	state.Step = step
	return s.save(ctx, state)
}

// SetData stores a single answer and preserves the current step.
func (s *StateService) SetData(ctx context.Context, userID int64, key, value string) error {
	state, err := s.Get(ctx, userID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("set state data: %w", domain.ErrNotFound)
	}

	if state.Data == nil {
		state.Data = make(map[string]string)
	}
	state.Data[key] = value

	return s.save(ctx, state)
}

// Finish ends the dialog and removes the state.
func (s *StateService) Finish(ctx context.Context, userID int64) error {
	if err := s.repo.ClearState(ctx, userID); err != nil {
		return fmt.Errorf("clear user state: %w", err)
	}
	return nil
}

func (s *StateService) save(ctx context.Context, state *domain.UserState) error {
	if err := s.repo.SetState(ctx, state); err != nil {
		return fmt.Errorf("save user state: %w", err)
	}
	return nil
}
