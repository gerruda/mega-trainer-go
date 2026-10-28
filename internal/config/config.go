package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config is the root of the application configuration. It is loaded from a YAML
// file (with ${ENV_VAR} expansion) and validated before use, so a misconfigured
// process fails fast with an actionable error instead of panicking later.
type Config struct {
	App        AppConfig        `yaml:"app"`
	Telegram   TelegramConfig   `yaml:"telegram"`
	Database   DatabaseConfig   `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	Reminders  RemindersConfig  `yaml:"reminders"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// AppConfig contains process-wide settings.
type AppConfig struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
	Version     string `yaml:"version"`
	// Workers limits how many Telegram updates are processed concurrently.
	Workers int `yaml:"workers"`
}

// TelegramConfig contains Telegram Bot API settings.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	// ProxyURL routes Bot API requests through an HTTP(S) proxy. Required in
	// networks where api.telegram.org is blocked, e.g. in Russia.
	ProxyURL string `yaml:"proxy_url"`
	Debug    bool   `yaml:"debug"`
}

// DatabaseConfig contains storage settings.
type DatabaseConfig struct {
	Postgres PostgresConfig `yaml:"postgres"`
}

// PostgresConfig contains PostgreSQL connection settings.
type PostgresConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Password        string `yaml:"password"`
	DBName          string `yaml:"dbname"`
	SSLMode         string `yaml:"sslmode"`
	MaxConnections  int    `yaml:"max_connections"`
	MaxIdle         int    `yaml:"max_idle"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime_min"` // minutes
}

// DSN builds a lib/pq connection string.
func (p PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, p.Port, p.User, p.Password, p.DBName, p.SSLMode,
	)
}

// RedisConfig contains Redis connection settings.
type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

// RemindersConfig configures the scheduled "log your weight" reminders.
type RemindersConfig struct {
	Enabled       bool `yaml:"enabled"`
	IntervalHours int  `yaml:"interval_hours"`
}

// MonitoringConfig contains the health/metrics HTTP server settings.
type MonitoringConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"` // stdout | file | both
	FilePath string `yaml:"file_path"`
}

// Load reads the YAML config file, expands ${ENV_VAR} references, applies
// defaults and validates the whole configuration.
//
// A missing .env file is not an error: local development may use a real
// environment, and containers never have one.
func Load(path string) (*Config, error) {
	cfg, err := load(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadForMigration loads the configuration needed to run schema migrations. It
// validates the database settings only, so the migration command works without
// a Telegram token.
func LoadForMigration(path string) (*Config, error) {
	cfg, err := load(path)
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateDatabase(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func load(path string) (*Config, error) {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q (set CONFIG_PATH to override): %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(raw))), &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	cfg.applyDefaults()

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.App.Name == "" {
		c.App.Name = "mega-trainer"
	}
	if c.App.Environment == "" {
		c.App.Environment = "development"
	}
	if c.App.Version == "" {
		c.App.Version = "dev"
	}
	if c.App.Workers <= 0 {
		c.App.Workers = 8
	}

	if c.Database.Postgres.Port == 0 {
		c.Database.Postgres.Port = 5432
	}
	if c.Database.Postgres.SSLMode == "" {
		c.Database.Postgres.SSLMode = "disable"
	}
	if c.Database.Postgres.MaxConnections <= 0 {
		c.Database.Postgres.MaxConnections = 10
	}
	if c.Database.Postgres.MaxIdle <= 0 {
		c.Database.Postgres.MaxIdle = 5
	}
	if c.Database.Postgres.ConnMaxLifetime <= 0 {
		c.Database.Postgres.ConnMaxLifetime = 30
	}

	if c.Redis.DB < 0 {
		c.Redis.DB = 0
	}
	if c.Redis.PoolSize <= 0 {
		c.Redis.PoolSize = 10
	}

	if c.Reminders.IntervalHours <= 0 {
		c.Reminders.IntervalHours = 24
	}

	if c.Monitoring.Port == 0 {
		c.Monitoring.Port = 9090
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "console"
	}
	if c.Logging.Output == "" {
		c.Logging.Output = "stdout"
	}
}

// Validate reports every missing or invalid required setting at once.
func (c *Config) Validate() error {
	var problems []string

	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	switch {
	case c.Telegram.BotToken == "":
		add("telegram.bot_token is required (set the BOT_TOKEN environment variable)")
	case !strings.Contains(c.Telegram.BotToken, ":"):
		add("telegram.bot_token does not look like a Telegram token")
	}

	if c.Telegram.ProxyURL != "" {
		parsed, err := url.Parse(c.Telegram.ProxyURL)
		switch {
		case err != nil:
			add("telegram.proxy_url is not a valid URL: %v", err)
		case parsed.Host == "":
			add("telegram.proxy_url must include a host, e.g. http://127.0.0.1:12334")
		case parsed.Scheme != "http" && parsed.Scheme != "https":
			add("telegram.proxy_url scheme must be http or https, got %q", parsed.Scheme)
		}
	}

	if c.Redis.Address == "" {
		add("redis.address is required (set REDIS_ADDR)")
	}

	if c.Monitoring.Enabled && (c.Monitoring.Port < 1 || c.Monitoring.Port > 65535) {
		add("monitoring.port must be a valid TCP port, got %d", c.Monitoring.Port)
	}

	if c.Reminders.Enabled && c.Reminders.IntervalHours < 1 {
		add("reminders.interval_hours must be at least 1, got %d", c.Reminders.IntervalHours)
	}

	problems = append(problems, c.databaseProblems()...)

	return problemsError(problems)
}

// ValidateDatabase validates the PostgreSQL settings only.
func (c *Config) ValidateDatabase() error {
	return problemsError(c.databaseProblems())
}

func (c *Config) databaseProblems() []string {
	var problems []string

	switch {
	case c.Database.Postgres.Host == "":
		problems = append(problems, "database.postgres.host is required (set POSTGRES_HOST)")
	case c.Database.Postgres.User == "":
		problems = append(problems, "database.postgres.user is required (set POSTGRES_USER)")
	case c.Database.Postgres.DBName == "":
		problems = append(problems, "database.postgres.dbname is required (set POSTGRES_DB)")
	}

	return problems
}

func problemsError(problems []string) error {
	if len(problems) == 0 {
		return nil
	}

	// errors.Join keeps every problem visible in a single startup error.
	errs := make([]error, 0, len(problems))
	for _, p := range problems {
		errs = append(errs, errors.New(p))
	}
	return fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
}
