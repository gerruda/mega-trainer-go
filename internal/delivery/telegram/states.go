package telegram

import (
	"context"
	"fmt"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"mega-trainer-go/internal/domain"
)

func (h *Handler) startTDEECalculation(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	chatID := message.Chat.ID

	msg := tgbotapi.NewMessage(chatID, "🔥 *Расчет суточного расхода калорий (TDEE)*\n\nВыберите ваш пол:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Мужской", "gender:male"),
			tgbotapi.NewInlineKeyboardButtonData("Женский", "gender:female"),
		),
	)

	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	return h.stateService.SetUserState(ctx, userID, "tdee", 1, nil)
}

func (h *Handler) handleTDEESate(ctx context.Context, message *tgbotapi.Message, state *domain.UserState) error {
	chatID := message.Chat.ID
	userID := message.From.ID

	switch state.Step {
	case 2: // Ввод возраста
		age, err := strconv.Atoi(message.Text)
		if err != nil || age < 10 || age > 100 {
			return h.sendError(chatID, "Пожалуйста, введите корректный возраст (10-100 лет)")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "age", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		msg := tgbotapi.NewMessage(chatID, "Введите ваш вес (кг):")
		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "tdee", 3, state.Data)

	case 3: // Ввод веса
		weight, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || weight <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный вес")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "weight", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		msg := tgbotapi.NewMessage(chatID, "Введите ваш рост (см):")
		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "tdee", 4, state.Data)

	case 4: // Ввод роста
		height, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || height <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный рост")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "height", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		return h.showActivitySelection(chatID, userID)
	}

	return nil
}

func (h *Handler) handleWaterState(ctx context.Context, message *tgbotapi.Message, state *domain.UserState) error {
	chatID := message.Chat.ID
	userID := message.From.ID

	switch state.Step {
	case 1: // Ввод веса
		weight, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || weight <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный вес (например: 70.5)")
		}

		// Сохраняем вес
		if err := h.stateService.UpdateUserStateData(ctx, userID, "weight", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		// Показываем выбор активности
		msg := tgbotapi.NewMessage(chatID, "Выберите уровень активности:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Низкая", "water_activity:low"),
				tgbotapi.NewInlineKeyboardButtonData("Средняя", "water_activity:medium"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Высокая", "water_activity:high"),
			),
		)

		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "water", 2, nil)
	}

	return nil
}

func (h *Handler) startBodyFatCalculation(ctx context.Context, message *tgbotapi.Message) error {
	userID := message.From.ID
	chatID := message.Chat.ID

	// Показываем выбор пола
	msg := tgbotapi.NewMessage(chatID, "👤 *Расчет процента жира в организме*\n\nВыберите ваш пол:")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Мужской", "gender:male"),
			tgbotapi.NewInlineKeyboardButtonData("Женский", "gender:female"),
		),
	)

	sentMsg, _ := h.bot.Send(msg)
	h.saveMessage(chatID, sentMsg.MessageID)

	return h.stateService.SetUserState(ctx, userID, "bodyfat", 1, nil)
}

func (h *Handler) handleBodyFatState(ctx context.Context, message *tgbotapi.Message, state *domain.UserState) error {
	chatID := message.Chat.ID
	userID := message.From.ID

	switch state.Step {
	case 2: // Ввод возраста
		age, err := strconv.Atoi(message.Text)
		if err != nil || age < 10 || age > 100 {
			return h.sendError(chatID, "Пожалуйста, введите корректный возраст (10-100 лет)")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "age", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		msg := tgbotapi.NewMessage(chatID, "Введите ваш вес (кг):")
		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "bodyfat", 3, state.Data)

	case 3: // Ввод веса
		weight, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || weight <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный вес")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "weight", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		msg := tgbotapi.NewMessage(chatID, "Введите ваш рост (см):")
		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "bodyfat", 4, state.Data)

	case 4: // Ввод роста
		height, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || height <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный рост")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "height", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		msg := tgbotapi.NewMessage(chatID, "Введите обхват шеи (см):")
		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "bodyfat", 5, state.Data)

	case 5: // Ввод обхвата шеи
		neck, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || neck <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный обхват шеи")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "neck", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		msg := tgbotapi.NewMessage(chatID, "Введите обхват талии (см):")
		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "bodyfat", 6, state.Data)

	case 6: // Ввод обхвата талии
		waist, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || waist <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный обхват талии")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "waist", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		// Для женщин запрашиваем обхват бедер
		if state.Data["gender"] == "female" {
			msg := tgbotapi.NewMessage(chatID, "Введите обхват бедер (см):")
			sentMsg, _ := h.bot.Send(msg)
			h.saveMessage(chatID, sentMsg.MessageID)
			return h.stateService.SetUserState(ctx, userID, "bodyfat", 7, state.Data)
		}

		// Для мужчин сразу рассчитываем
		return h.calculateAndShowBodyFat(ctx, userID, chatID)

	case 7: // Ввод обхвата бедер (только для женщин)
		hip, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || hip <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный обхват бедер")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "hip", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		return h.calculateAndShowBodyFat(ctx, userID, chatID)
	}

	return nil
}

func (h *Handler) calculateAndShowBodyFat(ctx context.Context, userID int64, chatID int64) error {
	state, err := h.stateService.GetUserState(ctx, userID)
	if err != nil || state == nil {
		return h.sendError(chatID, "Сессия устарела, начните заново")
	}

	// Парсим данные
	age, _ := strconv.Atoi(state.Data["age"])
	weight, _ := strconv.ParseFloat(state.Data["weight"], 64)
	height, _ := strconv.ParseFloat(state.Data["height"], 64)
	neck, _ := strconv.ParseFloat(state.Data["neck"], 64)
	waist, _ := strconv.ParseFloat(state.Data["waist"], 64)

	calculation := &domain.BodyFatCalculation{
		Gender: state.Data["gender"],
		Age:    age,
		Weight: weight,
		Height: height,
		Neck:   neck,
		Waist:  waist,
	}

	// Для женщин добавляем обхват бедер
	if state.Data["gender"] == "female" {
		hip, _ := strconv.ParseFloat(state.Data["hip"], 64)
		calculation.Hip = hip
	}

	result := h.calculatorService.CalculateBodyFat(calculation)

	// Форматируем результат
	msg := tgbotapi.NewMessage(chatID, h.formatBodyFatResult(result))
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)

	// Очищаем состояние
	return h.stateService.ClearUserState(ctx, userID)
}

func (h *Handler) formatBodyFatResult(calculation *domain.BodyFatCalculation) string {
	var category string
	fat := calculation.Result

	if calculation.Gender == "male" {
		switch {
		case fat < 6:
			category = "❌ Слишком низкий (опасно для здоровья)"
		case fat < 14:
			category = "🏆 Атлетическое телосложение"
		case fat < 18:
			category = "👍 Спортивное телосложение"
		case fat < 25:
			category = "✅ Нормальное телосложение"
		case fat < 30:
			category = "⚠️ Избыточный вес"
		default:
			category = "🚨 Ожирение"
		}
	} else {
		switch {
		case fat < 14:
			category = "❌ Слишком низкий (опасно для здоровья)"
		case fat < 21:
			category = "🏆 Атлетическое телосложение"
		case fat < 25:
			category = "👍 Спортивное телосложение"
		case fat < 32:
			category = "✅ Нормальное телосложение"
		case fat < 38:
			category = "⚠️ Избыточный вес"
		default:
			category = "🚨 Ожирение"
		}
	}

	return fmt.Sprintf(
		"📊 *Результат расчета процента жира*\n\n"+
			"• Пол: %s\n"+
			"• Возраст: %d лет\n"+
			"• Вес: %.1f кг\n"+
			"• Рост: %.1f см\n\n"+
			"💡 *Процент жира в организме:*\n"+
			"**%.1f%%**\n\n"+
			"📈 *Категория:* %s\n\n"+
			"Для нового расчета используйте команды из меню.",
		h.getGenderName(calculation.Gender),
		calculation.Age,
		calculation.Weight,
		calculation.Height,
		calculation.Result,
		category,
	)
}

func (h *Handler) handleOneRepMaxState(ctx context.Context, message *tgbotapi.Message, state *domain.UserState) error {
	chatID := message.Chat.ID
	userID := message.From.ID

	switch state.Step {
	case 2: // Ввод веса
		weight, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || weight <= 0 {
			return h.sendError(chatID, "Пожалуйста, введите корректный вес (например: 80.5)")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "weight", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		msg := tgbotapi.NewMessage(chatID, "Введите количество повторений (1-10):")
		sentMsg, _ := h.bot.Send(msg)
		h.saveMessage(chatID, sentMsg.MessageID)

		return h.stateService.SetUserState(ctx, userID, "onerepmax", 3, state.Data)

	case 3: // Ввод количества повторений
		reps, err := strconv.Atoi(message.Text)
		if err != nil || reps < 1 || reps > 10 {
			return h.sendError(chatID, "Пожалуйста, введите корректное количество повторений (1-10)")
		}

		if err := h.stateService.UpdateUserStateData(ctx, userID, "reps", message.Text); err != nil {
			return h.sendError(chatID, "Ошибка сохранения данных")
		}

		// Выполняем расчет
		return h.calculateAndShowOneRepMax(ctx, userID, chatID)
	}

	return nil
}

func (h *Handler) calculateAndShowOneRepMax(ctx context.Context, userID int64, chatID int64) error {
	state, err := h.stateService.GetUserState(ctx, userID)
	if err != nil || state == nil {
		return h.sendError(chatID, "Сессия устарела, начните заново")
	}

	// Парсим данные
	weight, _ := strconv.ParseFloat(state.Data["weight"], 64)
	reps, _ := strconv.Atoi(state.Data["reps"])
	formula := state.Data["formula"]

	calculation := &domain.OneRepMaxCalculation{
		Weight:  weight,
		Reps:    reps,
		Formula: formula,
	}

	result := h.calculatorService.CalculateOneRepMax(calculation)

	// Форматируем результат
	msg := tgbotapi.NewMessage(chatID, h.formatOneRepMaxResult(result))
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)

	// Очищаем состояние
	return h.stateService.ClearUserState(ctx, userID)
}

func (h *Handler) formatOneRepMaxResult(calculation *domain.OneRepMaxCalculation) string {
	formulaNames := map[string]string{
		"epley":    "Эпли",
		"brzycki":  "Бжицки",
		"lombardi": "Ломбарди",
	}

	return fmt.Sprintf(
		"🏋️ *Результат расчета одноповторного максимума*\n\n"+
			"• Поднятый вес: %.1f кг\n"+
			"• Количество повторений: %d\n"+
			"• Использованная формула: %s\n\n"+
			"💪 *Ваш одноповторный максимум:*\n"+
			"**%.1f кг**\n\n"+
			"📊 *Также рассчитываем по другим формулам:*\n"+
			"• Эпли: %.1f кг\n"+
			"• Бжицки: %.1f кг\n"+
			"• Ломбарди: %.1f кг\n\n"+
			"Для нового расчета используйте команды из меню.",
		calculation.Weight,
		calculation.Reps,
		formulaNames[calculation.Formula],
		calculation.Result,
		h.calculatorService.CalculateOneRepMax(&domain.OneRepMaxCalculation{
			Weight: calculation.Weight, Reps: calculation.Reps, Formula: "epley",
		}).Result,
		h.calculatorService.CalculateOneRepMax(&domain.OneRepMaxCalculation{
			Weight: calculation.Weight, Reps: calculation.Reps, Formula: "brzycki",
		}).Result,
		h.calculatorService.CalculateOneRepMax(&domain.OneRepMaxCalculation{
			Weight: calculation.Weight, Reps: calculation.Reps, Formula: "lombardi",
		}).Result,
	)
}
