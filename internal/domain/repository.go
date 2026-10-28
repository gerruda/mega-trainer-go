package domain

import "context"

// UserRepository persists bot users. Implementations live in the storage layer
// and must return domain sentinel errors (wrapped with %w) where applicable.
type UserRepository interface {
	// Upsert creates the user or updates their profile if the telegram_id is
	// already known, returning the stored entity.
	Upsert(ctx context.Context, user *User) (*User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	// ListActive returns active (not deleted) users, newest first. It is used by
	// the reminder scheduler and therefore bounded by limit.
	ListActive(ctx context.Context, limit int) ([]*User, error)
}

// StateRepository keeps the dialog state of a single user between updates.
type StateRepository interface {
	GetState(ctx context.Context, userID int64) (*UserState, error)
	SetState(ctx context.Context, state *UserState) error
	ClearState(ctx context.Context, userID int64) error
}
