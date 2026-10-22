// Package memory provides an in-memory SecretStore for tests.
package memory

import (
	"context"
	"fmt"
	"mygo-framework/secrets"
	"sync"
)

// Store keeps copied secret bytes in process memory.
type Store struct {
	mu     sync.RWMutex
	values map[string][]byte
}

var _ secrets.SecretStore = (*Store)(nil)

// New constructs an empty memory store.
func New() *Store {
	return &Store{values: make(map[string][]byte)}
}

// Get returns a caller-owned secret value.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get memory secret %q: %w", key, err)
	}
	if err := secrets.ValidateKey(key); err != nil {
		return nil, fmt.Errorf("get memory secret: %w", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	value, found := s.values[key]
	if !found {
		return nil, fmt.Errorf("get memory secret %q: %w", key, secrets.ErrNotFound)
	}

	return append([]byte(nil), value...), nil
}

// Set stores a copy of value, including an explicit empty value.
func (s *Store) Set(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("set memory secret %q: %w", key, err)
	}
	if err := secrets.ValidateKey(key); err != nil {
		return fmt.Errorf("set memory secret: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = append([]byte(nil), value...)
	return nil
}

// Delete removes a secret. A missing secret is already deleted.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete memory secret %q: %w", key, err)
	}
	if err := secrets.ValidateKey(key); err != nil {
		return fmt.Errorf("delete memory secret: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	return nil
}
