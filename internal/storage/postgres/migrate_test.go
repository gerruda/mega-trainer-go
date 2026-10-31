package postgres

import (
	"slices"
	"testing"
	"testing/fstest"

	"mega-trainer-go/migrations"
)

func TestMigratorVersionsSortsAndSkipsRollbackScripts(t *testing.T) {
	t.Parallel()

	migrator := NewMigrator(nil, fstest.MapFS{
		"0002_second.sql":       &fstest.MapFile{},
		"0001_first.sql":        &fstest.MapFile{},
		"0002_second.down.sql":  &fstest.MapFile{},
		"notes.md":              &fstest.MapFile{},
		"nested/0003_third.sql": &fstest.MapFile{},
	})

	versions, err := migrator.versions()
	if err != nil {
		t.Fatalf("versions: %v", err)
	}

	want := []string{"0001_first", "0002_second"}
	if !slices.Equal(versions, want) {
		t.Fatalf("versions = %v, want %v", versions, want)
	}
}

func TestEmbeddedMigrationsAreDiscovered(t *testing.T) {
	t.Parallel()

	versions, err := NewMigrator(nil, migrations.FS).versions()
	if err != nil {
		t.Fatalf("versions: %v", err)
	}

	for _, expected := range []string{"0001_init", "0002_measurements"} {
		if !slices.Contains(versions, expected) {
			t.Fatalf("embedded migrations %v do not contain %q", versions, expected)
		}
	}
}
