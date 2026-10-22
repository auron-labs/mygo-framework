package database

import (
	"context"
	"mygo-framework/app/models"
	"mygo-framework/database/migrations"
	"path/filepath"
	"strings"
	"testing"
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
}
