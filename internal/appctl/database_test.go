package appctl

import (
	"bytes"
	"mygo-framework/database/migrations"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseStatusDoesNotCreateDatabase(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "portable data")
	output := execute(t, "--data-directory", directory, "db", "status")
	if !strings.Contains(output, "(not created)") || !strings.Contains(output, "00001  pending") {
		t.Fatalf("status output = %q, want missing database and pending migrations", output)
	}
	if _, err := os.Stat(filepath.Join(directory, "notes.db")); !os.IsNotExist(err) {
		t.Fatalf("status created database: %v", err)
	}
}

func TestDatabaseUpAppliesOnlyPendingMigrations(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "portable data")
	output := execute(t, "--data-directory", directory, "db", "up")
	if !strings.Contains(output, "applied: 2") {
		t.Fatalf("first up output = %q, want two applied migrations", output)
	}

	output = execute(t, "--data-directory", directory, "db", "status")
	if strings.Count(output, "applied") != 2 || strings.Contains(output, "pending") {
		t.Fatalf("status output = %q, want two applied migrations", output)
	}

	output = execute(t, "--data-directory", directory, "db", "up")
	if !strings.Contains(output, "applied: 0") {
		t.Fatalf("second up output = %q, want no applied migrations", output)
	}
}

func execute(t *testing.T, arguments ...string) string {
	t.Helper()

	var output bytes.Buffer
	command := New("test", migrations.FS())
	command.SetArgs(arguments)
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("execute %q: %v\n%s", arguments, err, output.String())
	}
	return output.String()
}
