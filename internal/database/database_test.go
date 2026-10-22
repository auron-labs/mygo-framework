package database

import (
	"context"
	"mygo-framework/app/models"
	"mygo-framework/database/migrations"
	"path/filepath"
	"testing"
)

func TestOpenMigratesAndPersistsThroughSharedPool(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "notes.db")

	first, err := Open(ctx, path, migrations.FS())
	if err != nil {
		t.Fatalf("open first database: %v", err)
	}
	if first.GORM.ConnPool != first.SQL {
		t.Fatal("GORM does not use the owned sql.DB")
	}

	want := models.Note{Body: "persists across reopen"}
	if err := first.GORM.WithContext(ctx).Create(&want).Error; err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first database: %v", err)
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
