package telegram

import (
	"context"
	"fmt"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/service"
)

func (h *Handler) startOneRepMax(ctx context.Context, chatID, userID int64) error {
	if err := h.state.Start(ctx, userID, commandOneRepMax); err != nil {
		return err
	}
	return h.sendWithKeyboard(chatID, promptFormula, formulaKeyboard())
}

func (h *Handler) handleOneRepMaxFormula(ctx context.Context, chatID, userID int64, formula string) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}
	if state.Command != commandOneRepMax {
		return h.sendError(chatID, "Эта кнопка уже неактуальна. Начните заново: /onerepmax")
	}
	if _, ok := formulaNames[formula]; !ok {
		return h.sendError(chatID, "Неизвестная формула. Начните заново: /onerepmax")
	}

	// Advance the step and keep the collected data: overwriting the state with an
	// empty payload here would silently drop the chosen formula.
	if err := h.state.SetData(ctx, userID, keyFormula, formula); err != nil {
		return err
	}
	if err := h.state.SetStep(ctx, userID, 2); err != nil {
		return err
	}

	return h.send(chatID, "Введите вес, который можете поднять (кг):")
}

func (h *Handler) handleOneRepMaxState(ctx context.Context, chatID, userID int64, message string, state *domain.UserState) error {
	switch state.Step {
	case 2:
		weight, ok := parseDecimal(message, minWeightKG, maxWeightKG)
		if !ok {
			return h.sendError(chatID, fmt.Sprintf("Введите вес числом от %d до %d кг.", minWeightKG, maxWeightKG))
		}
		if err := h.state.SetData(ctx, userID, keyWeight, formatDecimal(weight)); err != nil {
			return err
		}
		if err := h.state.SetStep(ctx, userID, 3); err != nil {
			return err
		}
		return h.send(chatID, promptReps)

	case 3:
		reps, ok := parseIntInRange(message, minReps, maxReps)
		if !ok {
			return h.sendError(chatID, fmt.Sprintf("Введите число повторений от %d до %d.", minReps, maxReps))
		}
		if err := h.state.SetData(ctx, userID, keyReps, fmt.Sprintf("%d", reps)); err != nil {
			return err
		}
		return h.calculateAndShowOneRepMax(ctx, chatID, userID)

	default:
		return h.sendError(chatID, "Не понимаю этот шаг. Начните заново: /onerepmax")
	}
}

func (h *Handler) calculateAndShowOneRepMax(ctx context.Context, chatID, userID int64) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}

	weight, weightOK := stateFloat(state, keyWeight)
	reps, repsOK := stateInt(state, keyReps)
	formula := state.Data[keyFormula]
	if !weightOK || !repsOK || formula == "" {
		return h.sendError(chatID, "Не хватает данных. Начните заново: /onerepmax")
	}

	result, err := service.CalculateOneRepMax(domain.OneRepMaxCalculation{
		Weight:  weight,
		Reps:    reps,
		Formula: formula,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("one rep max calculation failed")
		return h.sendError(chatID, "Не удалось выполнить расчёт. Начните заново: /onerepmax")
	}

	if err := h.send(chatID, formatOneRepMaxResult(result)); err != nil {
		return err
	}
	return h.state.Finish(ctx, userID)
}
