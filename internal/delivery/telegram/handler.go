// Package telegram implements the Telegram transport: it parses updates, drives
// the step-by-step dialogs and renders the screens. All network access goes
// through the Sender interface, which keeps the dialogs testable.
package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"

	"mega-trainer-go/internal/service"
)

// Dialog commands. They are used both as Telegram commands and as dialog keys
// in the FSM state, so a user can always switch the flow by sending a command.
const (
	commandStart     = "start"
	commandWater     = "water"
	commandOneRepMax = "onerepmax"
	commandBodyFat   = "bodyfat"
	commandTDEE      = "tdee"
	commandCancel    = "cancel"
	commandMeasure   = "measure"
	commandChart     = "chart"
	commandHistory   = "history"
	commandExport    = "export"
)

// Callback actions.
const (
	callbackGender        = "gender"
	callbackActivity      = "activity"
	callbackGoal          = "goal"
	callbackWaterActivity = "water_activity"
	callbackOneRepFormula = "onerepmax_formula"
)

// State data keys.
const (
	keyWeight   = "weight"
	keyActivity = "activity"
	keyGender   = "gender"
	keyAge      = "age"
	keyHeight   = "height"
	keyNeck     = "neck"
	keyWaist    = "waist"
	keyHip      = "hip"
	keyReps     = "reps"
	keyFormula  = "formula"
	keyGoal     = "goal"
)

// maxHistoryPerChat caps the in-memory list of bot messages tracked for cleanup.
const maxHistoryPerChat = 20

// Sender is the subset of the Telegram Bot API used by the handler.
type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
}

// Handler processes Telegram updates and drives the dialogs.
type Handler struct {
	sender       Sender
	state        *service.StateService
	users        *service.UserService
	measurements *service.MeasurementService
	log          zerolog.Logger
	locks        *keyedLocker

	mu      sync.Mutex
	history map[int64][]int
}

// NewHandler creates a Handler. The users and measurements services may be nil
// when persistence is disabled; the corresponding commands then report that the
// feature is unavailable instead of panicking.
func NewHandler(
	sender Sender,
	state *service.StateService,
	users *service.UserService,
	measurements *service.MeasurementService,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		sender:       sender,
		state:        state,
		users:        users,
		measurements: measurements,
		log:          log,
		locks:        newKeyedLocker(),
		history:      make(map[int64][]int),
	}
}

// HandleUpdate processes a single Telegram update. Updates of the same user are
// serialized so that concurrent messages cannot interleave dialog state writes.
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	from, chatID, ok := updateActor(update)
	if !ok {
		return nil
	}

	unlock := h.locks.Lock(from.ID)
	defer unlock()

	h.ensureUser(ctx, from)

	if update.CallbackQuery != nil {
		return h.handleCallback(ctx, update.CallbackQuery, chatID)
	}
	return h.handleMessage(ctx, update.Message)
}

func (h *Handler) ensureUser(ctx context.Context, from *tgbotapi.User) {
	if h.users == nil {
		return
	}

	if _, err := h.users.Ensure(ctx, from.ID, from.UserName, from.FirstName, from.LastName, from.LanguageCode); err != nil {
		// Persistence problems must not break the calculator itself.
		h.log.Warn().Err(err).Int64("telegram_id", from.ID).Msg("failed to persist user")
	}
}

func (h *Handler) handleMessage(ctx context.Context, message *tgbotapi.Message) error {
	chatID := message.Chat.ID
	userID := message.From.ID

	h.cleanupMessages(chatID)

	if command := commandOf(message); command != "" {
		return h.handleCommand(ctx, message, command)
	}

	state, err := h.state.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get dialog state: %w", err)
	}
	if state == nil {
		return h.sendMainMenu(chatID)
	}

	return h.handleState(ctx, message, state)
}

func (h *Handler) handleCommand(ctx context.Context, message *tgbotapi.Message, command string) error {
	chatID := message.Chat.ID
	userID := message.From.ID

	// Any command abandons the current dialog, so users never get stuck.
	if state, err := h.state.Get(ctx, userID); err != nil {
		h.log.Warn().Err(err).Msg("failed to read dialog state before command")
	} else if state != nil {
		if err := h.state.Finish(ctx, userID); err != nil {
			return fmt.Errorf("reset dialog state: %w", err)
		}
	}

	switch command {
	case commandStart:
		return h.handleStart(chatID)
	case commandWater:
		return h.startWater(ctx, chatID, userID)
	case commandOneRepMax:
		return h.startOneRepMax(ctx, chatID, userID)
	case commandBodyFat:
		return h.startBodyFat(ctx, chatID, userID)
	case commandTDEE:
		return h.startTDEE(ctx, chatID, userID)
	case commandMeasure:
		return h.startMeasure(ctx, chatID, userID)
	case commandChart:
		return h.showWeightChart(ctx, chatID, userID)
	case commandHistory:
		return h.showHistory(ctx, chatID, userID)
	case commandExport:
		return h.exportMeasurements(ctx, chatID, userID)
	case commandCancel:
		return h.send(chatID, "Диалог отменён. Выберите расчёт командой из меню.")
	default:
		return h.sendMainMenu(chatID)
	}
}

func (h *Handler) handleCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, chatID int64) error {
	if err := h.answerCallback(callback.ID); err != nil {
		h.log.Debug().Err(err).Msg("failed to answer callback query")
	}

	h.cleanupMessages(chatID)

	action, value, found := strings.Cut(callback.Data, ":")
	if !found {
		return h.sendError(chatID, "Некорректный запрос. Начните заново.")
	}

	userID := callback.From.ID
	switch action {
	case callbackWaterActivity:
		return h.handleWaterActivity(ctx, chatID, userID, value)
	case callbackOneRepFormula:
		return h.handleOneRepMaxFormula(ctx, chatID, userID, value)
	case callbackGender:
		return h.handleGender(ctx, chatID, userID, value)
	case callbackActivity:
		return h.handleActivity(ctx, chatID, userID, value)
	case callbackGoal:
		return h.handleGoal(ctx, chatID, userID, value)
	default:
		return h.sendError(chatID, "Неизвестное действие. Начните заново.")
	}
}

// commandOf extracts a bot command from the message text. Unlike
// tgbotapi.Message.Command it does not depend on message entities, so a
// malformed or synthetic update is still routed correctly.
func commandOf(message *tgbotapi.Message) string {
	text := strings.TrimSpace(message.Text)
	if !strings.HasPrefix(text, "/") {
		return ""
	}

	text = strings.TrimPrefix(text, "/")
	if i := strings.IndexAny(text, " @"); i >= 0 {
		text = text[:i]
	}

	return strings.ToLower(text)
}

// updateActor returns the user and chat a supported update belongs to.
func updateActor(update tgbotapi.Update) (*tgbotapi.User, int64, bool) {
	switch {
	case update.Message != nil:
		msg := update.Message
		if msg.From == nil || !msg.Chat.IsPrivate() {
			return nil, 0, false
		}
		return msg.From, msg.Chat.ID, true
	case update.CallbackQuery != nil:
		cb := update.CallbackQuery
		if cb.From == nil || cb.Message == nil || !cb.Message.Chat.IsPrivate() {
			return nil, 0, false
		}
		return cb.From, cb.Message.Chat.ID, true
	default:
		return nil, 0, false
	}
}
