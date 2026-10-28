//go:build e2e

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"

	"mega-trainer-go/internal/app"
	"mega-trainer-go/internal/config"
	"mega-trainer-go/internal/storage/postgres"
	"mega-trainer-go/migrations"
)

// ---------- fake Telegram Bot API ----------

type tgRequest struct {
	Method   string
	ChatID   int64
	Text     string
	Caption  string
	File     []byte
	Consumed bool
}

type fakeTelegram struct {
	server    *httptest.Server
	updates   chan []byte
	ready     chan struct{}
	readyOnce sync.Once

	mu        sync.Mutex
	requests  []tgRequest
	messageID int
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	t.Helper()

	f := &fakeTelegram{
		updates: make(chan []byte, 32),
		ready:   make(chan struct{}),
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)

	return f
}

func (f *fakeTelegram) handle(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

	switch method {
	case "getMe":
		writeResult(w, map[string]any{
			"id": 1, "is_bot": true, "first_name": "Test", "username": "test_bot",
		})
	case "deleteWebhook":
		f.readyOnce.Do(func() { close(f.ready) })
		writeResult(w, true)
	case "getUpdates":
		select {
		case update := <-f.updates:
			writeRawResult(w, "["+string(update)+"]")
		case <-time.After(20 * time.Millisecond):
			writeRawResult(w, "[]")
		}
	default:
		chatID := f.record(r, method)
		writeResult(w, map[string]any{
			"message_id": f.nextMessageID(),
			"date":       time.Now().Unix(),
			"chat":       map[string]any{"id": chatID, "type": "private"},
		})
	}
}

// record stores a Bot API call; the returned chat id is echoed back to the bot.
func (f *fakeTelegram) record(r *http.Request, method string) int64 {
	req := tgRequest{Method: method}

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(16 << 20); err == nil {
			req.ChatID = parseChatID(r.FormValue("chat_id"))
			req.Text = r.FormValue("text")
			req.Caption = r.FormValue("caption")
			for _, field := range []string{"photo", "document"} {
				file, _, err := r.FormFile(field)
				if err != nil {
					continue
				}
				req.File, _ = io.ReadAll(file)
				_ = file.Close()
			}
		}
	} else {
		_ = r.ParseForm()
		req.ChatID = parseChatID(r.PostFormValue("chat_id"))
		req.Text = r.PostFormValue("text")
		req.Caption = r.PostFormValue("caption")
	}

	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()

	return req.ChatID
}

func (f *fakeTelegram) nextMessageID() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messageID++
	return f.messageID
}

func (f *fakeTelegram) push(t *testing.T, update []byte) {
	t.Helper()
	select {
	case f.updates <- update:
	case <-time.After(5 * time.Second):
		t.Fatal("fake telegram did not accept the update")
	}
}

func (f *fakeTelegram) waitFor(t *testing.T, method string, match func(tgRequest) bool) tgRequest {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for {
		f.mu.Lock()
		for i := range f.requests {
			req := f.requests[i]
			if req.Consumed || req.Method != method {
				continue
			}
			if match != nil && !match(req) {
				continue
			}
			f.requests[i].Consumed = true
			f.mu.Unlock()
			return req
		}
		dump := fmt.Sprintf("%+v", f.requests)
		f.mu.Unlock()

		if time.Now().After(deadline) {
			t.Fatalf("no %q request within timeout; see %s", method, dump)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (f *fakeTelegram) waitMessage(t *testing.T, fragment string) tgRequest {
	t.Helper()
	return f.waitFor(t, "sendMessage", func(req tgRequest) bool {
		return strings.Contains(req.Text, fragment)
	})
}

// ---------- test environment ----------

func testConfig(t *testing.T, apiURL string) *config.Config {
	t.Helper()

	host := os.Getenv("TEST_POSTGRES_HOST")
	user := os.Getenv("TEST_POSTGRES_USER")
	dbName := os.Getenv("TEST_POSTGRES_DB")
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if host == "" || user == "" || dbName == "" || redisAddr == "" {
		t.Skip("set TEST_POSTGRES_HOST/USER/PASSWORD/DB and TEST_REDIS_ADDR to run e2e tests")
	}

	port, err := strconv.Atoi(envOr("TEST_POSTGRES_PORT", "5432"))
	if err != nil {
		t.Fatalf("invalid TEST_POSTGRES_PORT: %v", err)
	}

	cfg := &config.Config{
		App: config.AppConfig{
			Name:        "mega_trainer_e2e",
			Environment: "test",
			Workers:     2,
		},
		Telegram: config.TelegramConfig{
			BotToken: "1:test-token",
			APIURL:   apiURL,
		},
		Database: config.DatabaseConfig{
			Postgres: config.PostgresConfig{
				Host:            host,
				Port:            port,
				User:            user,
				Password:        os.Getenv("TEST_POSTGRES_PASSWORD"),
				DBName:          dbName,
				SSLMode:         "disable",
				MaxConnections:  5,
				MaxIdle:         2,
				ConnMaxLifetime: 5,
				SearchPath:      createSchema(t, host, port, user, os.Getenv("TEST_POSTGRES_PASSWORD"), dbName),
			},
		},
		Redis: config.RedisConfig{
			Address:  redisAddr,
			Password: os.Getenv("TEST_REDIS_PASSWORD"),
			PoolSize: 5,
		},
		Monitoring: config.MonitoringConfig{Enabled: false},
		Logging:    config.LoggingConfig{Level: "error", Format: "console", Output: "stdout"},
	}

	return cfg
}

// createSchema isolates the e2e run from the developer database.
func createSchema(t *testing.T, host string, port int, user, password, dbName string) string {
	t.Helper()

	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbName)
	admin, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	schema := fmt.Sprintf("e2e_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	})

	return schema
}

func openTestDB(t *testing.T, cfg *config.Config) *sqlx.DB {
	t.Helper()

	db, err := postgres.Open(context.Background(), cfg.Database.Postgres)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db
}

func applyMigrations(t *testing.T, cfg *config.Config) {
	t.Helper()

	db := openTestDB(t, cfg)
	if _, err := postgres.NewMigrator(db, migrations.FS).Up(context.Background()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseChatID(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

// ---------- update builders ----------

func textUpdate(updateID int, userID int64, text string) []byte {
	payload := map[string]any{
		"update_id": updateID,
		"message": map[string]any{
			"message_id": updateID,
			"date":       time.Now().Unix(),
			"from":       tgUser(userID),
			"chat":       map[string]any{"id": userID, "type": "private", "first_name": "Tester"},
			"text":       text,
		},
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func callbackUpdate(updateID int, userID int64, data string) []byte {
	payload := map[string]any{
		"update_id": updateID,
		"callback_query": map[string]any{
			"id":   fmt.Sprintf("callback-%d", updateID),
			"from": tgUser(userID),
			"data": data,
			"message": map[string]any{
				"message_id": updateID,
				"date":       time.Now().Unix(),
				"chat":       map[string]any{"id": userID, "type": "private"},
			},
		},
	}
	raw, _ := json.Marshal(payload)
	return raw
}

func tgUser(userID int64) map[string]any {
	return map[string]any{
		"id":            userID,
		"is_bot":        false,
		"first_name":    "Tester",
		"username":      "tester",
		"language_code": "ru",
	}
}

// ---------- helpers for HTTP responses ----------

func writeResult(w http.ResponseWriter, result any) {
	writeRawResult(w, mustJSON(result))
}

func writeRawResult(w http.ResponseWriter, raw string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"ok":true,"result":`+raw+`}`)
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// ---------- the end-to-end scenario ----------

func TestBotEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e test is skipped in short mode")
	}

	telegram := newFakeTelegram(t)
	cfg := testConfig(t, telegram.server.URL)
	applyMigrations(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx, cfg, zerolog.Nop()) }()

	select {
	case <-telegram.ready:
	case <-time.After(20 * time.Second):
		t.Fatal("bot did not authorize against the fake Telegram API")
	}

	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("app.Run returned an error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("app did not stop after the context was cancelled")
		}
	})

	t.Run("start shows the menu", func(t *testing.T) {
		telegram.push(t, textUpdate(1, 990001, "/start"))
		telegram.waitMessage(t, "Mega Trainer")
	})

	t.Run("water dialog calculates the result", func(t *testing.T) {
		const userID int64 = 990002

		telegram.push(t, textUpdate(10, userID, "/water"))
		telegram.waitMessage(t, "Введите ваш вес")

		telegram.push(t, textUpdate(11, userID, "70.5"))
		telegram.waitMessage(t, "Выберите уровень активности")

		telegram.push(t, callbackUpdate(12, userID, "water_activity:medium"))
		// 70.5 * 0.03 * 1.2 = 2.538 -> 2.54 in the reply.
		telegram.waitMessage(t, "2.54")
	})

	t.Run("weight journal is stored and plotted", func(t *testing.T) {
		const userID int64 = 990003

		telegram.push(t, textUpdate(20, userID, "/measure"))
		telegram.waitMessage(t, "Введите ваш вес")
		telegram.push(t, textUpdate(21, userID, "70"))
		telegram.waitMessage(t, "Записано")

		telegram.push(t, textUpdate(22, userID, "/measure"))
		telegram.waitMessage(t, "Введите ваш вес")
		telegram.push(t, textUpdate(23, userID, "70.5"))
		telegram.waitMessage(t, "Записано")

		telegram.push(t, textUpdate(24, userID, "/chart"))
		photo := telegram.waitFor(t, "sendPhoto", func(req tgRequest) bool {
			return strings.Contains(req.Caption, "Динамика веса")
		})
		if !bytes.HasPrefix(photo.File, []byte{0x89, 'P', 'N', 'G'}) {
			t.Fatalf("chart attachment is not a PNG (%d bytes)", len(photo.File))
		}

		db := openTestDB(t, cfg)
		var count int
		if err := db.Get(&count, `SELECT COUNT(*) FROM measurements`); err != nil {
			t.Fatalf("count measurements: %v", err)
		}
		if count != 2 {
			t.Fatalf("stored %d measurements, want 2", count)
		}
	})

	t.Run("history and csv export", func(t *testing.T) {
		const userID int64 = 990003 // measurements from the previous scenario

		telegram.push(t, textUpdate(30, userID, "/history"))
		telegram.waitMessage(t, "Последние измерения")

		telegram.push(t, textUpdate(31, userID, "/export"))
		document := telegram.waitFor(t, "sendDocument", nil)
		if !bytes.Contains(document.File, []byte("measured_at,weight_kg")) {
			t.Fatalf("csv export does not contain the expected header: %q", string(document.File))
		}
	})

	t.Run("stale callback is reported without panic", func(t *testing.T) {
		telegram.push(t, callbackUpdate(40, 990005, "gender:male"))
		telegram.waitMessage(t, "Сессия устарела")
	})

	t.Run("invalid input keeps the dialog on the same step", func(t *testing.T) {
		const userID int64 = 990006

		telegram.push(t, textUpdate(50, userID, "/water"))
		telegram.waitMessage(t, "Введите ваш вес")

		telegram.push(t, textUpdate(51, userID, "not a number"))
		telegram.waitMessage(t, "Введите вес числом")
	})
}
