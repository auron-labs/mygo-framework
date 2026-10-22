// Package database owns the application's shared SQLite connection pool.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/gormlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
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

	sqlDB, err := driver.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database %q: %w", path, err)
	}

	database := &Database{SQL: sqlDB}
	defer func() {
		if err != nil {
			_ = database.Close()
		}
	}()

	if err = sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping SQLite database %q: %w", path, err)
	}

	database.GORM, err = gorm.Open(gormlite.OpenDB(sqlDB), &gorm.Config{
		DisableAutomaticPing: true,
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
		return nil, fmt.Errorf("apply SQLite migrations to %q: %w", path, err)
	}

	return database, nil
}

// Close releases the shared pool once.
func (d *Database) Close() error {
	d.closeOnce.Do(func() {
		d.closeErr = d.SQL.Close()
	})

	return d.closeErr
}
