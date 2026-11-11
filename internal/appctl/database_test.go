package appctl

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auron-labs/mygo-framework/database/migrations"
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

func TestDatabaseBackupAndRestore(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "portable data")
	backup := filepath.Join(t.TempDir(), "export.db")
	execute(t, "--data-directory", directory, "db", "up")

	output := execute(t, "--data-directory", directory, "db", "backup", backup)
	if !strings.Contains(output, "backup: "+backup) {
		t.Fatalf("backup output = %q", output)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatal(err)
	}

	output = execute(t, "--data-directory", directory, "db", "restore", backup)
	if !strings.Contains(output, "restored: ") {
		t.Fatalf("restore output = %q", output)
	}
	if got, _ := filepath.Glob(filepath.Join(directory, "backups", "*.db")); len(got) != 1 {
		t.Fatalf("restore kept snapshots %v, want one", got)
	}
}

func TestSupportBundleIsWrittenOnceAndNeverOverwritten(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "portable data")
	bundle := filepath.Join(t.TempDir(), "support.zip")
	output := execute(t, "--data-directory", directory, "support-bundle", bundle)
	if !strings.Contains(output, "support bundle: "+bundle) {
		t.Fatalf("output = %q", output)
	}
	archive, err := zip.OpenReader(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()
	if len(archive.File) == 0 || archive.File[0].Name != "info.txt" {
		t.Fatalf("bundle entries = %v", archive.File)
	}

	command := New("test", migrations.FS())
	command.SetArgs([]string{"--data-directory", directory, "support-bundle", bundle})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	if err := command.ExecuteContext(t.Context()); err == nil {
		t.Fatal("an existing file was overwritten")
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
