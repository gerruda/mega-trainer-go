package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"

	"mega-trainer-go/internal/domain"
)

// UserRepo is the PostgreSQL implementation of domain.UserRepository.
type UserRepo struct {
	db *sqlx.DB
}

// compile-time check that the implementation satisfies the contract.
var _ domain.UserRepository = (*UserRepo)(nil)

// NewUserRepo creates a user repository.
func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

const upsertUserQuery = `
	INSERT INTO users (telegram_id, username, first_name, last_name, language_code)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (telegram_id) DO UPDATE SET
		username      = EXCLUDED.username,
		first_name    = EXCLUDED.first_name,
		last_name     = EXCLUDED.last_name,
		language_code = EXCLUDED.language_code,
		updated_at    = NOW()
	RETURNING id, telegram_id, username, first_name, last_name, language_code,
	          is_active, created_at, updated_at, deleted_at
`

// Upsert inserts the user or refreshes the Telegram profile fields.
func (r *UserRepo) Upsert(ctx context.Context, user *domain.User) (*domain.User, error) {
	if user == nil {
		return nil, errors.New("upsert user: nil user")
	}

	var stored domain.User
	err := r.db.GetContext(ctx, &stored, upsertUserQuery,
		user.TelegramID,
		user.Username,
		user.FirstName,
		user.LastName,
		user.LanguageCode,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert user %d: %w", user.TelegramID, err)
	}

	return &stored, nil
}

const getUserByTelegramIDQuery = `
	SELECT id, telegram_id, username, first_name, last_name, language_code,
	       is_active, created_at, updated_at, deleted_at
	FROM users
	WHERE telegram_id = $1 AND deleted_at IS NULL
`

// GetByTelegramID returns a user by their Telegram ID.
func (r *UserRepo) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user, getUserByTelegramIDQuery, telegramID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get user %d: %w", telegramID, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("get user %d: %w", telegramID, err)
	}

	return &user, nil
}

const listActiveUsersQuery = `
	SELECT id, telegram_id, username, first_name, last_name, language_code,
	       is_active, created_at, updated_at, deleted_at
	FROM users
	WHERE is_active AND deleted_at IS NULL
	ORDER BY id
	LIMIT $1
`

// ListActive returns active users, oldest first, limited by limit.
func (r *UserRepo) ListActive(ctx context.Context, limit int) ([]*domain.User, error) {
	if limit <= 0 {
		limit = 1000
	}

	var users []*domain.User
	if err := r.db.SelectContext(ctx, &users, listActiveUsersQuery, limit); err != nil {
		return nil, fmt.Errorf("list active users: %w", err)
	}

	return users, nil
}
