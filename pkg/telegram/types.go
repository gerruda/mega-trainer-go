package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mega-trainer-go/internal/storage/postgres"
)

// UserService интерфейс для работы с пользователями
type UserService interface {
	GetOrCreateUser(ctx context.Context, userID int64, username, firstName, lastName, languageCode string) (*User, error)
	GetUser(ctx context.Context, userID int64) (*User, error)
	SaveMeasurement(ctx context.Context, userID int64, measurement *postgres.BodyMeasurement) error
	GetLastMeasurements(ctx context.Context, userID int64, limit int) ([]*postgres.BodyMeasurement, error)
}

// TelegramService интерфейс для работы с Telegram API
type TelegramService interface {
	SendMessage(chatID int64, text string) error
	SendMessageWithKeyboard(chatID int64, text string, keyboard interface{}) error
	AnswerCallback(callbackID, text string) error
	EditMessageText(chatID int64, messageID int, text string) error
}

// User структура пользователя
type User struct {
	ID           int64  `json:"id"`
	TelegramID   int64  `json:"telegram_id"`
	Username     string `json:"username"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	LanguageCode string `json:"language_code"`
}

// BodyMeasurement структура для хранения данных измерений
type BodyMeasurement struct {
	Height   float64 `json:"height"`
	Weight   float64 `json:"weight"`
	Neck     float64 `json:"neck"`
	Waist    float64 `json:"waist"`
	Hips     float64 `json:"hips"`
	BMI      float64 `json:"bmi"`
	BMR      float64 `json:"bmr"`
	TDEE     float64 `json:"tdee"`
	Calories int     `json:"calories"`
	Protein  int     `json:"protein"`
	Fat      int     `json:"fat"`
	Carbs    int     `json:"carbs"`
}

// MessageHandler обработчик текстовых сообщений в FSM
type MessageHandler func(ctx context.Context, message *tgbotapi.Message) error

// CallbackHandler обработчик callback запросов
type CallbackHandler func(ctx context.Context, callback *tgbotapi.CallbackQuery) error
