package postgres

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"mega-trainer-go/internal/config"
)

// Migration представляет одну миграцию
type Migration struct {
	ID      int
	Name    string
	UpSQL   string
	DownSQL string
}

// MigrationManager управляет миграциями БД
type MigrationManager struct {
	db *sqlx.DB
}

// NewMigrationManager создает новый менеджер миграций
func NewMigrationManager(db *sqlx.DB) *MigrationManager {
	return &MigrationManager{db: db}
}

// EnsureMigrationsTable создает таблицу миграций если она не существует
func (m *MigrationManager) EnsureMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`

	_, err := m.db.Exec(query)
	return err
}

// GetAppliedMigrations возвращает список примененных миграций
func (m *MigrationManager) GetAppliedMigrations() (map[string]bool, error) {
	applied := make(map[string]bool)

	query := `SELECT name FROM migrations ORDER BY id`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan migration name: %w", err)
		}
		applied[name] = true
	}

	return applied, nil
}

// MarkMigrationApplied отмечает миграцию как примененную
func (m *MigrationManager) MarkMigrationApplied(name string) error {
	query := `INSERT INTO migrations (name) VALUES ($1)`
	_, err := m.db.Exec(query, name)
	return err
}

// MarkMigrationRolledBack отмечает миграцию как откатанную
func (m *MigrationManager) MarkMigrationRolledBack(name string) error {
	query := `DELETE FROM migrations WHERE name = $1`
	_, err := m.db.Exec(query, name)
	return err
}

// GetAllMigrations возвращает все доступные миграции
func (m *MigrationManager) GetAllMigrations() []Migration {
	return []Migration{
		{
			ID:   1,
			Name: "create_users_table",
			UpSQL: `
				CREATE TABLE users (
					id SERIAL PRIMARY KEY,
					telegram_id BIGINT UNIQUE NOT NULL,
					username VARCHAR(255),
					first_name VARCHAR(255),
					last_name VARCHAR(255),
					language_code VARCHAR(10),
					is_active BOOLEAN DEFAULT true,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					deleted_at TIMESTAMP WITH TIME ZONE
				);
				
				CREATE INDEX idx_users_telegram_id ON users(telegram_id);
				CREATE INDEX idx_users_username ON users(username);
				CREATE INDEX idx_users_active ON users(is_active) WHERE is_active = true;
			`,
			DownSQL: `DROP TABLE IF EXISTS users;`,
		},
		{
			ID:   2,
			Name: "create_chats_table",
			UpSQL: `
				CREATE TABLE chats (
					id SERIAL PRIMARY KEY,
					telegram_id BIGINT UNIQUE NOT NULL,
					type VARCHAR(50) NOT NULL,
					title VARCHAR(255),
					username VARCHAR(255),
					first_name VARCHAR(255),
					last_name VARCHAR(255),
					description TEXT,
					is_active BOOLEAN DEFAULT true,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					deleted_at TIMESTAMP WITH TIME ZONE
				);
				
				CREATE INDEX idx_chats_telegram_id ON chats(telegram_id);
				CREATE INDEX idx_chats_type ON chats(type);
				CREATE INDEX idx_chats_active ON chats(is_active) WHERE is_active = true;
			`,
			DownSQL: `DROP TABLE IF EXISTS chats;`,
		},
		{
			ID:   3,
			Name: "create_user_settings_table",
			UpSQL: `
				CREATE TABLE user_settings (
					id SERIAL PRIMARY KEY,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					measurement_system VARCHAR(10) DEFAULT 'metric',
					notifications_enabled BOOLEAN DEFAULT true,
					language VARCHAR(10) DEFAULT 'ru',
					daily_calorie_goal INTEGER,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					UNIQUE(user_id)
				);
				
				CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);
			`,
			DownSQL: `DROP TABLE IF EXISTS user_settings;`,
		},
		{
			ID:   4,
			Name: "create_body_measurements_table",
			UpSQL: `
				CREATE TABLE body_measurements (
					id SERIAL PRIMARY KEY,
					user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
					weight DECIMAL(5,2),
					height DECIMAL(5,2),
					neck DECIMAL(5,2),
					waist DECIMAL(5,2),
					hips DECIMAL(5,2),
					bmi DECIMAL(4,2),
					bmr DECIMAL(8,2),
					tdee DECIMAL(8,2),
					calories INTEGER,
					protein INTEGER,
					fat INTEGER,
					carbs INTEGER,
					created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
					updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
				);
				
				CREATE INDEX idx_body_measurements_user_id ON body_measurements(user_id);
				CREATE INDEX idx_body_measurements_created_at ON body_measurements(created_at DESC);
			`,
			DownSQL: `DROP TABLE IF EXISTS body_measurements;`,
		},
		{
			ID:   5,
			Name: "add_user_metrics_fields",
			UpSQL: `
				ALTER TABLE users 
				ADD COLUMN IF NOT EXISTS age INTEGER,
				ADD COLUMN IF NOT EXISTS gender VARCHAR(10),
				ADD COLUMN IF NOT EXISTS activity_level VARCHAR(20),
				ADD COLUMN IF NOT EXISTS goal VARCHAR(20);
			`,
			DownSQL: `
				ALTER TABLE users 
				DROP COLUMN IF EXISTS age,
				DROP COLUMN IF EXISTS gender,
				DROP COLUMN IF EXISTS activity_level,
				DROP COLUMN IF EXISTS goal;
			`,
		},
	}
}

// MigrateUp применяет все pending миграции
func (m *MigrationManager) MigrateUp() error {
	// Создаем таблицу миграций если не существует
	if err := m.EnsureMigrationsTable(); err != nil {
		return fmt.Errorf("failed to ensure migrations table: %w", err)
	}

	// Получаем примененные миграции
	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Получаем все доступные миграции
	allMigrations := m.GetAllMigrations()

	// Применяем миграции по порядку
	for _, migration := range allMigrations {
		if applied[migration.Name] {
			fmt.Printf("Migration %s already applied, skipping\n", migration.Name)
			continue
		}

		fmt.Printf("Applying migration: %s\n", migration.Name)

		// Начинаем транзакцию
		tx, err := m.db.Beginx()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Применяем миграцию
		if _, err := tx.Exec(migration.UpSQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %s: %w", migration.Name, err)
		}

		// Отмечаем как примененную
		if _, err := tx.Exec(`INSERT INTO migrations (name) VALUES ($1)`, migration.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to mark migration %s as applied: %w", migration.Name, err)
		}

		// Коммитим транзакцию
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", migration.Name, err)
		}

		fmt.Printf("Successfully applied migration: %s\n", migration.Name)
	}

	fmt.Println("All migrations applied successfully")
	return nil
}

// MigrateDown откатывает последнюю миграцию
func (m *MigrationManager) MigrateDown() error {
	// Получаем примененные миграции
	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Получаем все доступные миграции
	allMigrations := m.GetAllMigrations()

	// Находим последнюю примененную миграцию
	var lastMigration *Migration
	for i := len(allMigrations) - 1; i >= 0; i-- {
		if applied[allMigrations[i].Name] {
			lastMigration = &allMigrations[i]
			break
		}
	}

	if lastMigration == nil {
		fmt.Println("No migrations to roll back")
		return nil
	}

	fmt.Printf("Rolling back migration: %s\n", lastMigration.Name)

	// Начинаем транзакцию
	tx, err := m.db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Откатываем миграцию
	if _, err := tx.Exec(lastMigration.DownSQL); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to roll back migration %s: %w", lastMigration.Name, err)
	}

	// Удаляем запись о миграции
	if _, err := tx.Exec(`DELETE FROM migrations WHERE name = $1`, lastMigration.Name); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove migration %s record: %w", lastMigration.Name, err)
	}

	// Коммитим транзакцию
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	fmt.Printf("Successfully rolled back migration: %s\n", lastMigration.Name)
	return nil
}

// Status показывает статус миграций
func (m *MigrationManager) Status() error {
	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	allMigrations := m.GetAllMigrations()

	fmt.Println("Migration Status:")
	fmt.Println("=================")

	for _, migration := range allMigrations {
		status := "PENDING"
		if applied[migration.Name] {
			status = "APPLIED"
		}
		fmt.Printf("%s: %s\n", migration.Name, status)
	}

	return nil
}

// CreateConnection создает подключение к PostgreSQL
func CreateConnection(cfg *config.Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Postgres.Host,
		cfg.Database.Postgres.Port,
		cfg.Database.Postgres.User,
		cfg.Database.Postgres.Password,
		cfg.Database.Postgres.DBName,
		cfg.Database.Postgres.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Проверка подключения
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return db, nil
}
