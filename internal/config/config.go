package config

import (
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App        AppConfig        `yaml:"app"`
	Telegram   TelegramConfig   `yaml:"telegram"`
	Database   DatabaseConfig   `yaml:"database"`
	Redis      RedisConfig      `yaml:"redis"`
	Backup     BackupConfig     `yaml:"backup"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Logging    LoggingConfig    `yaml:"logging"`
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
	Version     string `yaml:"version"`
}

type TelegramConfig struct {
	BotToken   string `yaml:"bot_token"`
	WebhookURL string `yaml:"webhook_url"`
	Debug      bool   `yaml:"debug"`
}

type DatabaseConfig struct {
	Postgres PostgresConfig `yaml:"postgres"`
}

type PostgresConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	DBName         string `yaml:"dbname"`
	SSLMode        string `yaml:"sslmode"`
	MaxConnections int    `yaml:"max_connections"`
	MigrationTable string `yaml:"migration_table"`
}

type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type BackupConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Schedule      string `yaml:"schedule"`
	RetentionDays int    `yaml:"retention_days"`
	StoragePath   string `yaml:"storage_path"`
}

type MonitoringConfig struct {
	PrometheusEnabled bool   `yaml:"prometheus_enabled"`
	PrometheusPort    int    `yaml:"prometheus_port"`
	HealthCheckPort   int    `yaml:"health_check_port"`
	LogLevel          string `yaml:"log_level"`
}

type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

func Load(configPath string) (*Config, error) {
	// Загружаем .env файл если существует
	err := godotenv.Load("env.env")
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Предварительная замена переменных окружения в YAML
	expandedData := []byte(os.ExpandEnv(string(data)))

	var config Config
	if err := yaml.Unmarshal(expandedData, &config); err != nil {
		return nil, err
	}

	// Заменяем переменные окружения
	// config.expandEnvVars()

	return &config, nil
}

func (c *Config) expandEnvVars() {
	c.Telegram.BotToken = os.ExpandEnv(c.Telegram.BotToken)
	c.Database.Postgres.Host = os.ExpandEnv(c.Database.Postgres.Host)
	c.Database.Postgres.User = os.ExpandEnv(c.Database.Postgres.User)
	c.Database.Postgres.Password = os.ExpandEnv(c.Database.Postgres.Password)
	c.Database.Postgres.DBName = os.ExpandEnv(c.Database.Postgres.DBName)
	c.Database.Postgres.SSLMode = os.ExpandEnv(c.Database.Postgres.SSLMode)
	c.Database.Postgres.MigrationTable = os.ExpandEnv(c.Database.Postgres.MigrationTable)

	c.Redis.Address = os.ExpandEnv(c.Redis.Address)
	c.Redis.Password = os.ExpandEnv(c.Redis.Password)
	c.Backup.StoragePath = os.ExpandEnv(c.Backup.StoragePath)
	c.Logging.FilePath = os.ExpandEnv(c.Logging.FilePath)
}
