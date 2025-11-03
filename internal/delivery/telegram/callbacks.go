package telegram

import (
	"context"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mega-trainer-go/internal/domain"
)

func (h *Handler) handleGenderSelection(ctx context.Context, userID int64, chatID int64, gender string) error {
	// Получаем текущее состояние
	state, err := h.stateService.GetUserState(ctx, userID)
	if err != nil || state == nil {
		return h.sendError(chatID, "Сессия устарела, начните заново")
	}

	// Сохраняем выбранный пол
	if err := h.stateService.UpdateUserStateData(ctx, userID, "gender", gender); err != nil {
		return h.sendError(chatID, "Ошибка сохранения данных")
	}

	// В зависимости от команды переходим к следующему шагу
	switch state.Command {
	case "bodyfat":
		return h.handleBodyFatAfterGender(ctx, userID, chatID, gender)
	case "tdee":
		return h.handleTDEEDataInput(ctx, userID, chatID, "age")
	}

	return nil
}

func (h *Handler) handleActivitySelection(ctx context.Context, userID int64, chatID int64, activity string) error {
	state, err := h.stateService.GetUserState(ctx, userID)
	if err != nil || state == nil {
		return h.sendError(chatID, "Сессия устарела, начните заново")
	}

	if err := h.stateService.UpdateUserStateData(ctx, userID, "activity", activity); err != nil {
		return h.sendError(chatID, "Ошибка сохранения данных")
	}

	switch state.Command {
	case "tdee":
		return h.showGoalSelection(chatID, userID)
	}

	return nil
}

func (h *Handler) handleGoalSelection(ctx context.Context, userID int64, chatID int64, goal string) error {
	state, err := h.stateService.GetUserState(ctx, userID)
	if err != nil || state == nil {
		return h.sendError(chatID, "Сессия устарела, начните заново")
	}

	if err := h.stateService.UpdateUserStateData(ctx, userID, "goal", goal); err != nil {
		return h.sendError(chatID, "Ошибка сохранения данных")
	}

	switch state.Command {
	case "tdee":
		return h.calculateAndShowTDEE(ctx, userID, chatID)
	}

	return nil
}

// Вспомогательные методы для Body Fat
func (h *Handler) handleBodyFatAfterGender(ctx context.Context, userID int64, chatID int64, gender string) error {
	msg := tgbotapi.NewMessage(chatID, "Введите ваш возраст:")
	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	return h.stateService.SetUserState(ctx, userID, "bodyfat", 2, nil)
}

// Вспомогательные методы для TDEE
func (h *Handler) handleTDEEDataInput(ctx context.Context, userID int64, chatID int64, field string) error {
	var message string
	switch field {
	case "age":
		message = "Введите ваш возраст:"
	case "weight":
		message = "Введите ваш вес (кг):"
	case "height":
		message = "Введите ваш рост (см):"
	}

	msg := tgbotapi.NewMessage(chatID, message)
	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	// Обновляем состояние для следующего шага
	currentState, _ := h.stateService.GetUserState(ctx, userID)
	var nextStep int
	switch field {
	case "age":
		nextStep = 2
	case "weight":
		nextStep = 3
	case "height":
		nextStep = 4
	}

	return h.stateService.SetUserState(ctx, userID, currentState.Command, nextStep, currentState.Data)
}

func (h *Handler) showActivitySelection(chatID int64, userID int64) error {
	msg := tgbotapi.NewMessage(chatID, "🏃‍♂️ Выберите уровень активности:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Сидячий", "activity:sedentary"),
			tgbotapi.NewInlineKeyboardButtonData("Легкая", "activity:light"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Умеренная", "activity:moderate"),
			tgbotapi.NewInlineKeyboardButtonData("Высокая", "activity:active"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Очень высокая", "activity:very_active"),
		),
	)

	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)
	return nil
}

func (h *Handler) showGoalSelection(chatID int64, userID int64) error {
	msg := tgbotapi.NewMessage(chatID, "🎯 Выберите вашу цель:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Похудение", "goal:loss"),
			tgbotapi.NewInlineKeyboardButtonData("Поддержание", "goal:maintenance"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Набор массы", "goal:gain"),
		),
	)

	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)
	return nil
}

func (h *Handler) calculateAndShowTDEE(ctx context.Context, userID int64, chatID int64) error {
	state, err := h.stateService.GetUserState(ctx, userID)
	if err != nil || state == nil {
		return h.sendError(chatID, "Сессия устарела, начните заново")
	}

	// Парсим данные
	age, _ := strconv.Atoi(state.Data["age"])
	weight, _ := strconv.ParseFloat(state.Data["weight"], 64)
	height, _ := strconv.ParseFloat(state.Data["height"], 64)

	calculation := &domain.TDEECalculation{
		Gender:   state.Data["gender"],
		Age:      age,
		Weight:   weight,
		Height:   height,
		Activity: state.Data["activity"],
		Goal:     state.Data["goal"],
	}

	result := h.calculatorService.CalculateTDEE(calculation)

	// Форматируем результат
	msg := tgbotapi.NewMessage(chatID, h.formatTDEEResult(result))
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)

	// Очищаем состояние
	return h.stateService.ClearUserState(ctx, userID)
}

func (h *Handler) formatTDEEResult(calculation *domain.TDEECalculation) string {
	activityNames := map[string]string{
		"sedentary":   "Сидячий образ жизни",
		"light":       "Легкая активность (1-3 тренировки в неделю)",
		"moderate":    "Умеренная активность (3-5 тренировок в неделю)",
		"active":      "Высокая активность (6-7 тренировок в неделю)",
		"very_active": "Очень высокая активность (профессиональные спортсмены)",
	}

	goalNames := map[string]string{
		"loss":        "Похудение",
		"maintenance": "Поддержание веса",
		"gain":        "Набор массы",
	}

	return fmt.Sprintf(
		"🔥 *Результат расчета TDEE*\n\n"+
			"• Пол: %s\n"+
			"• Возраст: %d лет\n"+
			"• Вес: %.1f кг\n"+
			"• Рост: %.1f см\n"+
			"• Активность: %s\n"+
			"• Цель: %s\n\n"+
			"💡 *Ваш суточный расход калорий:*\n"+
			"**%.0f ккал/день**\n\n"+
			"Для поддержания веса: %.0f ккал/день\n"+
			"Для набора массы: %.0f ккал/день\n"+
			"Для похудения: %.0f ккал/день",
		h.getGenderName(calculation.Gender),
		calculation.Age,
		calculation.Weight,
		calculation.Height,
		activityNames[calculation.Activity],
		goalNames[calculation.Goal],
		calculation.Result,
		calculation.Result/1.15, // Базовый TDEE без учета цели
		calculation.Result*1.15,
		calculation.Result*0.85,
	)
}

func (h *Handler) getGenderName(gender string) string {
	if gender == "male" {
		return "Мужской"
	}
	return "Женский"
}
