package sqlite

import (
	"mygo-framework/database/migrations"
	"mygo-framework/internal/database"
	"mygo-framework/internal/settingstest"
	"mygo-framework/settings"
	"path/filepath"
	"testing"
)

func TestStoreContract(t *testing.T) {
	t.Parallel()
	settingstest.Run(t, openStore)
}

func TestStoreBorrowsMigratedSharedPool(t *testing.T) {
	t.Parallel()

	database, err := database.Open(t.Context(), filepath.Join(t.TempDir(), "notes.db"), migrations.FS())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	store, err := New(database.SQL)
	if err != nil {
		t.Fatalf("new settings store: %v", err)
	}
	if store.database != database.SQL {
		t.Fatal("settings store does not borrow the shared pool")
	}
	if err := store.Write(t.Context(), "theme", []byte(`"dark"`)); err != nil {
		t.Fatalf("write migrated table: %v", err)
	}

	var got string
	if err := database.SQL.QueryRowContext(
		t.Context(),
		`SELECT CAST(value_json AS TEXT) FROM mygo_framework_settings WHERE name = ?`,
		"theme",
	).Scan(&got); err != nil {
		t.Fatalf("query setting through shared pool: %v", err)
	}
	if got != `"dark"` {
		t.Fatalf("stored JSON = %q, want %q", got, `"dark"`)
	}
}

func TestNewRejectsNilDatabase(t *testing.T) {
	t.Parallel()
	if _, err := New(nil); err == nil {
		t.Fatal("New accepted a nil database")
	}
}

func openStore(t *testing.T, location string) (settings.Store, func()) {
	t.Helper()

	database, err := database.Open(t.Context(), location, migrations.FS())
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	store, err := New(database.SQL)
	if err != nil {
		_ = database.Close()
		t.Fatalf("new settings store: %v", err)
	}

	return store, func() {
		if err := database.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	}
}
