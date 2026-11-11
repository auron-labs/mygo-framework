package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
)

const (
	// backupDirectory holds the snapshots taken before migrations, next to the
	// database file.
	backupDirectory = "backups"
	// keptSnapshots is how many pre-migration snapshots of one database remain.
	keptSnapshots = 5
)

// ErrSchemaNewer reports a database that a newer version of the application
// has migrated. Opening it with this version would run against tables it does
// not know.
var ErrSchemaNewer = errors.New("database was created by a newer version of the application")

// Backup writes a consistent copy of the database at path to destination. The
// database may be open in the running application. Destination must not exist.
func Backup(ctx context.Context, path, destination string) (err error) {
	sqlDB, err := openSQL(ctx, path, true)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, sqlDB.Close()) }()
	return vacuumInto(ctx, sqlDB, destination)
}

// Restore replaces the database at path with the backup at source. The
// application must not be running. A snapshot of the database being replaced
// is kept in its backups directory. A source that is not an intact SQLite
// database leaves the existing database untouched.
func Restore(ctx context.Context, source, path string) (err error) {
	check, err := openSQL(ctx, source, true)
	if err != nil {
		return err
	}
	var result string
	err = check.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result)
	err = errors.Join(err, check.Close())
	if err != nil {
		return fmt.Errorf("check backup %q: %w", source, err)
	}
	if result != "ok" {
		return fmt.Errorf("check backup %q: integrity check reported %q", source, result)
	}

	if _, statErr := os.Stat(path); statErr == nil {
		if err := snapshotFile(ctx, path, "pre-restore"); err != nil {
			return err
		}
	}
	// Copy beside the destination so the final rename stays on one volume.
	copyPath := path + ".restoring"
	if err := copyFile(source, copyPath); err != nil {
		return err
	}
	// A leftover write-ahead log would be replayed over the restored file.
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.Join(fmt.Errorf("remove stale %q: %w", path+suffix, err), os.Remove(copyPath))
		}
	}
	if err := os.Rename(copyPath, path); err != nil {
		return errors.Join(fmt.Errorf("replace database %q: %w", path, err), os.Remove(copyPath))
	}
	return nil
}

// guardMigrations refuses a database ahead of the application and snapshots one
// that is about to be migrated, so a failed or unwanted migration can be undone.
func guardMigrations(ctx context.Context, sqlDB *sql.DB, provider *goose.Provider, path string) error {
	current, err := provider.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema version of %q: %w", path, err)
	}
	sources := provider.ListSources()
	if len(sources) == 0 {
		return nil
	}
	latest := sources[len(sources)-1].Version
	if current > latest {
		return fmt.Errorf("%w: %q is at schema version %d and this version knows up to %d", ErrSchemaNewer, path, current, latest)
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check pending migrations of %q: %w", path, err)
	}
	if !pending || current == 0 {
		return nil
	}
	destination, err := snapshotPath(path, fmt.Sprintf("v%d", current))
	if err != nil {
		return err
	}
	if err := vacuumInto(ctx, sqlDB, destination); err != nil {
		return fmt.Errorf("back up %q before migrating: %w", path, err)
	}
	return pruneSnapshots(path)
}

func snapshotFile(ctx context.Context, path, label string) (err error) {
	sqlDB, err := openSQL(ctx, path, true)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, sqlDB.Close()) }()
	destination, err := snapshotPath(path, label)
	if err != nil {
		return err
	}
	if err := vacuumInto(ctx, sqlDB, destination); err != nil {
		return fmt.Errorf("back up %q before replacing it: %w", path, err)
	}
	return pruneSnapshots(path)
}

// snapshotPath names a snapshot so that names sort oldest first.
func snapshotPath(path, label string) (string, error) {
	directory := filepath.Join(filepath.Dir(path), backupDirectory)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create backup directory %q: %w", directory, err)
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	return filepath.Join(directory, fmt.Sprintf("%s.%s.%s.db", filepath.Base(path), stamp, label)), nil
}

func pruneSnapshots(path string) error {
	directory := filepath.Join(filepath.Dir(path), backupDirectory)
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("list backups in %q: %w", directory, err)
	}
	prefix := filepath.Base(path) + "."
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".db") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for len(names) > keptSnapshots {
		if err := os.Remove(filepath.Join(directory, names[0])); err != nil {
			return fmt.Errorf("remove old backup %q: %w", names[0], err)
		}
		names = names[1:]
	}
	return nil
}

func vacuumInto(ctx context.Context, sqlDB *sql.DB, destination string) error {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve backup path %q: %w", destination, err)
	}
	if _, err := sqlDB.ExecContext(ctx, "VACUUM INTO ?", absolute); err != nil {
		return fmt.Errorf("write backup %q: %w", destination, err)
	}
	return nil
}

func copyFile(source, destination string) (err error) {
	in, err := os.Open(source) // #nosec G304 -- the caller chooses which backup to restore.
	if err != nil {
		return fmt.Errorf("open backup %q: %w", source, err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- derived from the database path.
	if err != nil {
		return fmt.Errorf("create %q: %w", destination, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		return errors.Join(fmt.Errorf("copy backup to %q: %w", destination, err), out.Close(), os.Remove(destination))
	}
	if err := out.Close(); err != nil {
		return errors.Join(fmt.Errorf("write %q: %w", destination, err), os.Remove(destination))
	}
	return nil
}
