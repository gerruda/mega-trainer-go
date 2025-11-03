package service

import (
	"context"
	"fmt"

	"mega-trainer-go/internal/storage/postgres"
	"mega-trainer-go/pkg/logger"
)

// UserService интерфейс для работы с пользователями
type UserService interface {
	GetOrCreate(ctx context.Context, telegramID int64, username, firstName, lastName, languageCode string) (*postgres.User, error)
	GetUser(ctx context.Context, telegramID int64) (*postgres.User, error)
	UpdateUser(ctx context.Context, user *postgres.User) error
}

type userService struct {
	userRepo postgres.UserRepository
	log      *logger.Logger
}

func NewUserService(userRepo postgres.UserRepository, log *logger.Logger) UserService {
	return &userService{
		userRepo: userRepo,
		log:      log,
	}
}

func (s *userService) GetOrCreate(ctx context.Context, telegramID int64, username, firstName, lastName, languageCode string) (*postgres.User, error) {
	// Пытаемся найти существующего пользователя
	user, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err == nil {
		return user, nil
	}

	// Если пользователь не найден, создаем нового
	if err == postgres.ErrNotFound {
		newUser := &postgres.User{
			TelegramID:   telegramID,
			Username:     username,
			FirstName:    firstName,
			LastName:     lastName,
			LanguageCode: languageCode,
		}

		if err := s.userRepo.Create(ctx, newUser); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		s.log.Info().Msgf("new user created",
			"telegram_id", telegramID,
			"username", username,
		)
		return newUser, nil
	}

	return nil, fmt.Errorf("failed to get user: %w", err)
}

func (s *userService) GetUser(ctx context.Context, telegramID int64) (*postgres.User, error) {
	user, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

func (s *userService) UpdateUser(ctx context.Context, user *postgres.User) error {
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}
