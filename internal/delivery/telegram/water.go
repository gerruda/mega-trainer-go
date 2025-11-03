package telegram

import (
	"context"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mega-trainer-go/internal/domain"
)

func (h *Handler) startWaterCalculation(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	chatID := message.Chat.ID

	// Устанавливаем состояние
	if err := h.stateService.SetUserState(ctx, userID, "water", 1, nil); err != nil {
		return h.sendError(chatID, "Ошибка начала расчета")
	}

	msg := tgbotapi.NewMessage(chatID, "💧 *Расчет нормы воды*\n\nВведите ваш вес в кг:")
	msg.ParseMode = "Markdown"
	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	return nil
}

func (h *Handler) handleWaterActivity(ctx context.Context, userID int64, chatID int64, activity string) error {
	// Получаем данные из состояния
	state, err := h.stateService.GetUserState(ctx, userID)
	if err != nil || state == nil {
		return h.sendError(chatID, "Сессия устарела, начните заново")
	}

	weight, err := strconv.ParseFloat(state.Data["weight"], 64)
	if err != nil {
		return h.sendError(chatID, "Неверный формат веса")
	}

	// Выполняем расчет
	calculation := &domain.WaterCalculation{
		Weight:   weight,
		Activity: activity,
	}

	result := h.calculatorService.CalculateWater(calculation)

	// Отправляем результат
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
		"💧 *Результат расчета:*\n\n"+
			"• Вес: %.1f кг\n"+
			"• Уровень активности: %s\n"+
			"• Рекомендуемая норма воды: *%.2f литров/день*\n\n"+
			"Для нового расчета используйте команды из меню.",
		result.Weight,
		h.getActivityName(activity),
		result.Result,
	))
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)

	// Очищаем состояние
	return h.stateService.ClearUserState(ctx, userID)
}

func (h *Handler) getActivityName(activity string) string {
	names := map[string]string{
		"low":    "Низкая",
		"medium": "Средняя",
		"high":   "Высокая",
	}
	return names[activity]
}
