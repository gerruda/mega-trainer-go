package telegram

import (
	"context"
	"errors"
	"fmt"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/service"
)

func (h *Handler) startBodyFat(ctx context.Context, chatID, userID int64) error {
	if err := h.state.Start(ctx, userID, commandBodyFat); err != nil {
		return err
	}
	return h.sendWithKeyboard(chatID, "📊 *Процент жира*\n\nВыберите ваш пол:", genderKeyboard())
}

func (h *Handler) handleBodyFatState(ctx context.Context, chatID, userID int64, text string, state *domain.UserState) error {
	switch state.Step {
	case 2:
		return h.collectBodyFatAge(ctx, chatID, userID, text)
	case 3:
		return h.collectBodyFatWeight(ctx, chatID, userID, text)
	case 4:
		return h.collectBodyFatHeight(ctx, chatID, userID, text)
	case 5:
		return h.collectBodyFatNeck(ctx, chatID, userID, text)
	case 6:
		return h.collectBodyFatWaist(ctx, chatID, userID, text)
	case 7:
		return h.collectBodyFatHip(ctx, chatID, userID, text)
	default:
		return h.sendError(chatID, "Не понимаю этот шаг. Начните заново: /bodyfat")
	}
}

func (h *Handler) collectBodyFatAge(ctx context.Context, chatID, userID int64, text string) error {
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
}

func (h *Handler) collectBodyFatWeight(ctx context.Context, chatID, userID int64, text string) error {
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
}

func (h *Handler) collectBodyFatHeight(ctx context.Context, chatID, userID int64, text string) error {
	height, ok := parseDecimal(text, minHeightCM, maxHeightCM)
	if !ok {
		return h.sendError(chatID, fmt.Sprintf("Введите рост числом от %d до %d см.", minHeightCM, maxHeightCM))
	}
	if err := h.state.SetData(ctx, userID, keyHeight, formatDecimal(height)); err != nil {
		return err
	}
	if err := h.state.SetStep(ctx, userID, 5); err != nil {
		return err
	}
	return h.send(chatID, promptNeck)
}

func (h *Handler) collectBodyFatNeck(ctx context.Context, chatID, userID int64, text string) error {
	neck, ok := parseDecimal(text, minBodyCM, maxBodyCM)
	if !ok {
		return h.sendError(chatID, fmt.Sprintf("Введите обхват шеи числом от %d до %d см.", minBodyCM, maxBodyCM))
	}
	if err := h.state.SetData(ctx, userID, keyNeck, formatDecimal(neck)); err != nil {
		return err
	}
	if err := h.state.SetStep(ctx, userID, 6); err != nil {
		return err
	}
	return h.send(chatID, promptWaist)
}

func (h *Handler) collectBodyFatWaist(ctx context.Context, chatID, userID int64, text string) error {
	waist, ok := parseDecimal(text, minBodyCM, maxBodyCM)
	if !ok {
		return h.sendError(chatID, fmt.Sprintf("Введите обхват талии числом от %d до %d см.", minBodyCM, maxBodyCM))
	}
	if err := h.state.SetData(ctx, userID, keyWaist, formatDecimal(waist)); err != nil {
		return err
	}

	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}

	// Only the female formula needs the hip circumference.
	if state.Data[keyGender] == domain.GenderFemale {
		if err := h.state.SetStep(ctx, userID, 7); err != nil {
			return err
		}
		return h.send(chatID, promptHip)
	}

	return h.calculateAndShowBodyFat(ctx, chatID, userID)
}

func (h *Handler) collectBodyFatHip(ctx context.Context, chatID, userID int64, text string) error {
	hip, ok := parseDecimal(text, minBodyCM, maxBodyCM)
	if !ok {
		return h.sendError(chatID, fmt.Sprintf("Введите обхват бёдер числом от %d до %d см.", minBodyCM, maxBodyCM))
	}
	if err := h.state.SetData(ctx, userID, keyHip, formatDecimal(hip)); err != nil {
		return err
	}
	return h.calculateAndShowBodyFat(ctx, chatID, userID)
}

func (h *Handler) calculateAndShowBodyFat(ctx context.Context, chatID, userID int64) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}

	gender := state.Data[keyGender]
	age, ageOK := stateInt(state, keyAge)
	weight, weightOK := stateFloat(state, keyWeight)
	height, heightOK := stateFloat(state, keyHeight)
	neck, neckOK := stateFloat(state, keyNeck)
	waist, waistOK := stateFloat(state, keyWaist)
	if !ageOK || !weightOK || !heightOK || !neckOK || !waistOK {
		return h.sendError(chatID, "Не хватает данных. Начните заново: /bodyfat")
	}

	calculation := domain.BodyFatCalculation{
		Gender: gender,
		Age:    age,
		Weight: weight,
		Height: height,
		Neck:   neck,
		Waist:  waist,
	}
	if gender == domain.GenderFemale {
		hip, hipOK := stateFloat(state, keyHip)
		if !hipOK {
			return h.sendError(chatID, "Не хватает данных. Начните заново: /bodyfat")
		}
		calculation.Hip = hip
	}

	result, err := service.CalculateBodyFat(calculation)
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return h.sendError(chatID, "Проверьте измерения: обхват талии должен быть больше обхвата шеи. Начните заново: /bodyfat")
		}
		h.log.Error().Err(err).Msg("body fat calculation failed")
		return h.sendError(chatID, "Не удалось выполнить расчёт. Начните заново: /bodyfat")
	}

	text := formatBodyFatResult(result)
	if h.recordBodyFatMeasurement(ctx, userID, calculation, result.Result) {
		text += "\n\n💾 Измерение сохранено. Динамика: /chart"
	}

	if err := h.send(chatID, text); err != nil {
		return err
	}
	return h.state.Finish(ctx, userID)
}
