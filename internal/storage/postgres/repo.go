package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound  = errors.New("record not found")
	ErrDuplicate = errors.New("duplicate record")
)

// Repository базовый интерфейс репозитория
type Repository interface {
	HealthCheck(ctx context.Context) error
	Close() error
}

// BaseRepository базовая структура репозитория
type BaseRepository struct {
	db *sqlx.DB
}

// NewBaseRepository создает новый базовый репозиторий
func NewBaseRepository(db *sqlx.DB) *BaseRepository {
	return &BaseRepository{db: db}
}

// HealthCheck проверяет соединение с БД
func (r *BaseRepository) HealthCheck(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close закрывает соединение с БД
func (r *BaseRepository) Close() error {
	return r.db.Close()
}

// DB возвращает соединение с БД
func (r *BaseRepository) DB() *sqlx.DB {
	return r.db
}

// UserRepository интерфейс для работы с пользователями
type UserRepository interface {
	Repository

	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	Update(ctx context.Context, user *User) error
	Upsert(ctx context.Context, user *User) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*User, error)
	Count(ctx context.Context) (int, error)
}

// ChatRepository интерфейс для работы с чатами
type ChatRepository interface {
	Repository

	Create(ctx context.Context, chat *Chat) error
	GetByID(ctx context.Context, id int64) (*Chat, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*Chat, error)
	Update(ctx context.Context, chat *Chat) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*Chat, error)
}

// BodyMeasurementRepository интерфейс для работы с измерениями тела
type BodyMeasurementRepository interface {
	Repository

	Create(ctx context.Context, measurement *BodyMeasurement) error
	GetByID(ctx context.Context, id int64) (*BodyMeasurement, error)
	GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*BodyMeasurement, error)
	GetLastByUserID(ctx context.Context, userID int64, count int) ([]*BodyMeasurement, error)
	Delete(ctx context.Context, id int64) error
	CountByUserID(ctx context.Context, userID int64) (int, error)
}

// UserSettingsRepository интерфейс для работы с настройками пользователя
type UserSettingsRepository interface {
	Repository

	GetByUserID(ctx context.Context, userID int64) (*UserSettings, error)
	Update(ctx context.Context, settings *UserSettings) error
	Upsert(ctx context.Context, settings *UserSettings) error
}

// Модели данных

// User представляет пользователя бота
type User struct {
	ID           int64        `db:"id"`
	TelegramID   int64        `db:"telegram_id"`
	Username     string       `db:"username"`
	FirstName    string       `db:"first_name"`
	LastName     string       `db:"last_name"`
	LanguageCode string       `db:"language_code"`
	IsActive     bool         `db:"is_active"`
	CreatedAt    time.Time    `db:"created_at"`
	UpdatedAt    time.Time    `db:"updated_at"`
	DeletedAt    sql.NullTime `db:"deleted_at"`
}

// Chat представляет чат/канал
type Chat struct {
	ID          int64        `db:"id"`
	TelegramID  int64        `db:"telegram_id"`
	Type        string       `db:"type"` // "private", "group", "supergroup", "channel"
	Title       string       `db:"title"`
	Username    string       `db:"username"`
	FirstName   string       `db:"first_name"`
	LastName    string       `db:"last_name"`
	Description string       `db:"description"`
	IsActive    bool         `db:"is_active"`
	CreatedAt   time.Time    `db:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"`
	DeletedAt   sql.NullTime `db:"deleted_at"`
}

// BodyMeasurement представляет измерения тела пользователя
type BodyMeasurement struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Weight    float64   `db:"weight"`
	Height    float64   `db:"height"`
	Neck      float64   `db:"neck"`
	Waist     float64   `db:"waist"`
	Hips      float64   `db:"hips"`
	BMI       float64   `db:"bmi"`
	BMR       float64   `db:"bmr"`
	TDEE      float64   `db:"tdee"`
	Calories  int       `db:"calories"`
	Protein   int       `db:"protein"`
	Fat       int       `db:"fat"`
	Carbs     int       `db:"carbs"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// UserSettings представляет настройки пользователя
type UserSettings struct {
	ID                int64     `db:"id"`
	UserID            int64     `db:"user_id"`
	MeasurementSystem string    `db:"measurement_system"` // "metric", "imperial"
	Notifications     bool      `db:"notifications_enabled"`
	Language          string    `db:"language"`
	DailyCalorieGoal  int       `db:"daily_calorie_goal"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}
