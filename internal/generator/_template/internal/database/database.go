// Package database owns the application's shared SQLite connection pool.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
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

// Open opens the shared pool, applies migrations, and constructs GORM over it.
func Open(ctx context.Context, path string, migrations fs.FS) (_ *Database, err error) {
	if path == "" {
		return nil, errors.New("database path is required")
	}
	if migrations == nil {
		return nil, errors.New("migration filesystem is required")
	}

	sqlDB, err := driver.Open(dataSourceName(path))
	if err != nil {
		return nil, fmt.Errorf("open SQLite database %q: %w", path, err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConnections)
	sqlDB.SetMaxIdleConns(maxOpenConnections)

	database := &Database{SQL: sqlDB}
	defer func() {
		if err != nil {
			_ = database.Close()
		}
	}()

	if err = sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping SQLite database %q: %w", path, err)
	}

	var journalMode string
	if err = sqlDB.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		return nil, fmt.Errorf("enable SQLite WAL for %q: %w", path, err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return nil, fmt.Errorf("enable SQLite WAL for %q: got journal mode %q", path, journalMode)
	}

	database.GORM, err = gorm.Open(gormlite.OpenDB(sqlDB), &gorm.Config{
		DisableAutomaticPing: true,
		Logger:               logger.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("open GORM over SQLite database %q: %w", path, err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, migrations)
	if err != nil {
		return nil, fmt.Errorf("create migration provider for %q: %w", path, err)
	}
	// Provider.Close would close the borrowed pool; Database is its sole owner.
	if _, err = provider.Up(ctx); err != nil {
		var partial *goose.PartialError
		if errors.As(err, &partial) && partial.Failed != nil && partial.Failed.Source != nil {
			return nil, fmt.Errorf(
				"apply SQLite migration %q to %q: %w",
				filepath.Base(partial.Failed.Source.Path),
				path,
				err,
			)
		}

		return nil, fmt.Errorf("apply SQLite migrations to %q: %w", path, err)
	}

	return database, nil
}

func dataSourceName(path string) string {
	source := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	source.RawQuery = url.Values{
		"_pragma": {
			fmt.Sprintf("busy_timeout(%d)", busyTimeoutMilliseconds),
			"foreign_keys(1)",
		},
	}.Encode()

	return source.String()
}

// Close releases the shared pool once.
func (d *Database) Close() error {
	d.closeOnce.Do(func() {
		d.closeErr = d.SQL.Close()
	})

	return d.closeErr
}
