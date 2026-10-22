package settings_test

import (
	"encoding/json"
	"errors"
	"mygo-framework/database/migrations"
	"mygo-framework/internal/database"
	"mygo-framework/settings"
	settingsbbolt "mygo-framework/settings/bbolt"
	settingssqlite "mygo-framework/settings/sqlite"
	"path/filepath"
	"testing"
)

type openBackend func(t *testing.T, path string) (settings.Store, func())

func TestPersistentBackendsTypedAcceptance(t *testing.T) {
	t.Parallel()

	backends := []struct {
		name      string
		extension string
		open      openBackend
	}{
		{name: "SQLite", extension: "sqlite", open: openSQLite},
		{name: "bbolt", extension: "bbolt", open: openBbolt},
	}

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			t.Parallel()
			testPersistentBackend(t, filepath.Join(t.TempDir(), "settings."+backend.extension), backend.open)
		})
	}
}

func testPersistentBackend(t *testing.T, path string, open openBackend) {
	t.Helper()

	enabled := settings.NewKey("enabled", true, nil)
	label := settings.NewKey("label", "default", nil)
	count := settings.NewKey("count", 3, func(value int) error {
		if value < 0 {
			return errors.New("must be non-negative")
		}
		return nil
	})

	firstStore, closeFirst := open(t, path)
	firstClosed := false
	t.Cleanup(func() {
		if !firstClosed {
			closeFirst()
		}
	})
	first, err := settings.New(firstStore, enabled, label, count)
	if err != nil {
		t.Fatalf("new first settings: %v", err)
	}
	if err := settings.Set(t.Context(), first, enabled, false); err != nil {
		t.Fatalf("set false: %v", err)
	}
	if err := settings.Set(t.Context(), first, label, ""); err != nil {
		t.Fatalf("set empty string: %v", err)
	}
	if err := settings.Set(t.Context(), first, count, 0); err != nil {
		t.Fatalf("set zero: %v", err)
	}

	flag, err := settings.NewOverride(count, 5)
	if err != nil {
		t.Fatalf("new flag override: %v", err)
	}
	environment, err := settings.NewOverride(count, 4)
	if err != nil {
		t.Fatalf("new environment override: %v", err)
	}
	overlay, err := settings.NewOverlay(firstStore, []settings.Override{flag}, []settings.Override{environment})
	if err != nil {
		t.Fatalf("new overlay: %v", err)
	}
	overlaid, err := settings.New(overlay, count)
	if err != nil {
		t.Fatalf("new overlaid settings: %v", err)
	}
	if got, err := settings.Get(t.Context(), overlaid, count); err != nil || got != 5 {
		t.Fatalf("overlaid count = %d, error = %v, want 5", got, err)
	}

	closeFirst()
	firstClosed = true
	secondStore, closeSecond := open(t, path)
	t.Cleanup(closeSecond)
	second, err := settings.New(secondStore, enabled, label, count)
	if err != nil {
		t.Fatalf("new reopened settings: %v", err)
	}
	if got, err := settings.Get(t.Context(), second, enabled); err != nil || got {
		t.Fatalf("reopened enabled = %v, error = %v, want false", got, err)
	}
	if got, err := settings.Get(t.Context(), second, label); err != nil || got != "" {
		t.Fatalf("reopened label = %q, error = %v, want empty", got, err)
	}
	if got, err := settings.Get(t.Context(), second, count); err != nil || got != 0 {
		t.Fatalf("reopened count = %d, error = %v, want persisted zero", got, err)
	}

	if err := secondStore.Write(t.Context(), count.Name(), json.RawMessage(`"wrong type"`)); err != nil {
		t.Fatalf("write wrong typed JSON: %v", err)
	}
	if _, err := settings.Get(t.Context(), second, count); !errors.Is(err, settings.ErrInvalidValue) {
		t.Fatalf("wrong-type error = %v, want ErrInvalidValue", err)
	}
	if err := secondStore.Write(t.Context(), count.Name(), json.RawMessage("[")); err == nil {
		t.Fatal("backend accepted malformed JSON")
	}
	if err := settings.Set(t.Context(), second, count, 7); err != nil {
		t.Fatalf("restore valid count: %v", err)
	}
	if err := settings.Set(t.Context(), second, count, -1); !errors.Is(err, settings.ErrInvalidValue) {
		t.Fatalf("invalid Set error = %v, want ErrInvalidValue", err)
	}
	if got, err := settings.Get(t.Context(), second, count); err != nil || got != 7 {
		t.Fatalf("count after rejected Set = %d, error = %v, want 7", got, err)
	}
}

func openSQLite(t *testing.T, path string) (settings.Store, func()) {
	t.Helper()

	database, err := database.Open(t.Context(), path, migrations.FS())
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	store, err := settingssqlite.New(database.SQL)
	if err != nil {
		_ = database.Close()
		t.Fatalf("new SQLite settings: %v", err)
	}

	return store, func() {
		if err := database.Close(); err != nil {
			t.Errorf("close SQLite database: %v", err)
		}
	}
}

func openBbolt(t *testing.T, path string) (settings.Store, func()) {
	t.Helper()

	store, err := settingsbbolt.Open(path)
	if err != nil {
		t.Fatalf("open bbolt settings: %v", err)
	}

	return store, func() {
		if err := store.Close(); err != nil {
			t.Errorf("close bbolt settings: %v", err)
		}
	}
}
