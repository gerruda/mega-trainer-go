package service

import (
	"context"
	"fmt"

	"mega-trainer-go/internal/domain"
)

// UserService keeps the local user record in sync with Telegram profiles.
type UserService struct {
	repo domain.UserRepository
}

// NewUserService creates a UserService.
func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// Ensure creates or updates the user identified by the Telegram profile and
// returns the stored entity.
func (s *UserService) Ensure(ctx context.Context, telegramID int64, username, firstName, lastName, languageCode string) (*domain.User, error) {
	if telegramID == 0 {
		return nil, fmt.Errorf("%w: telegram id is required", domain.ErrValidation)
	}

	user, err := s.repo.Upsert(ctx, &domain.User{
		TelegramID:   telegramID,
		Username:     username,
		FirstName:    firstName,
		LastName:     lastName,
		LanguageCode: languageCode,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	return user, nil
}
