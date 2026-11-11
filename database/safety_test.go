package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// migrationsUpTo returns migrations that each create one table.
func migrationsUpTo(count int) fstest.MapFS {
	files := fstest.MapFS{}
	for version := 1; version <= count; version++ {
		files[fmt.Sprintf("%05d_table%d.sql", version, version)] = &fstest.MapFile{
			Data: fmt.Appendf(nil, "-- +goose Up\nCREATE TABLE table%d (value TEXT);\n", version),
		}
	}
	return files
}

func openAt(t *testing.T, path string, count int) *Database {
	t.Helper()
	db, err := Open(t.Context(), path, migrationsUpTo(count))
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func snapshots(t *testing.T, path string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(filepath.Dir(path), backupDirectory, filepath.Base(path)+".*.db"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func tableCount(t *testing.T, path string) int {
	t.Helper()
	db, err := openSQL(t.Context(), path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var count int
	if err := db.QueryRowContext(t.Context(),
		"SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name LIKE 'table%'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMigrationSnapshotsHoldTheSchemaBeforeTheMigration(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "app.db")

	if err := openAt(t, path, 1).Close(); err != nil {
		t.Fatal(err)
	}
	if got := snapshots(t, path); len(got) != 0 {
		t.Fatalf("a new database was snapshotted: %v", got)
	}
	if err := openAt(t, path, 1).Close(); err != nil {
		t.Fatal(err)
	}
	if got := snapshots(t, path); len(got) != 0 {
		t.Fatalf("an up-to-date database was snapshotted: %v", got)
	}

	if err := openAt(t, path, 2).Close(); err != nil {
		t.Fatal(err)
	}
	got := snapshots(t, path)
	if len(got) != 1 {
		t.Fatalf("snapshots = %v, want one", got)
	}
	if before, after := tableCount(t, got[0]), tableCount(t, path); before != 1 || after != 2 {
		t.Fatalf("tables before = %d, after = %d, want 1 and 2", before, after)
	}
}

func TestMigrationSnapshotsKeepTheNewest(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "app.db")
	if err := openAt(t, path, 1).Close(); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(filepath.Dir(path), backupDirectory)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	for day := 1; day <= keptSnapshots+2; day++ {
		old := filepath.Join(directory, fmt.Sprintf("app.db.2020010%dT000000Z.v1.db", day))
		if err := os.WriteFile(old, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	other := filepath.Join(directory, "other.db.20200101T000000Z.v1.db")
	if err := os.WriteFile(other, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := openAt(t, path, 2).Close(); err != nil {
		t.Fatal(err)
	}

	got := snapshots(t, path)
	if len(got) != keptSnapshots {
		t.Fatalf("kept %d snapshots, want %d: %v", len(got), keptSnapshots, got)
	}
	if tableCount(t, got[len(got)-1]) != 1 {
		t.Fatalf("the newest snapshot %q is not the one just taken", got[len(got)-1])
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("another database's backup was removed: %v", err)
	}
}

func TestOpenRefusesADatabaseFromANewerVersion(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "app.db")
	if err := openAt(t, path, 3).Close(); err != nil {
		t.Fatal(err)
	}

	_, err := Open(t.Context(), path, migrationsUpTo(2))
	if !errors.Is(err, ErrSchemaNewer) {
		t.Fatalf("Open error = %v, want ErrSchemaNewer", err)
	}
	if got := tableCount(t, path); got != 3 {
		t.Fatalf("tables = %d, want the database left alone", got)
	}
}

func TestBackupAndRestoreRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	directory := t.TempDir()
	path := filepath.Join(directory, "app.db")
	backup := filepath.Join(directory, "export.db")

	db := openAt(t, path, 2)
	if _, err := db.SQL.ExecContext(ctx, "INSERT INTO table1 (value) VALUES ('kept')"); err != nil {
		t.Fatal(err)
	}
	// The application stays open while its data is exported.
	if err := Backup(ctx, path, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.ExecContext(ctx, "INSERT INTO table1 (value) VALUES ('later')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Restore(ctx, backup, path); err != nil {
		t.Fatal(err)
	}
	restored := openAt(t, path, 2)
	defer func() { _ = restored.Close() }()
	var values int
	if err := restored.SQL.QueryRowContext(ctx, "SELECT count(*) FROM table1").Scan(&values); err != nil {
		t.Fatal(err)
	}
	if values != 1 {
		t.Fatalf("restored rows = %d, want 1", values)
	}
	if got := snapshots(t, path); len(got) != 1 {
		t.Fatalf("pre-restore snapshots = %v, want one", got)
	}
}

func TestRestoreRejectsAFileThatIsNotADatabase(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "app.db")
	if err := openAt(t, path, 1).Close(); err != nil {
		t.Fatal(err)
	}
	notDatabase := filepath.Join(directory, "notes.txt")
	if err := os.WriteFile(notDatabase, []byte("not a SQLite database, just some longer text to fill a page"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Restore(t.Context(), notDatabase, path); err == nil {
		t.Fatal("restored a file that is not a database")
	}
	if got := tableCount(t, path); got != 1 {
		t.Fatalf("tables = %d, want the database left alone", got)
	}
	if got := snapshots(t, path); len(got) != 0 {
		t.Fatalf("snapshots = %v, want none for a rejected restore", got)
	}
}
