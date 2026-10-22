package database

import (
	"context"
	"errors"
	"io/fs"
	"mygo-framework/app/models"
	"mygo-framework/database/migrations"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestOpenMigratesAndPersistsThroughSharedPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "notes database.db")

	first, err := Open(ctx, path, migrations.FS())
	if err != nil {
		t.Fatalf("open first database: %v", err)
	}
	if first.GORM.ConnPool != first.SQL {
		t.Fatal("GORM does not use the owned sql.DB")
	}
	if got := first.SQL.Stats().MaxOpenConnections; got != maxOpenConnections {
		t.Fatalf("max open connections = %d, want %d", got, maxOpenConnections)
	}

	var busyTimeout int
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy timeout: %v", err)
	}
	if busyTimeout != busyTimeoutMilliseconds {
		t.Fatalf("busy timeout = %d, want %d", busyTimeout, busyTimeoutMilliseconds)
	}

	var foreignKeys int
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign key mode: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign keys = %d, want enabled", foreignKeys)
	}

	var journalMode string
	if err := first.SQL.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal mode = %q, want wal", journalMode)
	}

	want := models.Note{Body: "persists across reopen"}
	if err := first.GORM.WithContext(ctx).Create(&want).Error; err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first database: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first database again: %v", err)
	}
	if err := first.SQL.PingContext(ctx); err == nil {
		t.Fatal("owned sql.DB remains open after Close")
	}

	second, err := Open(ctx, path, migrations.FS())
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	t.Cleanup(func() {
		if err := second.Close(); err != nil {
			t.Errorf("close reopened database: %v", err)
		}
	})

	var got models.Note
	if err := second.GORM.WithContext(ctx).First(&got, want.ID).Error; err != nil {
		t.Fatalf("reload note: %v", err)
	}
	if got.Body != want.Body {
		t.Fatalf("reloaded body = %q, want %q", got.Body, want.Body)
	}
}

func TestOpenRejectsMissingInputs(t *testing.T) {
	t.Parallel()

	if _, err := Open(context.Background(), "", migrations.FS()); err == nil {
		t.Fatal("Open accepted an empty database path")
	}
	if _, err := Open(context.Background(), filepath.Join(t.TempDir(), "notes.db"), nil); err == nil {
		t.Fatal("Open accepted a nil migration filesystem")
	}
	if _, _, err := InspectMigrations(context.Background(), filepath.Join(t.TempDir(), "notes.db"), nil); err == nil {
		t.Fatal("InspectMigrations accepted a nil migration filesystem")
	}
}

func TestInspectMigrationsDoesNotCreateOrMigrate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "offline", "notes.db")
	allMigrations := migrationFiles(
		migrationFile{"00001_first.sql", "CREATE TABLE first_record (id INTEGER PRIMARY KEY);"},
		migrationFile{"00002_second.sql", "CREATE TABLE second_record (id INTEGER PRIMARY KEY);"},
	)

	statuses, exists, err := InspectMigrations(ctx, path, allMigrations)
	if err != nil {
		t.Fatalf("inspect missing database: %v", err)
	}
	if exists {
		t.Fatal("missing database reported as existing")
	}
	if len(statuses) != 2 || statuses[0].Applied || statuses[1].Applied {
		t.Fatalf("missing database statuses = %#v, want two pending migrations", statuses)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("status created missing database: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create data directory: %v", err)
	}
	if _, err := Migrate(ctx, path, migrationFiles(
		migrationFile{"00001_first.sql", "CREATE TABLE first_record (id INTEGER PRIMARY KEY);"},
	)); err != nil {
		t.Fatalf("apply initial migration: %v", err)
	}

	statuses, exists, err = InspectMigrations(ctx, path, allMigrations)
	if err != nil {
		t.Fatalf("inspect database with pending migration: %v", err)
	}
	if !exists || len(statuses) != 2 || !statuses[0].Applied || statuses[1].Applied {
		t.Fatalf("database statuses = %#v, want applied then pending", statuses)
	}

	sqlDB, err := openSQL(ctx, path, true)
	if err != nil {
		t.Fatalf("open inspected database: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()
	var secondTableExists bool
	if err := sqlDB.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'second_record')",
	).Scan(&secondTableExists); err != nil {
		t.Fatalf("check pending migration table: %v", err)
	}
	if secondTableExists {
		t.Fatal("InspectMigrations applied the pending migration")
	}
}

func TestMigrateAppliesPendingMigrations(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "notes.db")
	applied, err := Migrate(t.Context(), path, migrationFiles(
		migrationFile{"00001_first.sql", "CREATE TABLE first_record (id INTEGER PRIMARY KEY);"},
		migrationFile{"00002_second.sql", "CREATE TABLE second_record (id INTEGER PRIMARY KEY);"},
	))
	if err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if applied != 2 {
		t.Fatalf("applied migrations = %d, want 2", applied)
	}

	applied, err = Migrate(t.Context(), path, migrationFiles(
		migrationFile{"00001_first.sql", "CREATE TABLE first_record (id INTEGER PRIMARY KEY);"},
		migrationFile{"00002_second.sql", "CREATE TABLE second_record (id INTEGER PRIMARY KEY);"},
	))
	if err != nil {
		t.Fatalf("migrate current database: %v", err)
	}
	if applied != 0 {
		t.Fatalf("reapplied migrations = %d, want 0", applied)
	}
}

type migrationFile struct {
	name      string
	statement string
}

func migrationFiles(migrations ...migrationFile) fs.FS {
	files := fstest.MapFS{}
	for _, migration := range migrations {
		files[migration.name] = &fstest.MapFile{Data: []byte(
			"-- +goose Up\n" + migration.statement + "\n",
		)}
	}
	return files
}
