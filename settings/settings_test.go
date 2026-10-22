package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
)

type memoryStore struct {
	mu        sync.RWMutex
	values    map[string]json.RawMessage
	writes    int
	readErr   error
	writeErr  error
	deleteErr error
}

func newMemoryStore() *memoryStore {
	return &memoryStore{values: make(map[string]json.RawMessage)}
}

func (s *memoryStore) Read(ctx context.Context, name string) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("read memory setting: %w", err)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.readErr != nil {
		return nil, false, s.readErr
	}
	value, found := s.values[name]
	return append(json.RawMessage(nil), value...), found, nil
}

func (s *memoryStore) Write(ctx context.Context, name string, value json.RawMessage) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write memory setting: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.writeErr != nil {
		return s.writeErr
	}
	s.values[name] = append(json.RawMessage(nil), value...)
	s.writes++
	return nil
}

func (s *memoryStore) Delete(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete memory setting: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.values, name)
	return nil
}

func (s *memoryStore) put(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name] = json.RawMessage(value)
}

func TestGetReturnsFreshDefault(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	key := NewKey("window", map[string]int{"width": 960}, nil)
	values, err := New(store, key)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}

	first, err := Get(t.Context(), values, key)
	if err != nil {
		t.Fatalf("get first default: %v", err)
	}
	first["width"] = 1

	second, err := Get(t.Context(), values, key)
	if err != nil {
		t.Fatalf("get second default: %v", err)
	}
	if second["width"] != 960 {
		t.Fatalf("default width = %d, want 960", second["width"])
	}
}

func TestExplicitZeroValuesRemainStored(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	enabled := NewKey("enabled", true, nil)
	label := NewKey("label", "fallback", nil)
	retries := NewKey("retries", 3, nil)
	values, err := New(store, enabled, label, retries)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}

	if err := Set(t.Context(), values, enabled, false); err != nil {
		t.Fatalf("set false: %v", err)
	}
	if err := Set(t.Context(), values, label, ""); err != nil {
		t.Fatalf("set empty string: %v", err)
	}
	if err := Set(t.Context(), values, retries, 0); err != nil {
		t.Fatalf("set zero: %v", err)
	}

	gotEnabled, err := Get(t.Context(), values, enabled)
	if err != nil || gotEnabled {
		t.Fatalf("enabled = %v, error = %v, want false", gotEnabled, err)
	}
	gotLabel, err := Get(t.Context(), values, label)
	if err != nil || gotLabel != "" {
		t.Fatalf("label = %q, error = %v, want empty", gotLabel, err)
	}
	gotRetries, err := Get(t.Context(), values, retries)
	if err != nil || gotRetries != 0 {
		t.Fatalf("retries = %d, error = %v, want zero", gotRetries, err)
	}
}

func TestSetValidatesBeforeWritingAndGetValidatesStoredData(t *testing.T) {
	t.Parallel()

	nonNegative := func(value int) error {
		if value < 0 {
			return errors.New("must be non-negative")
		}
		return nil
	}
	store := newMemoryStore()
	key := NewKey("retries", 3, nonNegative)
	values, err := New(store, key)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}

	err = Set(t.Context(), values, key, -1)
	if !errors.Is(err, ErrInvalidValue) || !strings.Contains(err.Error(), `validate setting "retries"`) {
		t.Fatalf("set error = %v, want contextual invalid value", err)
	}
	if store.writes != 0 {
		t.Fatalf("writes = %d, want 0", store.writes)
	}

	store.put(key.Name(), "-2")
	_, err = Get(t.Context(), values, key)
	if !errors.Is(err, ErrInvalidValue) || !strings.Contains(err.Error(), "must be non-negative") {
		t.Fatalf("get error = %v, want stored-value validation error", err)
	}
}

func TestGetReportsMalformedWrongTypeAndNullValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr any
	}{
		{name: "malformed", raw: "[", wantErr: new(*json.SyntaxError)},
		{name: "wrong type", raw: `"many"`, wantErr: new(*json.UnmarshalTypeError)},
		{name: "null", raw: "null", wantErr: new(*json.UnmarshalTypeError)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := newMemoryStore()
			key := NewKey("retries", 3, nil)
			values, err := New(store, key)
			if err != nil {
				t.Fatalf("new settings: %v", err)
			}
			store.put(key.Name(), test.raw)

			_, err = Get(t.Context(), values, key)
			if !errors.Is(err, ErrInvalidValue) {
				t.Fatalf("error = %v, want ErrInvalidValue", err)
			}
			if !strings.Contains(err.Error(), `decode setting "retries" as int`) {
				t.Fatalf("error = %v, want contextual decode error", err)
			}
			switch target := test.wantErr.(type) {
			case **json.SyntaxError:
				if !errors.As(err, target) {
					t.Fatalf("error = %v, want *json.SyntaxError", err)
				}
			case **json.UnmarshalTypeError:
				if !errors.As(err, target) {
					t.Fatalf("error = %v, want *json.UnmarshalTypeError", err)
				}
			}
		})
	}
}

func TestUnknownAndIncompatibleKeysFail(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	registered := NewKey("retries", 3, nil)
	values, err := New(store, registered)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}

	unknown := NewKey("theme", "system", nil)
	if _, err := Get(t.Context(), values, unknown); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("unknown-key error = %v, want ErrUnknownKey", err)
	}

	incompatible := NewKey("retries", "three", nil)
	_, err = Get(t.Context(), values, incompatible)
	if !errors.Is(err, ErrUnknownKey) || !strings.Contains(err.Error(), "registered as int, requested as string") {
		t.Fatalf("incompatible-key error = %v", err)
	}
}

func TestDeleteRestoresDefault(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	key := NewKey("theme", "system", nil)
	values, err := New(store, key)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}
	if err := Set(t.Context(), values, key, "dark"); err != nil {
		t.Fatalf("set theme: %v", err)
	}
	if err := Delete(t.Context(), values, key); err != nil {
		t.Fatalf("delete theme: %v", err)
	}

	got, err := Get(t.Context(), values, key)
	if err != nil || got != "system" {
		t.Fatalf("theme = %q, error = %v, want default", got, err)
	}
}

func TestNewRejectsInvalidRegistrations(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	tests := []struct {
		name string
		keys []Definition
	}{
		{name: "nil definition", keys: []Definition{nil}},
		{name: "zero key", keys: []Definition{Key[int]{}}},
		{name: "empty name", keys: []Definition{NewKey(" ", 1, nil)}},
		{name: "duplicate name", keys: []Definition{NewKey("same", 1, nil), NewKey("same", "one", nil)}},
		{name: "unsupported default", keys: []Definition{NewKey("ratio", math.NaN(), nil)}},
		{name: "invalid default", keys: []Definition{NewKey("count", -1, func(value int) error {
			if value < 0 {
				return errors.New("must be non-negative")
			}
			return nil
		})}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(store, test.keys...); err == nil {
				t.Fatal("New accepted invalid registration")
			}
		})
	}

	if _, err := New(nil); err == nil {
		t.Fatal("New accepted a nil store")
	}
}

func TestStoreErrorsKeepOperationAndCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("store unavailable")
	key := NewKey("theme", "system", nil)
	store := newMemoryStore()
	values, err := New(store, key)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}

	store.readErr = cause
	_, err = Get(t.Context(), values, key)
	assertStoreError(t, err, cause, `read setting "theme"`)
	store.readErr = nil

	store.writeErr = cause
	err = Set(t.Context(), values, key, "dark")
	assertStoreError(t, err, cause, `write setting "theme"`)
	store.writeErr = nil

	store.deleteErr = cause
	err = Delete(t.Context(), values, key)
	assertStoreError(t, err, cause, `delete setting "theme"`)
}

func TestValuesAreSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	key := NewKey("count", 0, func(value int) error {
		if value < 0 {
			return fmt.Errorf("negative count %d", value)
		}
		return nil
	})
	values, err := New(store, key)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}

	const workers = 64
	errorsFound := make(chan error, workers*2)
	var wait sync.WaitGroup
	for worker := range workers {
		wait.Add(1)
		go func(value int) {
			defer wait.Done()
			if err := Set(t.Context(), values, key, value); err != nil {
				errorsFound <- err
			}
			if _, err := Get(t.Context(), values, key); err != nil {
				errorsFound <- err
			}
		}(worker)
	}
	wait.Wait()
	close(errorsFound)

	for err := range errorsFound {
		t.Errorf("concurrent settings operation: %v", err)
	}
}

func assertStoreError(t *testing.T, err, cause error, operation string) {
	t.Helper()
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), operation) {
		t.Fatalf("error = %v, want %q wrapping cause", err, operation)
	}
}
