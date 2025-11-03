package telegram

import (
	"context"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mega-trainer-go/internal/service"
)

type Handler struct {
	bot               *tgbotapi.BotAPI
	calculatorService *service.CalculatorService
	stateService      *service.StateService
	messageHistory    map[int64][]int // Храним ID сообщений для удаления
}

func NewHandler(
	bot *tgbotapi.BotAPI,
	calculatorService *service.CalculatorService,
	stateService *service.StateService,
) *Handler {
	return &Handler{
		bot:               bot,
		calculatorService: calculatorService,
		stateService:      stateService,
		messageHistory:    make(map[int64][]int),
	}
}

func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	// Работаем только в личных сообщениях
	if update.Message != nil && !update.Message.Chat.IsPrivate() {
		return nil
	}

	if update.CallbackQuery != nil {
		return h.handleCallback(ctx, update.CallbackQuery)
	}

	if update.Message != nil {
		return h.handleMessage(ctx, update.Message)
	}

	return nil
}

func (h *Handler) handleMessage(ctx context.Context, message *tgbotapi.Message) error {
	// Очищаем предыдущие сообщения
	h.cleanupMessages(message.Chat.ID)

	// Проверяем состояние пользователя
	state, err := h.stateService.GetUserState(ctx, message.From.ID)
	if err != nil {
		return h.sendError(message.Chat.ID, "Ошибка получения состояния")
	}

	if state != nil {
		return h.handleState(ctx, message, state)
	}

	// Обработка команд
	switch message.Command() {
	case "start":
		return h.handleStart(message)
	case "water":
		return h.startWaterCalculation(ctx, message)
	case "onerepmax":
		return h.startOneRepMaxCalculation(ctx, message)
	case "bodyfat":
		return h.startBodyFatCalculation(ctx, message)
	case "tdee":
		return h.startTDEECalculation(ctx, message)
	default:
		return h.showMainMenu(message.Chat.ID)
	}
}

func (h *Handler) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	chatID := callback.Message.Chat.ID
	userID := callback.From.ID

	// Очищаем предыдущие сообщения
	h.cleanupMessages(chatID)

	data := strings.Split(callback.Data, ":")
	if len(data) < 2 {
		return h.sendError(chatID, "Неверный формат callback")
	}

	command := data[0]
	value := data[1]

	switch command {
	case "water_activity":
		return h.handleWaterActivity(ctx, userID, chatID, value)
	case "onerepmax_formula":
		return h.handleOneRepMaxFormula(ctx, userID, chatID, value)
	case "gender":
		return h.handleGenderSelection(ctx, userID, chatID, value)
	case "activity":
		return h.handleActivitySelection(ctx, userID, chatID, value)
	case "goal":
		return h.handleGoalSelection(ctx, userID, chatID, value)
	}

	return nil
}
