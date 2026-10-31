package telegram

import "context"

// handleGender stores the gender selected on the inline keyboard and moves the
// dialog to the next step of whichever flow is active.
func (h *Handler) handleGender(ctx context.Context, chatID, userID int64, gender string) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}
	if _, ok := genderNames[gender]; !ok {
		return h.sendError(chatID, "Неизвестный вариант. Начните заново.")
	}

	switch state.Command {
	case commandTDEE, commandBodyFat:
	default:
		return h.sendError(chatID, "Эта кнопка уже неактуальна. Начните заново.")
	}

	if err := h.state.SetData(ctx, userID, keyGender, gender); err != nil {
		return err
	}
	if err := h.state.SetStep(ctx, userID, 2); err != nil {
		return err
	}

	if state.Command == commandBodyFat {
		return h.send(chatID, promptBodyFat)
	}
	return h.send(chatID, promptTDEE)
}

// handleActivity stores the activity level selected for the TDEE flow.
func (h *Handler) handleActivity(ctx context.Context, chatID, userID int64, activity string) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}
	if state.Command != commandTDEE {
		return h.sendError(chatID, "Эта кнопка уже неактуальна. Начните заново: /tdee")
	}
	if _, ok := activityNames[activity]; !ok {
		return h.sendError(chatID, "Неизвестный уровень активности. Начните заново: /tdee")
	}

	if err := h.state.SetData(ctx, userID, keyActivity, activity); err != nil {
		return err
	}
	return h.sendWithKeyboard(chatID, "🎯 Выберите вашу цель:", goalKeyboard())
}

// handleGoal stores the goal selected for the TDEE flow and shows the result.
func (h *Handler) handleGoal(ctx context.Context, chatID, userID int64, goal string) error {
	state, err := h.activeState(ctx, chatID, userID)
	if err != nil || state == nil {
		return err
	}
	if state.Command != commandTDEE {
		return h.sendError(chatID, "Эта кнопка уже неактуальна. Начните заново: /tdee")
	}
	if _, ok := goalNames[goal]; !ok {
		return h.sendError(chatID, "Неизвестная цель. Начните заново: /tdee")
	}

	if err := h.state.SetData(ctx, userID, keyGoal, goal); err != nil {
		return err
	}
	return h.calculateAndShowTDEE(ctx, chatID, userID)
}
