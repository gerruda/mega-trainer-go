package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/logger"
)

// Validation bounds shared by all dialogs. They also protect the formulas from
// division by zero and logarithms of non-positive numbers.
const (
	minWeightKG = 20
	maxWeightKG = 400
	minHeightCM = 100
	maxHeightCM = 250
	minAgeYears = 10
	maxAgeYears = 120
	minBodyCM   = 20
	maxBodyCM   = 200
	minReps     = 1
	maxReps     = 30
)

// send sends a Markdown formatted message and remembers it for later cleanup.
func (h *Handler) send(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	return h.dispatch(chatID, msg)
}

// sendWithKeyboard sends a Markdown message with an inline keyboard.
func (h *Handler) sendWithKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = keyboard
	return h.dispatch(chatID, msg)
}

// sendPhoto uploads a PNG image with a caption.
func (h *Handler) sendPhoto(chatID int64, png []byte, caption string) error {
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FileBytes{Name: "chart.png", Bytes: png})
	photo.Caption = caption
	photo.ParseMode = tgbotapi.ModeMarkdown
	return h.dispatch(chatID, photo)
}

// sendDocument uploads a file with a caption.
func (h *Handler) sendDocument(chatID int64, name string, data []byte, caption string) error {
	document := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{Name: name, Bytes: data})
	document.Caption = caption
	return h.dispatch(chatID, document)
}

// sendPlain sends a message without any parse mode. It is used for strings that
// must not be interpreted as Markdown markup.
func (h *Handler) sendPlain(chatID int64, text string) error {
	return h.dispatch(chatID, tgbotapi.NewMessage(chatID, text))
}

// sendError reports a validation problem to the user.
func (h *Handler) sendError(chatID int64, text string) error {
	return h.sendPlain(chatID, "❌ "+text)
}

func (h *Handler) dispatch(chatID int64, message tgbotapi.Chattable) error {
	sent, err := h.sender.Send(message)
	if err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	h.remember(chatID, sent.MessageID)
	return nil
}

// answerCallback stops the loading indicator on the user's side.
func (h *Handler) answerCallback(callbackID string) error {
	if _, err := h.sender.Request(tgbotapi.NewCallback(callbackID, "")); err != nil {
		return fmt.Errorf("answer callback query: %w", err)
	}
	return nil
}

// activeState returns the current dialog state, telling the user when there is
// no active dialog. A nil state with a nil error means "already reported".
func (h *Handler) activeState(ctx context.Context, chatID, userID int64) (*domain.UserState, error) {
	state, err := h.state.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, h.sendError(chatID, "Сессия устарела. Начните расчёт заново.")
	}
	return state, nil
}

func (h *Handler) remember(chatID int64, messageID int) {
	if messageID == 0 {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	history := h.history[chatID]
	history = append(history, messageID)
	if len(history) > maxHistoryPerChat {
		history = history[len(history)-maxHistoryPerChat:]
	}
	h.history[chatID] = history
}

// cleanupMessages deletes the messages the bot sent during the previous step.
// Deletion failures are expected (a message may be too old to be deleted), so
// they are only logged by the caller when they matter.
func (h *Handler) cleanupMessages(chatID int64) {
	h.mu.Lock()
	messageIDs := h.history[chatID]
	delete(h.history, chatID)
	h.mu.Unlock()

	for _, messageID := range messageIDs {
		if _, err := h.sender.Request(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
			h.log.Debug().Err(err).Int64("chat_id", chatID).Int("message_id", messageID).
				Msg(logger.Source(logComponent, "cleanupMessages") + "не удалось удалить сообщение бота")
		}
	}
}

// handleState routes a text message to the active dialog.
func (h *Handler) handleState(ctx context.Context, message *tgbotapi.Message, state *domain.UserState) error {
	chatID := message.Chat.ID
	userID := message.From.ID

	switch state.Command {
	case commandWater:
		return h.handleWaterState(ctx, chatID, userID, message.Text, state)
	case commandOneRepMax:
		return h.handleOneRepMaxState(ctx, chatID, userID, message.Text, state)
	case commandBodyFat:
		return h.handleBodyFatState(ctx, chatID, userID, message.Text, state)
	case commandTDEE:
		return h.handleTDEEState(ctx, chatID, userID, message.Text, state)
	case commandMeasure:
		return h.handleMeasureState(ctx, chatID, userID, message.Text, state)
	default:
		// Unknown command in state means stale data: reset and show the menu.
		if err := h.state.Finish(ctx, message.From.ID); err != nil {
			return fmt.Errorf("reset stale dialog state: %w", err)
		}
		return h.sendMainMenu(message.Chat.ID)
	}
}

// parseDecimal accepts both "70.5" and "70,5" and checks the bounds. Bots are
// frequently used on mobile keyboards with a comma decimal separator.
func parseDecimal(value string, minimum, maximum float64) (float64, bool) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parsed, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, false
	}
	return parsed, true
}

// parseIntInRange parses an integer and checks the bounds.
func parseIntInRange(value string, minimum, maximum int) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, false
	}
	return parsed, true
}

// formatDecimal stores a float in the dialog state in a canonical form.
func formatDecimal(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// stateFloat reads a float previously stored with formatDecimal.
func stateFloat(state *domain.UserState, key string) (float64, bool) {
	raw, ok := state.Data[key]
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	return parsed, err == nil
}

// stateInt reads an integer previously stored with formatDecimal/strconv.Itoa.
func stateInt(state *domain.UserState, key string) (int, bool) {
	raw, ok := state.Data[key]
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(raw)
	return parsed, err == nil
}

func (h *Handler) handleStart(chatID int64) error {
	return h.send(chatID,
		"🏋️ *Mega Trainer*\n\n"+
			"Помогу рассчитать показатели тренировок и питания.\n\n"+
			"*Расчёты:*\n"+
			"/water — норма воды\n"+
			"/onerepmax — одноповторный максимум\n"+
			"/bodyfat — процент жира\n"+
			"/tdee — суточный расход калорий\n\n"+
			"*Дневник измерений:*\n"+
			"/measure — записать вес\n"+
			"/chart — график динамики веса\n"+
			"/history — последние измерения\n"+
			"/export — выгрузить данные в CSV\n\n"+
			"В любой момент можно отправить /cancel, чтобы прервать расчёт.")
}

func (h *Handler) sendMainMenu(chatID int64) error {
	return h.send(chatID,
		"Выберите действие:\n\n"+
			"💧 /water — норма воды\n"+
			"🏋️ /onerepmax — одноповторный максимум\n"+
			"📊 /bodyfat — процент жира\n"+
			"🔥 /tdee — суточный расход калорий\n\n"+
			"⚖️ /measure — записать вес\n"+
			"📈 /chart — график веса\n"+
			"📋 /history — история измерений\n"+
			"📄 /export — CSV")
}
