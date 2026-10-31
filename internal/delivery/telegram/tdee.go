package telegram

import (
	"context"
	"errors"
	"fmt"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/logger"
	"mega-trainer-go/internal/service"
)

func (h *Handler) startTDEE(ctx context.Context, chatID, userID int64) error {
	if err := h.state.Start(ctx, userID, commandTDEE); err != nil {
		return err
	}
	return h.sendWithKeyboard(chatID, "🔥 *Суточный расход калорий*\n\nВыберите ваш пол:", genderKeyboard())
}

func (h *Handler) handleTDEEState(ctx context.Context, chatID, userID int64, text string, state *domain.UserState) error {
	switch state.Step {
	case 2:
		age, ok := parseIntInRange(text, minAgeYears, maxAgeYears)
		if !ok {
			return h.sendError(chatID, fmt.Sprintf("Введите возраст числом от %d до %d лет.", minAgeYears, maxAgeYears))
		}
		if err := h.state.SetData(ctx, userID, keyAge, fmt.Sprintf("%d", age)); err != nil {
			return err
		}
		if err := h.state.SetStep(ctx, userID, 3); err != nil {
			return err
		}
		return h.send(chatID, promptWeight)

	case 3:
		weight, ok := parseDecimal(text, minWeightKG, maxWeightKG)
		if !ok {
			return h.sendError(chatID, fmt.Sprintf("Введите вес числом от %d до %d кг.", minWeightKG, maxWeightKG))
		}
		if err := h.state.SetData(ctx, userID, keyWeight, formatDecimal(weight)); err != nil {
			return err
		}
		if err := h.state.SetStep(ctx, userID, 4); err != nil {
			return err
		}
		return h.send(chatID, promptHeight)

	case 4:
		height, ok := parseDecimal(text, minHeightCM, maxHeightCM)
		if !ok {
			return h.sendError(chatID, fmt.Sprintf("Введите рост числом от %d до %d см.", minHeightCM, maxHeightCM))
		}
		if err := h.state.SetData(ctx, userID, keyHeight, formatDecimal(height)); err != nil {
			return err
		}
		return h.sendWithKeyboard(chatID, "🏃 Выберите уровень активности:", activityKeyboard())

	default:
		return h.sendError(chatID, "Выберите вариант на клавиатуре или начните заново: /tdee")
	}
}

// calculateAndShowTDEE runs the TDEE formula with the collected answers.
func (h *Handler) calculateAndShowTDEE(ctx context.Context, chatID, userID int64) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}

	age, ageOK := stateInt(state, keyAge)
	weight, weightOK := stateFloat(state, keyWeight)
	height, heightOK := stateFloat(state, keyHeight)
	calculation := domain.TDEECalculation{
		Gender:   state.Data[keyGender],
		Age:      age,
		Weight:   weight,
		Height:   height,
		Activity: state.Data[keyActivity],
		Goal:     state.Data[keyGoal],
	}
	if !ageOK || !weightOK || !heightOK {
		return h.sendError(chatID, "Не хватает данных. Начните заново: /tdee")
	}

	result, err := service.CalculateTDEE(calculation)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return h.sendError(chatID, "Не хватает данных. Начните заново: /tdee")
		}
		h.log.Error().Err(err).Msg(logger.Source(logComponent, "calculateAndShowTDEE") + "не удалось рассчитать суточный расход калорий")
		return h.sendError(chatID, "Не удалось выполнить расчёт. Начните заново: /tdee")
	}

	if err := h.send(chatID, formatTDEEResult(result)); err != nil {
		return err
	}
	return h.state.Finish(ctx, userID)
}
