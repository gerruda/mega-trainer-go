package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes a minimal config file that references environment
// variables, mimicking configs/config.yaml.
func writeConfig(t *testing.T) string {
	t.Helper()

	content := `
app:
  workers: 4
telegram:
  bot_token: ${TEST_BOT_TOKEN}
database:
  postgres:
    host: ${TEST_POSTGRES_HOST}
    user: ${TEST_POSTGRES_USER}
    dbname: ${TEST_POSTGRES_DB}
    max_connections: 7
redis:
  address: ${TEST_REDIS_ADDR}
monitoring:
  enabled: true
  port: 9191
logging:
  level: debug
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}

func setValidEnv(t *testing.T) {
	t.Helper()

	t.Setenv("TEST_BOT_TOKEN", "123456789:AAHtest-token")
	t.Setenv("TEST_POSTGRES_HOST", "localhost")
	t.Setenv("TEST_POSTGRES_USER", "bot")
	t.Setenv("TEST_POSTGRES_DB", "botdb")
	t.Setenv("TEST_REDIS_ADDR", "localhost:6379")
}

func TestLoadAppliesEnvironmentAndDefaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := Load(writeConfig(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.Telegram.BotToken != "123456789:AAHtest-token" {
		t.Fatalf("bot token = %q, want the expanded environment value", cfg.Telegram.BotToken)
	}
	if cfg.App.Workers != 4 {
		t.Fatalf("workers = %d, want 4", cfg.App.Workers)
	}
	if cfg.Database.Postgres.Port != 5432 {
		t.Fatalf("postgres port = %d, want default 5432", cfg.Database.Postgres.Port)
	}
	if cfg.Database.Postgres.SSLMode != "disable" {
		t.Fatalf("sslmode = %q, want default disable", cfg.Database.Postgres.SSLMode)
	}
	if cfg.Database.Postgres.MaxIdle != 5 {
		t.Fatalf("max idle = %d, want default 5", cfg.Database.Postgres.MaxIdle)
	}
	if cfg.Database.Postgres.ConnMaxLifetime != 30 {
		t.Fatalf("conn max lifetime = %d, want default 30", cfg.Database.Postgres.ConnMaxLifetime)
	}
	if cfg.Database.Postgres.MaxConnections != 7 {
		t.Fatalf("max connections = %d, want 7", cfg.Database.Postgres.MaxConnections)
	}
	if cfg.Logging.Format != "console" {
		t.Fatalf("log format = %q, want default console", cfg.Logging.Format)
	}
}

func TestLoadFailsWithoutRequiredValues(t *testing.T) {
	t.Setenv("TEST_BOT_TOKEN", "")
	t.Setenv("TEST_POSTGRES_HOST", "")
	t.Setenv("TEST_POSTGRES_USER", "")
	t.Setenv("TEST_POSTGRES_DB", "")
	t.Setenv("TEST_REDIS_ADDR", "")

	_, err := Load(writeConfig(t))
	if err == nil {
		t.Fatal("expected an error for an incomplete configuration")
	}

	for _, fragment := range []string{"bot_token", "host", "redis.address"} {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("error %q does not mention %q", err, fragment)
		}
	}
}

func TestLoadForMigrationOnlyRequiresDatabase(t *testing.T) {
	setValidEnv(t)
	t.Setenv("TEST_BOT_TOKEN", "")
	t.Setenv("TEST_REDIS_ADDR", "")

	cfg, err := LoadForMigration(writeConfig(t))
	if err != nil {
		t.Fatalf("load for migration: %v", err)
	}
	if cfg.Database.Postgres.Host != "localhost" {
		t.Fatalf("host = %q, want localhost", cfg.Database.Postgres.Host)
	}
}

func TestLoadReportsMissingFile(t *testing.T) {
	setValidEnv(t)

	_, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err == nil || !strings.Contains(err.Error(), "CONFIG_PATH") {
		t.Fatalf("error = %v, want a hint about CONFIG_PATH", err)
	}
}

func TestPostgresConfigDSN(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "db",
		Port:     5432,
		User:     "bot",
		Password: "secret",
		DBName:   "botdb",
		SSLMode:  "disable",
	}

	want := "host=db port=5432 user=bot password=secret dbname=botdb sslmode=disable"
	if got := cfg.DSN(); got != want {
		t.Fatalf("dsn = %q, want %q", got, want)
	}
}

func TestValidateTelegramProxy(t *testing.T) {
	t.Parallel()

	base := func(proxyURL string) *Config {
		return &Config{
			Telegram: TelegramConfig{BotToken: "123456789:AAHtest-token", ProxyURL: proxyURL},
			Database: DatabaseConfig{Postgres: PostgresConfig{
				Host: "localhost", User: "bot", DBName: "botdb",
			}},
			Redis: RedisConfig{Address: "localhost:6379"},
		}
	}

	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{name: "no proxy", proxyURL: ""},
		{name: "http proxy", proxyURL: "http://127.0.0.1:12334"},
		{name: "https proxy with credentials", proxyURL: "https://user:pass@proxy.example:8080"},
		{name: "socks5 is rejected", proxyURL: "socks5://127.0.0.1:1080", wantErr: true},
		{name: "missing host is rejected", proxyURL: "http://", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := base(tt.proxyURL).Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected a validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
