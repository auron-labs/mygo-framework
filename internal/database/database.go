// Package database owns the application's shared SQLite connection pool.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/gormlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	busyTimeoutMilliseconds = 5000
	maxOpenConnections      = 1
)

// Database owns one sql.DB shared by GORM and Goose.
type Database struct {
	SQL  *sql.DB
	GORM *gorm.DB

	closeOnce sync.Once
	closeErr  error
}

// MigrationStatus describes one migration without changing database state.
type MigrationStatus struct {
	Version int64
	Path    string
	Applied bool
}

// Open opens the shared pool, applies migrations, and constructs GORM over it.
func Open(ctx context.Context, path string, migrations fs.FS) (_ *Database, err error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if migrations == nil {
		return nil, errors.New("migration filesystem is required")
	}

	sqlDB, err := openSQL(ctx, path, false)
	if err != nil {
		return nil, err
	}

	database := &Database{SQL: sqlDB}
	defer func() {
		if err != nil {
			_ = database.Close()
		}
	}()

	database.GORM, err = gorm.Open(gormlite.OpenDB(sqlDB), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("open GORM over SQLite database %q: %w", path, err)
	}

	if _, err = up(ctx, sqlDB, path, migrations); err != nil {
		return nil, err
	}

	return database, nil
}

// InspectMigrations reports applied and pending migrations without creating a
// database, a Goose version table, or applying migrations.
func InspectMigrations(ctx context.Context, path string, migrations fs.FS) ([]MigrationStatus, bool, error) {
	if path == "" {
		return nil, false, errors.New("database path is required")
	}
	if migrations == nil {
		return nil, false, errors.New("migration filesystem is required")
	}
	exists := true
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		exists = false
	} else if err != nil {
		return nil, false, fmt.Errorf("inspect SQLite database %q: %w", path, err)
	}

	var sqlDB *sql.DB
	var err error
	if exists {
		sqlDB, err = openSQL(ctx, path, true)
	} else {
		// Goose requires a pool while it discovers sources; no connection is made.
		sqlDB, err = driver.Open(":memory:")
	}
	if err != nil {
		return nil, exists, fmt.Errorf("open migration status database for %q: %w", path, err)
	}
	defer func() { _ = sqlDB.Close() }()

	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, migrations)
	if err != nil {
		return nil, exists, fmt.Errorf("read migrations for %q: %w", path, err)
	}
	sources := provider.ListSources()
	statuses := make([]MigrationStatus, len(sources))
	for index, source := range sources {
		statuses[index] = MigrationStatus{Version: source.Version, Path: source.Path}
	}
	if !exists {
		return statuses, false, nil
	}

	var versionTableExists bool
	if err := sqlDB.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = 'goose_db_version')",
	).Scan(&versionTableExists); err != nil {
		return nil, true, fmt.Errorf("inspect migration table in %q: %w", path, err)
	}
	if !versionTableExists {
		return statuses, true, nil
	}

	for index := range statuses {
		var applied bool
		err := sqlDB.QueryRowContext(ctx,
			"SELECT is_applied FROM goose_db_version WHERE version_id = ? ORDER BY id DESC LIMIT 1",
			statuses[index].Version,
		).Scan(&applied)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, true, fmt.Errorf("inspect migration %05d in %q: %w", statuses[index].Version, path, err)
		}
		statuses[index].Applied = applied
	}

	return statuses, true, nil
}

// Migrate applies all pending migrations without constructing application services.
func Migrate(ctx context.Context, path string, migrations fs.FS) (_ int, err error) {
	if path == "" {
		return 0, errors.New("database path is required")
	}
	if migrations == nil {
		return 0, errors.New("migration filesystem is required")
	}

	sqlDB, err := openSQL(ctx, path, false)
	if err != nil {
		return 0, err
	}
	defer func() {
		err = errors.Join(err, sqlDB.Close())
	}()

	return up(ctx, sqlDB, path, migrations)
}

func openSQL(ctx context.Context, path string, readOnly bool) (*sql.DB, error) {
	sqlDB, err := driver.Open(dataSourceName(path, readOnly))
	if err != nil {
		return nil, fmt.Errorf("open SQLite database %q: %w", path, err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConnections)
	sqlDB.SetMaxIdleConns(maxOpenConnections)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping SQLite database %q: %w", path, err)
	}
	if readOnly {
		return sqlDB, nil
	}

	var journalMode string
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("enable SQLite WAL for %q: %w", path, err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("enable SQLite WAL for %q: got journal mode %q", path, journalMode)
	}

	return sqlDB, nil
}

func up(ctx context.Context, sqlDB *sql.DB, path string, migrations fs.FS) (int, error) {
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, migrations)
	if err != nil {
		return 0, fmt.Errorf("create migration provider for %q: %w", path, err)
	}
	// Provider.Close would close the borrowed pool; its caller owns the pool.
	results, err := provider.Up(ctx)
	if err == nil {
		return len(results), nil
	}

	var partial *goose.PartialError
	if errors.As(err, &partial) && partial.Failed != nil && partial.Failed.Source != nil {
		return 0, fmt.Errorf(
			"apply SQLite migration %q to %q: %w",
			filepath.Base(partial.Failed.Source.Path),
			path,
			err,
		)
	}

	return 0, fmt.Errorf("apply SQLite migrations to %q: %w", path, err)
}

func dataSourceName(path string, readOnly bool) string {
	source := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := url.Values{
		"_pragma": {
			fmt.Sprintf("busy_timeout(%d)", busyTimeoutMilliseconds),
			"foreign_keys(1)",
		},
	}
	if readOnly {
		query.Set("mode", "ro")
	}
	source.RawQuery = query.Encode()

	return source.String()
}

// Close releases the shared pool once.
func (d *Database) Close() error {
	d.closeOnce.Do(func() {
		d.closeErr = d.SQL.Close()
	})

	return d.closeErr
}
