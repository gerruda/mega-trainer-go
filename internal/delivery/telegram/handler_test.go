package telegram

import (
	"bytes"
	"context"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/internal/service"
)

// ---- fakes -----------------------------------------------------------------

type sentMessage struct {
	chatID    int64
	kind      string // message | photo | document
	text      string
	caption   string
	fileBytes []byte
}

type fakeSender struct {
	mu     sync.Mutex
	sent   []sentMessage
	nextID int
	err    error
}

func (f *fakeSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return tgbotapi.Message{}, f.err
	}

	f.nextID++
	message := tgbotapi.Message{MessageID: f.nextID}

	switch config := c.(type) {
	case tgbotapi.MessageConfig:
		f.sent = append(f.sent, sentMessage{chatID: config.ChatID, kind: "message", text: config.Text})
		message.Chat = &tgbotapi.Chat{ID: config.ChatID}
	case tgbotapi.PhotoConfig:
		f.sent = append(f.sent, sentMessage{
			chatID:    config.ChatID,
			kind:      "photo",
			caption:   config.Caption,
			fileBytes: fileBytesOf(config.File),
		})
		message.Chat = &tgbotapi.Chat{ID: config.ChatID}
	case tgbotapi.DocumentConfig:
		f.sent = append(f.sent, sentMessage{
			chatID:    config.ChatID,
			kind:      "document",
			caption:   config.Caption,
			fileBytes: fileBytesOf(config.File),
		})
		message.Chat = &tgbotapi.Chat{ID: config.ChatID}
	}

	return message, nil
}

func fileBytesOf(file tgbotapi.RequestFileData) []byte {
	if bytes, ok := file.(tgbotapi.FileBytes); ok {
		return bytes.Bytes
	}
	return nil
}

func (f *fakeSender) Request(tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return &tgbotapi.APIResponse{Ok: true}, nil
}

func (f *fakeSender) lastText(t *testing.T) string {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.sent) == 0 {
		t.Fatal("no messages were sent")
	}
	return f.sent[len(f.sent)-1].text
}

func (f *fakeSender) lastSent(t *testing.T) sentMessage {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.sent) == 0 {
		t.Fatal("no messages were sent")
	}
	return f.sent[len(f.sent)-1]
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

type fakeStateRepo struct {
	mu     sync.Mutex
	states map[int64]domain.UserState
}

func newFakeStateRepo() *fakeStateRepo {
	return &fakeStateRepo{states: make(map[int64]domain.UserState)}
}

func (r *fakeStateRepo) GetState(_ context.Context, userID int64) (*domain.UserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[userID]
	if !ok {
		return nil, nil
	}
	return copyState(&state), nil
}

func (r *fakeStateRepo) SetState(_ context.Context, state *domain.UserState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.states[state.UserID] = *copyState(state)
	return nil
}

func (r *fakeStateRepo) ClearState(_ context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.states, userID)
	return nil
}

func (r *fakeStateRepo) get(userID int64) *domain.UserState {
	r.mu.Lock()
	defer r.mu.Unlock()

	state, ok := r.states[userID]
	if !ok {
		return nil
	}
	return copyState(&state)
}

func copyState(state *domain.UserState) *domain.UserState {
	clone := *state
	clone.Data = make(map[string]string, len(state.Data))
	for key, value := range state.Data {
		clone.Data[key] = value
	}
	return &clone
}

type fakeUserRepo struct {
	mu    sync.Mutex
	users map[int64]*domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{users: make(map[int64]*domain.User)}
}

func (r *fakeUserRepo) Upsert(_ context.Context, user *domain.User) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored := *user
	if existing, ok := r.users[user.TelegramID]; ok {
		stored.ID = existing.ID
	} else {
		stored.ID = user.TelegramID
	}
	r.users[user.TelegramID] = &stored

	copied := stored
	return &copied, nil
}

func (r *fakeUserRepo) ListActive(_ context.Context, limit int) ([]*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	users := make([]*domain.User, 0, len(r.users))
	for _, user := range r.users {
		if limit > 0 && len(users) == limit {
			break
		}
		copied := *user
		users = append(users, &copied)
	}
	return users, nil
}

func (r *fakeUserRepo) GetByTelegramID(_ context.Context, telegramID int64) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[telegramID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	copied := *user
	return &copied, nil
}

type fakeMeasurementRepo struct {
	mu    sync.Mutex
	items []domain.Measurement
	next  int64
}

func newFakeMeasurementRepo() *fakeMeasurementRepo {
	return &fakeMeasurementRepo{}
}

func (r *fakeMeasurementRepo) Create(_ context.Context, measurement *domain.Measurement) (*domain.Measurement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.next++
	stored := *measurement
	stored.ID = r.next
	if stored.MeasuredAt.IsZero() {
		stored.MeasuredAt = time.Now().UTC()
	}
	stored.CreatedAt = stored.MeasuredAt
	r.items = append(r.items, stored)

	return &stored, nil
}

func (r *fakeMeasurementRepo) ListByUserID(_ context.Context, userID int64, limit int) ([]*domain.Measurement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	matched := make([]*domain.Measurement, 0, len(r.items))
	for i := range r.items {
		if r.items[i].UserID == userID {
			matched = append(matched, &r.items[i])
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		return matched[i].MeasuredAt.After(matched[j].MeasuredAt)
	})
	if limit > 0 && len(matched) > limit {
		matched = matched[:limit]
	}

	return matched, nil
}

func (r *fakeMeasurementRepo) seed(userID int64, weight float64, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.next++
	r.items = append(r.items, domain.Measurement{
		ID:         r.next,
		UserID:     userID,
		WeightKG:   weight,
		MeasuredAt: at,
		CreatedAt:  at,
	})
}

func (r *fakeMeasurementRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.items)
}

func (r *fakeMeasurementRepo) last(t *testing.T) domain.Measurement {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.items) == 0 {
		t.Fatal("no measurements stored")
	}
	return r.items[len(r.items)-1]
}

// ---- helpers ---------------------------------------------------------------

func newTestHandler(t *testing.T) (*Handler, *fakeSender, *fakeStateRepo) {
	t.Helper()

	handler, sender, states, _ := newFullTestHandler(t)
	return handler, sender, states
}

func newMeasurementTestHandler(t *testing.T) (*Handler, *fakeSender, *fakeMeasurementRepo) {
	t.Helper()

	handler, sender, _, measurements := newFullTestHandler(t)
	return handler, sender, measurements
}

func newFullTestHandler(t *testing.T) (*Handler, *fakeSender, *fakeStateRepo, *fakeMeasurementRepo) {
	t.Helper()

	sender := &fakeSender{}
	states := newFakeStateRepo()
	users := newFakeUserRepo()
	measurements := newFakeMeasurementRepo()

	handler := NewHandler(
		sender,
		service.NewStateService(states),
		service.NewUserService(users),
		service.NewMeasurementService(measurements, users),
		zerolog.Nop(),
	)
	return handler, sender, states, measurements
}

// testChatID is the private chat used by the message helpers.
const testChatID int64 = 100

func messageUpdate(userID int64, text string) tgbotapi.Update {
	return tgbotapi.Update{
		UpdateID: 1,
		Message: &tgbotapi.Message{
			MessageID: 1,
			From:      &tgbotapi.User{ID: userID, UserName: "tester", FirstName: "Test"},
			Chat:      &tgbotapi.Chat{ID: testChatID, Type: "private"},
			Text:      text,
		},
	}
}

func callbackUpdate(userID int64, data string) tgbotapi.Update {
	return tgbotapi.Update{
		UpdateID: 2,
		CallbackQuery: &tgbotapi.CallbackQuery{
			ID:   "callback-1",
			From: &tgbotapi.User{ID: userID, UserName: "tester"},
			Data: data,
			Message: &tgbotapi.Message{
				MessageID: 10,
				Chat:      &tgbotapi.Chat{ID: testChatID, Type: "private"},
			},
		},
	}
}

func send(t *testing.T, handler *Handler, update tgbotapi.Update) {
	t.Helper()

	if err := handler.HandleUpdate(context.Background(), update); err != nil {
		t.Fatalf("handle update: %v", err)
	}
}

func requireContains(t *testing.T, text, fragment string) {
	t.Helper()

	if !strings.Contains(text, fragment) {
		t.Fatalf("message %q does not contain %q", text, fragment)
	}
}

// ---- tests -----------------------------------------------------------------

func TestWaterFlow(t *testing.T) {
	handler, sender, states := newTestHandler(t)
	const userID int64 = 1001

	send(t, handler, messageUpdate(userID, "/water"))
	requireContains(t, sender.lastText(t), "Норма воды")

	state := states.get(userID)
	if state == nil || state.Command != commandWater || state.Step != 1 {
		t.Fatalf("state = %+v, want command=water step=1", state)
	}

	// A comma decimal separator is accepted on purpose: mobile keyboards use it.
	send(t, handler, messageUpdate(userID, "70,5"))

	state = states.get(userID)
	if state == nil || state.Step != 2 {
		t.Fatalf("state = %+v, want step=2", state)
	}
	if state.Data[keyWeight] != "70.5" {
		t.Fatalf("weight = %q, want 70.5", state.Data[keyWeight])
	}
	requireContains(t, sender.lastText(t), "Выберите уровень активности")

	send(t, handler, callbackUpdate(userID, callbackWaterActivity+":"+domain.WaterActivityMedium))

	// 70.5 kg * 0.03 l/kg * 1.2 = 2.538 -> "2.54" in the reply.
	requireContains(t, sender.lastText(t), "2.54")

	if state := states.get(userID); state != nil {
		t.Fatalf("state = %+v, want it to be cleared after the result", state)
	}
}

func TestWaterFlowRejectsInvalidInput(t *testing.T) {
	handler, sender, states := newTestHandler(t)
	const userID int64 = 1002

	send(t, handler, messageUpdate(userID, "/water"))
	send(t, handler, messageUpdate(userID, "not a number"))

	requireContains(t, sender.lastText(t), "Введите вес числом")

	state := states.get(userID)
	if state == nil || state.Step != 1 {
		t.Fatalf("state = %+v, want the dialog to stay on step 1", state)
	}
}

func TestOneRepMaxKeepsChosenFormula(t *testing.T) {
	handler, sender, states := newTestHandler(t)
	const userID int64 = 1003

	send(t, handler, messageUpdate(userID, "/onerepmax"))
	send(t, handler, callbackUpdate(userID, callbackOneRepFormula+":"+domain.FormulaBrzycki))

	state := states.get(userID)
	if state == nil || state.Step != 2 {
		t.Fatalf("state = %+v, want step=2", state)
	}
	if state.Data[keyFormula] != domain.FormulaBrzycki {
		t.Fatalf("formula = %q, want brzycki to survive the step change", state.Data[keyFormula])
	}

	send(t, handler, messageUpdate(userID, "100"))
	send(t, handler, messageUpdate(userID, "5"))

	// Brzycki: 100 * 36 / (37 - 5) = 112.5
	requireContains(t, sender.lastText(t), "112.5")

	if state := states.get(userID); state != nil {
		t.Fatalf("state = %+v, want it to be cleared after the result", state)
	}
}

func TestTDEEFlow(t *testing.T) {
	handler, sender, states := newTestHandler(t)
	const userID int64 = 1004

	send(t, handler, messageUpdate(userID, "/tdee"))
	send(t, handler, callbackUpdate(userID, callbackGender+":"+domain.GenderMale))
	send(t, handler, messageUpdate(userID, "30"))
	send(t, handler, messageUpdate(userID, "80"))
	send(t, handler, messageUpdate(userID, "180"))
	send(t, handler, callbackUpdate(userID, callbackActivity+":"+domain.ActivityModerate))
	send(t, handler, callbackUpdate(userID, callbackGoal+":"+domain.GoalMaintenance))

	// BMR 1780 kcal, TDEE 1780 * 1.55 = 2759 kcal.
	requireContains(t, sender.lastText(t), "1780")
	requireContains(t, sender.lastText(t), "2759")

	if state := states.get(userID); state != nil {
		t.Fatalf("state = %+v, want it to be cleared after the result", state)
	}
}

func TestBodyFatFlowForMale(t *testing.T) {
	handler, sender, states := newTestHandler(t)
	const userID int64 = 1005

	send(t, handler, messageUpdate(userID, "/bodyfat"))
	send(t, handler, callbackUpdate(userID, callbackGender+":"+domain.GenderMale))
	send(t, handler, messageUpdate(userID, "30"))
	send(t, handler, messageUpdate(userID, "80"))
	send(t, handler, messageUpdate(userID, "180"))
	send(t, handler, messageUpdate(userID, "40"))
	send(t, handler, messageUpdate(userID, "90"))

	requireContains(t, sender.lastText(t), "18.4")

	if state := states.get(userID); state != nil {
		t.Fatalf("state = %+v, want it to be cleared after the result", state)
	}
}

func TestStaleCallbackIsReported(t *testing.T) {
	handler, sender, _ := newTestHandler(t)

	send(t, handler, callbackUpdate(1006, callbackWaterActivity+":"+domain.WaterActivityLow))

	requireContains(t, sender.lastText(t), "Сессия устарела")
}

func TestNewCommandResetsActiveDialog(t *testing.T) {
	handler, _, states := newTestHandler(t)
	const userID int64 = 1007

	send(t, handler, messageUpdate(userID, "/water"))
	send(t, handler, messageUpdate(userID, "70"))
	send(t, handler, messageUpdate(userID, "/tdee"))

	state := states.get(userID)
	if state == nil || state.Command != commandTDEE || state.Step != 1 {
		t.Fatalf("state = %+v, want a fresh tdee dialog", state)
	}
}

func TestGroupChatsAreIgnored(t *testing.T) {
	handler, sender, states := newTestHandler(t)

	update := messageUpdate(1008, "/water")
	update.Message.Chat.Type = "group"

	send(t, handler, update)

	if count := sender.count(); count != 0 {
		t.Fatalf("sent %d messages to a group chat, want 0", count)
	}
	if state := states.get(1008); state != nil {
		t.Fatalf("state = %+v, want no dialog in a group chat", state)
	}
}

func TestSendFailureIsReturned(t *testing.T) {
	handler, sender, _ := newTestHandler(t)
	sender.err = context.DeadlineExceeded

	err := handler.HandleUpdate(context.Background(), messageUpdate(1009, "/start"))
	if err == nil {
		t.Fatal("expected the send error to be returned")
	}
}

func TestMeasureFlowRecordsWeight(t *testing.T) {
	handler, sender, measurements := newMeasurementTestHandler(t)
	const userID int64 = 2001

	send(t, handler, messageUpdate(userID, "/measure"))
	requireContains(t, sender.lastText(t), "Введите ваш вес")

	send(t, handler, messageUpdate(userID, "70,5"))
	requireContains(t, sender.lastText(t), "Записано")

	if count := measurements.count(); count != 1 {
		t.Fatalf("stored %d measurements, want 1", count)
	}
	if weight := measurements.last(t).WeightKG; weight != 70.5 {
		t.Fatalf("weight = %v, want 70.5", weight)
	}
}

func TestChartSendsPhoto(t *testing.T) {
	handler, sender, measurements := newMeasurementTestHandler(t)
	const userID int64 = 2002

	now := time.Now().UTC()
	measurements.seed(userID, 72, now.Add(-48*time.Hour))
	measurements.seed(userID, 71, now.Add(-24*time.Hour))
	measurements.seed(userID, 70, now)

	send(t, handler, messageUpdate(userID, "/chart"))

	sent := sender.lastSent(t)
	if sent.kind != "photo" {
		t.Fatalf("sent kind = %q, want photo", sent.kind)
	}
	if !bytes.HasPrefix(sent.fileBytes, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("attached file is not a PNG image")
	}
	requireContains(t, sent.caption, "Динамика веса")
	requireContains(t, sent.caption, "Сейчас: 70.0 кг")
}

func TestChartRequiresTwoMeasurements(t *testing.T) {
	handler, sender, measurements := newMeasurementTestHandler(t)
	const userID int64 = 2003

	measurements.seed(userID, 70, time.Now().UTC())

	send(t, handler, messageUpdate(userID, "/chart"))
	requireContains(t, sender.lastText(t), "минимум два измерения")
}

func TestHistoryShowsMeasurements(t *testing.T) {
	handler, sender, measurements := newMeasurementTestHandler(t)
	const userID int64 = 2004

	now := time.Now().UTC()
	measurements.seed(userID, 71, now.Add(-24*time.Hour))
	measurements.seed(userID, 70, now)

	send(t, handler, messageUpdate(userID, "/history"))

	text := sender.lastText(t)
	requireContains(t, text, "Последние измерения")
	requireContains(t, text, "70.0 кг")
}

func TestExportSendsCSV(t *testing.T) {
	handler, sender, measurements := newMeasurementTestHandler(t)
	const userID int64 = 2005

	now := time.Now().UTC()
	measurements.seed(userID, 72, now.Add(-24*time.Hour))
	measurements.seed(userID, 70, now)

	send(t, handler, messageUpdate(userID, "/export"))

	sent := sender.lastSent(t)
	if sent.kind != "document" {
		t.Fatalf("sent kind = %q, want document", sent.kind)
	}

	content := string(sent.fileBytes)
	requireContains(t, content, "measured_at,weight_kg")
	// A header plus two chronological rows.
	if rows := strings.Count(strings.TrimSpace(content), "\n") + 1; rows != 3 {
		t.Fatalf("csv has %d rows, want 3:\n%s", rows, content)
	}
}

func TestBodyFatFlowSavesMeasurement(t *testing.T) {
	handler, sender, measurements := newMeasurementTestHandler(t)
	const userID int64 = 2006

	send(t, handler, messageUpdate(userID, "/bodyfat"))
	send(t, handler, callbackUpdate(userID, callbackGender+":"+domain.GenderMale))
	send(t, handler, messageUpdate(userID, "30"))
	send(t, handler, messageUpdate(userID, "80"))
	send(t, handler, messageUpdate(userID, "180"))
	send(t, handler, messageUpdate(userID, "40"))
	send(t, handler, messageUpdate(userID, "90"))

	requireContains(t, sender.lastText(t), "Измерение сохранено")

	if count := measurements.count(); count != 1 {
		t.Fatalf("stored %d measurements, want 1", count)
	}

	stored := measurements.last(t)
	if stored.WeightKG != 80 {
		t.Fatalf("weight = %v, want 80", stored.WeightKG)
	}
	if stored.BodyFatPct == nil {
		t.Fatal("body fat percentage was not saved")
	}
	if *stored.BodyFatPct < 18 || *stored.BodyFatPct > 19 {
		t.Fatalf("body fat = %v, want about 18.4", *stored.BodyFatPct)
	}
}
