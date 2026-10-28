package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

// advisoryLockKey serializes migration runners started by different processes.
const advisoryLockKey int64 = 7_301_202_501

// Migrator applies SQL migrations embedded in the binary.
type Migrator struct {
	db *sqlx.DB
	fs fs.FS
}

// NewMigrator creates a migration runner reading *.sql files from fsys.
func NewMigrator(db *sqlx.DB, fsys fs.FS) *Migrator {
	return &Migrator{db: db, fs: fsys}
}

// Status describes one migration and whether it has been applied.
type Status struct {
	Version string
	Applied bool
}

// Up applies all pending migrations in lexicographical order. It returns the
// list of versions that were applied during this call.
func (m *Migrator) Up(ctx context.Context) ([]string, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}

	unlock, err := m.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()

	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions, err := m.versions()
	if err != nil {
		return nil, err
	}

	var appliedNow []string
	for _, version := range versions {
		if applied[version] {
			continue
		}

		script, err := fs.ReadFile(m.fs, version+".sql")
		if err != nil {
			return appliedNow, fmt.Errorf("read migration %s: %w", version, err)
		}

		if err := m.applyOne(ctx, version, string(script)); err != nil {
			return appliedNow, err
		}
		appliedNow = append(appliedNow, version)
	}

	return appliedNow, nil
}

// Down rolls back the most recently applied migration.
func (m *Migrator) Down(ctx context.Context) (string, error) {
	if err := m.ensureTable(ctx); err != nil {
		return "", err
	}

	unlock, err := m.lock(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()

	var version string
	err = m.db.GetContext(ctx, &version,
		`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("no migrations to roll back")
	}
	if err != nil {
		return "", fmt.Errorf("select last migration: %w", err)
	}

	script, err := fs.ReadFile(m.fs, version+".down.sql")
	if err != nil {
		return "", fmt.Errorf("read down migration %s: %w", version, err)
	}

	tx, err := m.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		return "", fmt.Errorf("roll back migration %s: %w", version, err)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
		return "", fmt.Errorf("delete migration record %s: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit rollback %s: %w", version, err)
	}

	return version, nil
}

// Status reports every known migration and whether it has been applied.
func (m *Migrator) Status(ctx context.Context) ([]Status, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}

	applied, err := m.appliedVersions(ctx)
	if err != nil {
		return nil, err
	}

	versions, err := m.versions()
	if err != nil {
		return nil, err
	}

	statuses := make([]Status, 0, len(versions))
	for _, version := range versions {
		statuses = append(statuses, Status{Version: version, Applied: applied[version]})
	}

	return statuses, nil
}

func (m *Migrator) ensureTable(ctx context.Context) error {
	const query = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`
	if _, err := m.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func (m *Migrator) lock(ctx context.Context) (func(), error) {
	if _, err := m.db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey); err != nil {
		return nil, fmt.Errorf("acquire migration lock: %w", err)
	}

	return func() {
		// The context may already be cancelled at this point, so use a fresh
		// one. The unlock is best effort: PostgreSQL also releases advisory
		// locks when the connection closes.
		_, _ = m.db.ExecContext(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, advisoryLockKey)
	}, nil
}

func (m *Migrator) applyOne(ctx context.Context, version, script string) error {
	tx, err := m.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}

	return nil
}

func (m *Migrator) appliedVersions(ctx context.Context) (map[string]bool, error) {
	var versions []string
	if err := m.db.SelectContext(ctx, &versions, `SELECT version FROM schema_migrations`); err != nil {
		return nil, fmt.Errorf("select applied migrations: %w", err)
	}

	applied := make(map[string]bool, len(versions))
	for _, v := range versions {
		applied[v] = true
	}
	return applied, nil
}

// versions lists migration versions (file names without the .sql suffix),
// ignoring rollback scripts.
func (m *Migrator) versions() ([]string, error) {
	entries, err := fs.ReadDir(m.fs, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	var versions []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".down.sql") {
			continue
		}
		versions = append(versions, strings.TrimSuffix(name, ".sql"))
	}

	sort.Strings(versions)
	return versions, nil
}
