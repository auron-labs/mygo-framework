// Package bbolt provides an optional owning settings store backed by bbolt.
package bbolt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mygo-framework/settings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
)

const openTimeout = time.Second

var settingsBucket = []byte("mygo_framework_settings")

// Store owns one bbolt database and its single settings bucket.
type Store struct {
	database *bolt.DB

	closeOnce sync.Once
	closeErr  error
}

var _ settings.Store = (*Store)(nil)

// Open opens the settings database with a bounded process-lock wait.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("bbolt settings path is required")
	}

	database, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		if errors.Is(err, berrors.ErrTimeout) {
			return nil, fmt.Errorf(
				"open bbolt settings database %q: database is busy after %s: %w",
				path,
				openTimeout,
				err,
			)
		}

		return nil, fmt.Errorf("open bbolt settings database %q: %w", path, err)
	}

	store := &Store{database: database}
	if err := database.Update(func(transaction *bolt.Tx) error {
		if _, err := transaction.CreateBucketIfNotExists(settingsBucket); err != nil {
			return fmt.Errorf("create settings bucket: %w", err)
		}
		return nil
	}); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("initialize bbolt settings database %q: %w", path, err)
	}

	return store, nil
}

// Read copies a value before the read transaction ends.
func (s *Store) Read(ctx context.Context, name string) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("read bbolt setting %q: %w", name, err)
	}

	var value json.RawMessage
	found := false
	err := s.database.View(func(transaction *bolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check read context: %w", err)
		}
		bucket := transaction.Bucket(settingsBucket)
		if bucket == nil {
			return errors.New("settings bucket is missing")
		}
		stored := bucket.Get([]byte(name))
		if stored != nil {
			value = append(json.RawMessage(nil), stored...)
			found = true
		}
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("read bbolt setting %q: %w", name, err)
	}

	return value, found, nil
}

// Write commits one JSON value before returning.
func (s *Store) Write(ctx context.Context, name string, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write bbolt setting %q: %w", name, err)
	}
	if !json.Valid(value) {
		return fmt.Errorf("write bbolt setting %q: value is not valid JSON", name)
	}

	if err := s.database.Update(func(transaction *bolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check write context: %w", err)
		}
		bucket := transaction.Bucket(settingsBucket)
		if bucket == nil {
			return errors.New("settings bucket is missing")
		}
		if err := bucket.Put([]byte(name), value); err != nil {
			return fmt.Errorf("put value: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("write bbolt setting %q: %w", name, err)
	}

	return nil
}

// Delete commits removal of one value. A missing value is already deleted.
func (s *Store) Delete(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete bbolt setting %q: %w", name, err)
	}

	if err := s.database.Update(func(transaction *bolt.Tx) error {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("check delete context: %w", err)
		}
		bucket := transaction.Bucket(settingsBucket)
		if bucket == nil {
			return errors.New("settings bucket is missing")
		}
		if err := bucket.Delete([]byte(name)); err != nil {
			return fmt.Errorf("delete value: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("delete bbolt setting %q: %w", name, err)
	}

	return nil
}

// Close releases the owned database once.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		if err := s.database.Close(); err != nil {
			s.closeErr = fmt.Errorf("close bbolt settings database: %w", err)
		}
	})

	return s.closeErr
}
