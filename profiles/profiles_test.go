package profiles_test

import (
	"mygo-framework/database/migrations"
	"mygo-framework/profiles"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsNamesThatCouldEscapeTheDataDirectory(t *testing.T) {
	for _, name := range []string{
		"", "..", ".", "../x", "a/b", `a\b`, "/abs", "C:", "a.b", " lead", "trail ", "-lead", "nul", "COM1",
		"a\x00b", "café", strings.Repeat("a", 65),
	} {
		if err := profiles.Validate(name); err == nil {
			t.Errorf("Validate(%q) accepted the name", name)
		}
	}
	for _, name := range []string{"Default", "Work", "my profile", "client_2-b", "a", strings.Repeat("a", 64)} {
		if err := profiles.Validate(name); err != nil {
			t.Errorf("Validate(%q) = %v", name, err)
		}
	}
}

func TestOpenKeepsProfilesInSeparateFilesUnderTheDataDirectory(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()

	work, err := profiles.Open(ctx, dir, "Work", migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.SQL.ExecContext(ctx, "INSERT INTO mygo_framework_settings (name, value_json) VALUES ('native.compact', 'true')"); err != nil {
		t.Fatal(err)
	}
	if err := work.Close(); err != nil {
		t.Fatal(err)
	}
	home, err := profiles.Open(ctx, dir, "home", migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = home.Close() })
	var rows int
	if err := home.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM mygo_framework_settings").Scan(&rows); err != nil || rows != 0 {
		t.Fatalf("another profile sees %d rows, %v", rows, err)
	}

	// Case does not select a different file.
	again, err := profiles.Open(ctx, dir, "WORK", migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = again.Close() })
	if err := again.SQL.QueryRowContext(ctx, "SELECT COUNT(*) FROM mygo_framework_settings").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("WORK sees %d rows, %v", rows, err)
	}

	for _, want := range []string{"work.db", "home.db"} {
		if _, err := os.Stat(filepath.Join(dir, "profiles", want)); err != nil {
			t.Error(err)
		}
	}
	if _, err := profiles.Open(ctx, dir, "../escape", migrations.FS()); err == nil {
		t.Fatal("a traversal name was opened")
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.db")); err == nil {
		t.Fatal("a database was created outside the profiles directory")
	}
}
