// Package sqlite persists settings in the application's shared SQLite pool.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mygo-framework/settings"
)

const (
	readSetting  = `SELECT value_json FROM mygo_framework_settings WHERE name = ?`
	writeSetting = `INSERT INTO mygo_framework_settings (name, value_json)
		VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET value_json = excluded.value_json`
	deleteSetting = `DELETE FROM mygo_framework_settings WHERE name = ?`
)

// Store borrows the application sql.DB; its owner remains responsible for
// migration and closure.
type Store struct {
	database *sql.DB
}

var _ settings.Store = (*Store)(nil)

// New constructs a store over the migrated shared pool without opening it.
func New(database *sql.DB) (*Store, error) {
	if database == nil {
		return nil, errors.New("SQLite settings store requires a database")
	}

	return &Store{database: database}, nil
}

// Read returns the persisted JSON value, if present.
func (s *Store) Read(ctx context.Context, name string) (json.RawMessage, bool, error) {
	var value []byte
	err := s.database.QueryRowContext(ctx, readSetting, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read SQLite setting %q: %w", name, err)
	}

	return append(json.RawMessage(nil), value...), true, nil
}

// Write inserts or replaces one JSON value.
func (s *Store) Write(ctx context.Context, name string, value json.RawMessage) error {
	if !json.Valid(value) {
		return fmt.Errorf("write SQLite setting %q: value is not valid JSON", name)
	}
	if _, err := s.database.ExecContext(ctx, writeSetting, name, []byte(value)); err != nil {
		return fmt.Errorf("write SQLite setting %q: %w", name, err)
	}

	return nil
}

// Delete removes one value. A missing value is already deleted.
func (s *Store) Delete(ctx context.Context, name string) error {
	if _, err := s.database.ExecContext(ctx, deleteSetting, name); err != nil {
		return fmt.Errorf("delete SQLite setting %q: %w", name, err)
	}

	return nil
}
