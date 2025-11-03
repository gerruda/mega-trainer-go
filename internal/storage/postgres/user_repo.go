package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// UserRepo реализация UserRepository
type UserRepo struct {
	*BaseRepository
}

// NewUserRepository создает новый репозиторий пользователей
func NewUserRepository(db *sqlx.DB) UserRepository {
	return &UserRepo{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create создает нового пользователя
func (r *UserRepo) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (
			telegram_id, username, first_name, last_name, language_code, is_active,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.IsActive = true

	err := r.db.QueryRowxContext(ctx, query,
		user.TelegramID,
		user.Username,
		user.FirstName,
		user.LastName,
		user.LanguageCode,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return ErrDuplicate
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByID получает пользователя по ID
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code,
		       is_active, created_at, updated_at, deleted_at
		FROM users 
		WHERE id = $1 AND deleted_at IS NULL
	`

	var user User
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &user, nil
}

// GetByTelegramID получает пользователя по Telegram ID
func (r *UserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code,
		       is_active, created_at, updated_at, deleted_at
		FROM users 
		WHERE telegram_id = $1 AND deleted_at IS NULL
	`

	var user User
	err := r.db.GetContext(ctx, &user, query, telegramID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by telegram id: %w", err)
	}

	return &user, nil
}

// Update обновляет данные пользователя
func (r *UserRepo) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users 
		SET username = $1, first_name = $2, last_name = $3, language_code = $4,
		    is_active = $5, updated_at = $6
		WHERE id = $7 AND deleted_at IS NULL
	`

	user.UpdatedAt = time.Now()
	result, err := r.db.ExecContext(ctx, query,
		user.Username,
		user.FirstName,
		user.LastName,
		user.LanguageCode,
		user.IsActive,
		user.UpdatedAt,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Upsert создает или обновляет пользователя
func (r *UserRepo) Upsert(ctx context.Context, user *User) error {
	existingUser, err := r.GetByTelegramID(ctx, user.TelegramID)
	if err != nil {
		if err == ErrNotFound {
			// Создаем нового пользователя
			return r.Create(ctx, user)
		}
		return err
	}

	// Обновляем существующего пользователя
	user.ID = existingUser.ID
	return r.Update(ctx, user)
}

// Delete мягко удаляет пользователя (устанавливает deleted_at)
func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	query := `
		UPDATE users 
		SET deleted_at = $1, updated_at = $1, is_active = false
		WHERE id = $2 AND deleted_at IS NULL
	`

	now := time.Now()
	result, err := r.db.ExecContext(ctx, query, now, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// List возвращает список пользователей с пагинацией
func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, language_code,
		       is_active, created_at, updated_at, deleted_at
		FROM users 
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	var users []*User
	err := r.db.SelectContext(ctx, &users, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return users, nil
}

// Count возвращает общее количество активных пользователей
func (r *UserRepo) Count(ctx context.Context) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM users 
		WHERE deleted_at IS NULL AND is_active = true
	`

	var count int
	err := r.db.GetContext(ctx, &count, query)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// GetOrCreate получает пользователя или создает нового если не существует
func (r *UserRepo) GetOrCreate(ctx context.Context, telegramID int64, username, firstName, lastName, languageCode string) (*User, error) {
	user, err := r.GetByTelegramID(ctx, telegramID)
	if err == nil {
		return user, nil
	}

	if err != ErrNotFound {
		return nil, err
	}

	// Создаем нового пользователя
	newUser := &User{
		TelegramID:   telegramID,
		Username:     username,
		FirstName:    firstName,
		LastName:     lastName,
		LanguageCode: languageCode,
	}

	if err := r.Create(ctx, newUser); err != nil {
		return nil, err
	}

	return newUser, nil
}
