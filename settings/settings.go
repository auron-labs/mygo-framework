// Package settings provides typed access to JSON-backed application settings.
package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	// ErrUnknownKey means a key was not registered with the Values instance.
	ErrUnknownKey = errors.New("unknown setting key")
	// ErrInvalidValue means a default, stored value, or proposed value is invalid.
	ErrInvalidValue = errors.New("invalid setting value")
)

// Store persists opaque JSON values. Implementations must be safe for concurrent
// use, honor request cancellation, return caller-owned read bytes, and not retain
// write bytes after Write returns.
type Store interface {
	Read(context.Context, string) (json.RawMessage, bool, error)
	Write(context.Context, string, json.RawMessage) error
	Delete(context.Context, string) error
}

// Definition permits heterogeneous Key values to be registered together.
// Only Key values implement Definition.
type Definition interface {
	settingDefinition() keyDefinition
}

// Key describes one typed setting and its default and optional validation.
// Validation functions must be safe for concurrent calls.
type Key[T any] struct {
	spec *keySpec[T]
}

type keySpec[T any] struct {
	name         string
	defaultValue T
	validate     func(T) error
}

type keyDefinition struct {
	valid        bool
	name         string
	token        any
	valueType    reflect.Type
	defaultValue any
	checkJSON    func(json.RawMessage) error
}

// NewKey defines a setting. Register the returned Key with New before use.
func NewKey[T any](name string, defaultValue T, validate func(T) error) Key[T] {
	return Key[T]{spec: &keySpec[T]{
		name:         name,
		defaultValue: defaultValue,
		validate:     validate,
	}}
}

// Name returns the storage name of the key.
func (k Key[T]) Name() string {
	if k.spec == nil {
		return ""
	}

	return k.spec.name
}

func (k Key[T]) settingDefinition() keyDefinition {
	if k.spec == nil {
		return keyDefinition{}
	}

	return keyDefinition{
		valid:        true,
		name:         k.spec.name,
		token:        k.spec,
		valueType:    reflect.TypeFor[T](),
		defaultValue: k.spec.defaultValue,
		checkJSON: func(raw json.RawMessage) error {
			value, err := decodeJSON[T](raw)
			if err != nil {
				return err
			}
			return k.validate(value)
		},
	}
}

func (k Key[T]) validate(value T) error {
	if k.spec.validate == nil {
		return nil
	}

	return k.spec.validate(value)
}

type registration struct {
	token       any
	valueType   reflect.Type
	defaultJSON json.RawMessage
}

// Values provides typed access to a Store without caching mutable values.
type Values struct {
	store         Store
	registrations map[string]registration
}

// New registers keys and binds them to store. Defaults must be valid JSON values
// that satisfy their key validation.
func New(store Store, definitions ...Definition) (*Values, error) {
	if store == nil {
		return nil, errors.New("settings: nil store")
	}

	registrations := make(map[string]registration, len(definitions))
	for _, definition := range definitions {
		if definition == nil {
			return nil, fmt.Errorf("%w: nil definition", ErrUnknownKey)
		}

		key := definition.settingDefinition()
		if !key.valid || strings.TrimSpace(key.name) == "" {
			return nil, fmt.Errorf("%w: key name is empty", ErrUnknownKey)
		}
		if _, exists := registrations[key.name]; exists {
			return nil, fmt.Errorf("settings: duplicate key %q", key.name)
		}

		defaultJSON, err := json.Marshal(key.defaultValue)
		if err != nil {
			return nil, fmt.Errorf("%w: encode default for setting %q: %w", ErrInvalidValue, key.name, err)
		}
		if err := key.checkJSON(defaultJSON); err != nil {
			return nil, fmt.Errorf("%w: validate default for setting %q: %w", ErrInvalidValue, key.name, err)
		}

		registrations[key.name] = registration{
			token:       key.token,
			valueType:   key.valueType,
			defaultJSON: defaultJSON,
		}
	}

	return &Values{store: store, registrations: registrations}, nil
}

// Get reads and validates a setting, returning a fresh copy of its default when
// the key is missing.
func Get[T any](ctx context.Context, values *Values, key Key[T]) (T, error) {
	var zero T

	registered, err := registeredKey(values, key.settingDefinition())
	if err != nil {
		return zero, err
	}

	raw, found, err := values.store.Read(ctx, key.Name())
	if err != nil {
		return zero, fmt.Errorf("read setting %q: %w", key.Name(), err)
	}
	if !found {
		raw = registered.defaultJSON
	}

	value, err := decodeJSON[T](raw)
	if err != nil {
		return zero, fmt.Errorf("%w: decode setting %q as %s: %w", ErrInvalidValue, key.Name(), registered.valueType, err)
	}
	if err := key.validate(value); err != nil {
		return zero, fmt.Errorf("%w: validate setting %q: %w", ErrInvalidValue, key.Name(), err)
	}

	return value, nil
}

// Set validates and persists a setting before returning.
func Set[T any](ctx context.Context, values *Values, key Key[T], value T) error {
	if _, err := registeredKey(values, key.settingDefinition()); err != nil {
		return err
	}
	if err := key.validate(value); err != nil {
		return fmt.Errorf("%w: validate setting %q: %w", ErrInvalidValue, key.Name(), err)
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("%w: encode setting %q: %w", ErrInvalidValue, key.Name(), err)
	}
	if err := values.store.Write(ctx, key.Name(), raw); err != nil {
		return fmt.Errorf("write setting %q: %w", key.Name(), err)
	}

	return nil
}

// Delete removes a persisted value so the next Get returns the key default.
func Delete[T any](ctx context.Context, values *Values, key Key[T]) error {
	if _, err := registeredKey(values, key.settingDefinition()); err != nil {
		return err
	}
	if err := values.store.Delete(ctx, key.Name()); err != nil {
		return fmt.Errorf("delete setting %q: %w", key.Name(), err)
	}

	return nil
}

func registeredKey(values *Values, key keyDefinition) (registration, error) {
	if values == nil {
		return registration{}, errors.New("settings: nil values")
	}
	if !key.valid {
		return registration{}, fmt.Errorf("%w: invalid key", ErrUnknownKey)
	}

	registered, exists := values.registrations[key.name]
	if !exists {
		return registration{}, fmt.Errorf("%w %q", ErrUnknownKey, key.name)
	}
	if registered.token != key.token {
		return registration{}, fmt.Errorf(
			"%w %q: registered as %s, requested as %s",
			ErrUnknownKey,
			key.name,
			registered.valueType,
			key.valueType,
		)
	}

	return registered, nil
}

func decodeJSON[T any](raw json.RawMessage) (T, error) {
	var value T
	valueType := reflect.TypeFor[T]()
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) && rejectsNull(valueType) {
		return value, &json.UnmarshalTypeError{Value: "null", Type: valueType}
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, fmt.Errorf("unmarshal JSON: %w", err)
	}

	return value, nil
}

func rejectsNull(valueType reflect.Type) bool {
	if valueType == nil {
		return false
	}
	if reflect.PointerTo(valueType).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		return false
	}

	kind := valueType.Kind()
	return kind != reflect.Interface && kind != reflect.Map && kind != reflect.Pointer && kind != reflect.Slice
}
