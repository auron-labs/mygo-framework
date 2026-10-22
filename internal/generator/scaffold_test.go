package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldFocusedKinds(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	migrations := filepath.Join(directory, "database", "migrations")
	if err := os.MkdirAll(migrations, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrations, "00002_existing.sql"), []byte("existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		kind string
		name string
		path string
		want string
	}{
		{kind: "service", name: "AuditLog", path: "app/services/audit_log.go", want: "func NewAuditLog() *AuditLog"},
		{kind: "request", name: "create-entry", path: "app/requests/create_entry.go", want: "type CreateEntryInput struct{}"},
		{kind: "model", name: "APIKey", path: "app/models/api_key.go", want: "type ApiKey struct"},
		{kind: "migration", name: "add_entries", path: "database/migrations/00003_add_entries.sql", want: "-- +goose Down"},
		{kind: "command", name: "sync-data", path: "app/commands/sync_data.go", want: "func NewSyncDataCommand(run func(context.Context) error)"},
	}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			result, err := Scaffold(ScaffoldConfig{Dir: directory, Kind: test.kind, Name: test.name})
			if err != nil {
				t.Fatal(err)
			}
			if result.Path != test.path {
				t.Fatalf("path = %q, want %q", result.Path, test.path)
			}
			// #nosec G304 -- result.Path is produced by the scaffold under the temporary project.
			content, err := os.ReadFile(filepath.Join(directory, result.Path))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), test.want) {
				t.Fatalf("%s does not contain %q:\n%s", result.Path, test.want, content)
			}
		})
	}
}

func TestScaffoldRefusesOverwriteAndKeepsMigrationIdentity(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()

	first, err := Scaffold(ScaffoldConfig{Dir: directory, Kind: "migration", Name: "AddEntries"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != "database/migrations/00001_add_entries.sql" {
		t.Fatalf("first path = %q", first.Path)
	}
	if _, err := Scaffold(ScaffoldConfig{Dir: directory, Kind: "migration", Name: "AddTags"}); err != nil {
		t.Fatal(err)
	}
	_, err = Scaffold(ScaffoldConfig{Dir: directory, Kind: "migration", Name: "AddEntries"})
	if err == nil || !strings.Contains(err.Error(), first.Path) {
		t.Fatalf("duplicate migration error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(directory, "database", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("migration files = %d, want 2", len(entries))
	}

	target := filepath.Join(directory, "app", "services", "billing.go")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	const handwritten = "package services\n\nconst Handwritten = true\n"
	if err := os.WriteFile(target, []byte(handwritten), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Scaffold(ScaffoldConfig{Dir: directory, Kind: "service", Name: "Billing"})
	if err == nil || !strings.Contains(err.Error(), "refuse to overwrite") {
		t.Fatalf("overwrite error = %v", err)
	}
	// #nosec G304 -- target is inside the temporary project.
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != handwritten {
		t.Fatalf("hand-written file changed: %q, %v", content, readErr)
	}
}

func TestScaffoldRejectsUnsafeNamesAndKinds(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", " ../outside", "../outside", "9lives", "two words", "two--parts", "naïve", "CON", "lpt9"} {
		_, err := Scaffold(ScaffoldConfig{Dir: t.TempDir(), Kind: "service", Name: name})
		if err == nil || !strings.Contains(err.Error(), "invalid scaffold name") {
			t.Errorf("name %q error = %v", name, err)
		}
	}

	directory := t.TempDir()
	_, err := Scaffold(ScaffoldConfig{Dir: directory, Kind: "feature", Name: "SafeName"})
	if err == nil || !strings.Contains(err.Error(), "unsupported scaffold kind") {
		t.Fatalf("kind error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "app")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unsupported kind wrote files: %v", statErr)
	}
}
