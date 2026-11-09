package main

import (
	"context"
	"mygo-framework/settings"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func TestNativePreferenceControl(t *testing.T) {
	s := &screen{}
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 640, 360)
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if !s.compact {
		t.Fatal("checkbox did not update the retained preference")
	}
	if err := tt.Click("Compact spacing"); err != nil {
		t.Fatal(err)
	}
	if s.compact {
		t.Fatal("checkbox did not toggle back")
	}
}

func TestOpenPreferencesMigratesAndPersists(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "data")

	db, values, err := openPreferences(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	var applied int
	if err := db.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM goose_db_version WHERE version_id = 1 AND is_applied").Scan(&applied); err != nil || applied != 1 {
		t.Fatalf("settings migration applied = %d, %v", applied, err)
	}
	if got, err := settings.Get(ctx, values, compactKey); err != nil || got {
		t.Fatalf("default preference = %v, %v", got, err)
	}
	if err := settings.Set(ctx, values, compactKey, true); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, values, err = openPreferences(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if got, err := settings.Get(ctx, values, compactKey); err != nil || !got {
		t.Fatalf("reloaded preference = %v, %v", got, err)
	}
}

func TestPreferenceSaveFailure(t *testing.T) {
	db, values, err := openPreferences(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &screen{compact: true}
	if err := s.save(values); err != nil || s.saveError != "" {
		t.Fatalf("save = %v, error text %q", err, s.saveError)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.save(values); err == nil || s.saveError == "" {
		t.Fatal("save failure must keep the window open with a visible error")
	}
	tt := ui.NewTester(func(c *ui.Context) { theme.Apply(c); s.view(c) }, 640, 360)
	if !tt.HasText(s.saveError) {
		t.Fatalf("error is not visible: %v", tt.Texts())
	}
}

func TestOpenPreferencesAcceptsRelativeDataDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "launch"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "launch"))

	db, _, err := openPreferences(t.Context(), filepath.Join("..", "..", filepath.Base(root), "data"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "app.db")); err != nil {
		t.Fatalf("database is not in the directory relative to the working directory: %v", err)
	}
}
