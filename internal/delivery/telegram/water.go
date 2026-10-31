package telegram

import (
	"context"
	"fmt"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/logger"
	"mega-trainer-go/internal/service"
)

func (h *Handler) startWater(ctx context.Context, chatID, userID int64) error {
	if err := h.state.Start(ctx, userID, commandWater); err != nil {
		return err
	}
	return h.send(chatID, promptWater)
}

func (h *Handler) handleWaterState(ctx context.Context, chatID, userID int64, message string, state *domain.UserState) error {
	if state.Step != 1 {
		return h.sendError(chatID, "Выберите уровень активности на клавиатуре или начните заново: /water")
	}

	weight, ok := parseDecimal(message, minWeightKG, maxWeightKG)
	if !ok {
		return h.sendError(chatID, fmt.Sprintf("Введите вес числом от %d до %d кг.", minWeightKG, maxWeightKG))
	}

	if err := h.state.SetData(ctx, userID, keyWeight, formatDecimal(weight)); err != nil {
		return err
	}
	if err := h.state.SetStep(ctx, userID, 2); err != nil {
		return err
	}

	return h.sendWithKeyboard(chatID, "Выберите уровень активности:", waterActivityKeyboard())
}

func (h *Handler) handleWaterActivity(ctx context.Context, chatID, userID int64, activity string) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}
	if state.Command != commandWater {
		return h.sendError(chatID, "Эта кнопка уже неактуальна. Начните заново: /water")
	}

	weight, ok := stateFloat(state, keyWeight)
	if !ok {
		return h.sendError(chatID, "Не хватает данных. Начните заново: /water")
	}

	result, err := service.CalculateWater(domain.WaterCalculation{Weight: weight, Activity: activity})
	if err != nil {
		h.log.Error().Err(err).Str("activity", activity).Msg(logger.Source(logComponent, "handleWaterActivity") + "не удалось рассчитать норму воды")
		return h.sendError(chatID, "Не удалось выполнить расчёт. Начните заново: /water")
	}

	if err := h.send(chatID, formatWaterResult(result)); err != nil {
		return err
	}
	return h.state.Finish(ctx, userID)
}
