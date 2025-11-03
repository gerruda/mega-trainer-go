package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mega-trainer-go/internal/domain"
)

func (h *Handler) handleStart(message *tgbotapi.Message) error {
	msg := tgbotapi.NewMessage(message.Chat.ID,
		"🏋️ *Фитнес-бот*\n\n"+
			"Я помогу рассчитать различные фитнес-показатели.\n\n"+
			"Доступные команды:\n"+
			"/water - норма воды\n"+
			"/onerepmax - одноповторный максимум\n"+
			"/bodyfat - процент жира\n"+
			"/tdee - суточный расход калорий")
	msg.ParseMode = "Markdown"

	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(message.Chat.ID, sentMsg.MessageID)

	return nil
}

func (h *Handler) showMainMenu(chatID int64) error {
	msg := tgbotapi.NewMessage(chatID,
		"Выберите тип расчета:\n\n"+
			"💧 /water - Норма воды\n"+
			"🏋️ /onerepmax - Одноповторный максимум\n"+
			"📊 /bodyfat - Процент жира\n"+
			"🔥 /tdee - Суточный расход калорий")

	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	return nil
}

func (h *Handler) handleState(ctx context.Context, message *tgbotapi.Message, state *domain.UserState) error {
	switch state.Command {
	case "water":
		return h.handleWaterState(ctx, message, state)
	case "onerepmax":
		return h.handleOneRepMaxState(ctx, message, state)
	case "bodyfat":
		return h.handleBodyFatState(ctx, message, state)
	case "tdee":
		return h.handleTDEESate(ctx, message, state)
	}
	return nil
}

func (h *Handler) sendError(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, "❌ "+text)
	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)
	return nil
}

func (h *Handler) saveMessage(chatID int64, messageID int) {
	h.messageHistory[chatID] = append(h.messageHistory[chatID], messageID)
}

func (h *Handler) cleanupMessages(chatID int64) {
	if messages, exists := h.messageHistory[chatID]; exists {
		for _, msgID := range messages {
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, msgID)
			h.bot.Send(deleteMsg)
		}
		delete(h.messageHistory, chatID)
	}
}
