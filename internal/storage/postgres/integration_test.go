//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"mega-trainer-go/internal/domain"
	"mega-trainer-go/migrations"
)

// openTestDB connects to the database from TEST_POSTGRES_DSN and isolates every
// test in its own schema, so a developer database is never touched.
func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_DSN is not set; integration tests are skipped")
	}

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())

	admin, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = admin.Close()
	})

	db, err := sqlx.Connect("postgres", dsn+" search_path="+schema)
	if err != nil {
		t.Fatalf("connect with search_path %s: %v", schema, err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db
}

func migratedDB(t *testing.T) *sqlx.DB {
	t.Helper()

	db := openTestDB(t)
	if _, err := NewMigrator(db, migrations.FS).Up(context.Background()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func tableExists(t *testing.T, db *sqlx.DB, table string) bool {
	t.Helper()

	var exists bool
	err := db.Get(&exists, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_name = $1
		)`, table)
	if err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	return exists
}

func TestMigratorUpIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	migrator := NewMigrator(db, migrations.FS)

	applied, err := migrator.Up(ctx)
	if err != nil {
		t.Fatalf("first up: %v", err)
	}
	if len(applied) != 2 || applied[0] != "0001_init" || applied[1] != "0002_measurements" {
		t.Fatalf("applied = %v, want [0001_init 0002_measurements]", applied)
	}

	again, err := migrator.Up(ctx)
	if err != nil {
		t.Fatalf("second up: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second up applied %v, want nothing", again)
	}

	statuses, err := migrator.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, status := range statuses {
		if !status.Applied {
			t.Fatalf("migration %s is not applied: %+v", status.Version, statuses)
		}
	}

	if !tableExists(t, db, "users") || !tableExists(t, db, "measurements") {
		t.Fatal("expected tables were not created by the migrations")
	}
}

func TestMigratorDownRollsBackLastMigration(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	migrator := NewMigrator(db, migrations.FS)

	if _, err := migrator.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}

	version, err := migrator.Down(ctx)
	if err != nil {
		t.Fatalf("down: %v", err)
	}
	if version != "0002_measurements" {
		t.Fatalf("rolled back %q, want 0002_measurements", version)
	}
	if tableExists(t, db, "measurements") {
		t.Fatal("measurements table still exists after rollback")
	}
	if !tableExists(t, db, "users") {
		t.Fatal("users table must survive rolling back only the last migration")
	}

	if _, err := migrator.Up(ctx); err != nil {
		t.Fatalf("re-apply after rollback: %v", err)
	}
	if !tableExists(t, db, "measurements") {
		t.Fatal("measurements table was not recreated")
	}
}

func TestUserRepoUpsert(t *testing.T) {
	db := migratedDB(t)
	ctx := context.Background()
	repo := NewUserRepo(db)

	created, err := repo.Upsert(ctx, &domain.User{
		TelegramID:   424242,
		Username:     "gopher",
		FirstName:    "Gordon",
		LastName:     "Freeman",
		LanguageCode: "ru",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("id = %d, want a generated id", created.ID)
	}
	if !created.IsActive {
		t.Fatal("new user must be active")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatal("timestamps were not set by the database")
	}

	updated, err := repo.Upsert(ctx, &domain.User{
		TelegramID:   424242,
		Username:     "gopher-pro",
		FirstName:    "Gordon",
		LanguageCode: "en",
	})
	if err != nil {
		t.Fatalf("update user: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("upsert created a new row: id %d -> %d", created.ID, updated.ID)
	}
	if updated.Username != "gopher-pro" || updated.LanguageCode != "en" {
		t.Fatalf("profile was not updated: %+v", updated)
	}

	found, err := repo.GetByTelegramID(ctx, 424242)
	if err != nil {
		t.Fatalf("get by telegram id: %v", err)
	}
	if found.ID != created.ID || found.Username != "gopher-pro" {
		t.Fatalf("found = %+v, want the updated user", found)
	}

	if _, err := repo.GetByTelegramID(ctx, 999999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}

	active, err := repo.ListActive(ctx, 10)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].TelegramID != 424242 {
		t.Fatalf("active users = %+v, want the created user", active)
	}
}

func TestMeasurementRepoCreateAndList(t *testing.T) {
	db := migratedDB(t)
	ctx := context.Background()

	user, err := NewUserRepo(db).Upsert(ctx, &domain.User{TelegramID: 777001})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	repo := NewMeasurementRepo(db)

	base := time.Now().UTC().Truncate(time.Second)
	bodyFat := 18.4
	neck := 40.0
	waist := 90.0

	older, err := repo.Create(ctx, &domain.Measurement{
		UserID:     user.ID,
		WeightKG:   70.5,
		BodyFatPct: &bodyFat,
		NeckCM:     &neck,
		WaistCM:    &waist,
		MeasuredAt: base.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("create measurement: %v", err)
	}
	if older.ID <= 0 || older.BodyFatPct == nil || *older.BodyFatPct != bodyFat {
		t.Fatalf("stored measurement = %+v", older)
	}
	if older.HipsCM != nil {
		t.Fatalf("hips = %v, want NULL for a male measurement", *older.HipsCM)
	}

	newer, err := repo.Create(ctx, &domain.Measurement{
		UserID:     user.ID,
		WeightKG:   69.8,
		MeasuredAt: base,
	})
	if err != nil {
		t.Fatalf("create second measurement: %v", err)
	}

	list, err := repo.ListByUserID(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("list measurements: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	// Newest first.
	if list[0].ID != newer.ID || list[1].ID != older.ID {
		t.Fatalf("order = [%d %d], want [%d %d]", list[0].ID, list[1].ID, newer.ID, older.ID)
	}

	limited, err := repo.ListByUserID(ctx, user.ID, 1)
	if err != nil {
		t.Fatalf("list with limit: %v", err)
	}
	if len(limited) != 1 || limited[0].ID != newer.ID {
		t.Fatalf("limited = %+v, want only the newest measurement", limited)
	}

	otherUser, err := NewUserRepo(db).Upsert(ctx, &domain.User{TelegramID: 777002})
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	if _, err := repo.Create(ctx, &domain.Measurement{UserID: otherUser.ID, WeightKG: 90, MeasuredAt: base}); err != nil {
		t.Fatalf("create measurement for other user: %v", err)
	}

	onlyFirst, err := repo.ListByUserID(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("list for first user: %v", err)
	}
	if len(onlyFirst) != 2 {
		t.Fatalf("measurements leaked between users: %d rows", len(onlyFirst))
	}
}

func TestMeasurementsAreDeletedWithUser(t *testing.T) {
	db := migratedDB(t)
	ctx := context.Background()

	user, err := NewUserRepo(db).Upsert(ctx, &domain.User{TelegramID: 888001})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	repo := NewMeasurementRepo(db)
	if _, err := repo.Create(ctx, &domain.Measurement{
		UserID: user.ID, WeightKG: 70, MeasuredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("create measurement: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	list, err := repo.ListByUserID(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("list measurements: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("measurements were not removed by ON DELETE CASCADE: %d rows", len(list))
	}
}
