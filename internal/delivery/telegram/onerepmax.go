package telegram

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) startOneRepMaxCalculation(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	chatID := message.Chat.ID

	// Показываем выбор формулы
	msg := tgbotapi.NewMessage(chatID, "🏋️ *Расчет одноповторного максимума*\n\nВыберите формулу расчета:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Эпли", "onerepmax_formula:epley"),
			tgbotapi.NewInlineKeyboardButtonData("Бжицки", "onerepmax_formula:brzycki"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Ломбарди", "onerepmax_formula:lombardi"),
		),
	)

	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	return h.stateService.SetUserState(ctx, userID, "onerepmax", 1, nil)
}

func (h *Handler) handleOneRepMaxFormula(ctx context.Context, userID int64, chatID int64, formula string) error {
	// Сохраняем выбранную формулу
	if err := h.stateService.UpdateUserStateData(ctx, userID, "formula", formula); err != nil {
		return h.sendError(chatID, "Ошибка сохранения данных")
	}

	msg := tgbotapi.NewMessage(chatID, "Введите вес, который можете поднять (в кг):")
	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	return h.stateService.SetUserState(ctx, userID, "onerepmax", 2, nil)
}
