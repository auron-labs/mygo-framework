package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// Override is one explicitly mapped transient setting value.
type Override struct {
	name  string
	value json.RawMessage
}

// NewOverride validates and encodes a typed transient value.
func NewOverride[T any](key Key[T], value T) (Override, error) {
	raw, err := encodeValue(key, value)
	if err != nil {
		return Override{}, err
	}

	return Override{name: key.Name(), value: raw}, nil
}

// Overlay applies immutable flag and environment values over a persistent
// Store. Writes and deletes always pass through to the persistent store.
type Overlay struct {
	persistent  Store
	flags       map[string]json.RawMessage
	environment map[string]json.RawMessage
}

var _ Store = (*Overlay)(nil)

// NewOverlay creates flag > environment > persistence precedence. Only
// overrides explicitly provided by the caller are considered.
func NewOverlay(persistent Store, flags, environment []Override) (*Overlay, error) {
	if persistent == nil {
		return nil, errors.New("settings overlay requires a persistent store")
	}

	flagValues, err := indexOverrides("flag", flags)
	if err != nil {
		return nil, err
	}
	environmentValues, err := indexOverrides("environment", environment)
	if err != nil {
		return nil, err
	}

	return &Overlay{
		persistent:  persistent,
		flags:       flagValues,
		environment: environmentValues,
	}, nil
}

// Read resolves flags, then environment values, then persistence.
func (o *Overlay) Read(ctx context.Context, name string) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("read setting override %q: %w", name, err)
	}
	if value, found := o.flags[name]; found {
		return append(json.RawMessage(nil), value...), true, nil
	}
	if value, found := o.environment[name]; found {
		return append(json.RawMessage(nil), value...), true, nil
	}

	value, found, err := o.persistent.Read(ctx, name)
	if err != nil {
		return nil, false, fmt.Errorf("read persistent setting %q: %w", name, err)
	}

	return value, found, nil
}

// Write persists a value without changing transient overrides.
func (o *Overlay) Write(ctx context.Context, name string, value json.RawMessage) error {
	if err := o.persistent.Write(ctx, name, value); err != nil {
		return fmt.Errorf("write persistent setting %q: %w", name, err)
	}

	return nil
}

// Delete removes a persisted value without changing transient overrides.
func (o *Overlay) Delete(ctx context.Context, name string) error {
	if err := o.persistent.Delete(ctx, name); err != nil {
		return fmt.Errorf("delete persistent setting %q: %w", name, err)
	}

	return nil
}

func indexOverrides(source string, overrides []Override) (map[string]json.RawMessage, error) {
	indexed := make(map[string]json.RawMessage, len(overrides))
	for _, override := range overrides {
		if override.name == "" || !json.Valid(override.value) {
			return nil, fmt.Errorf("settings: invalid %s override", source)
		}
		if _, exists := indexed[override.name]; exists {
			return nil, fmt.Errorf("settings: duplicate %s override for %q", source, override.name)
		}
		indexed[override.name] = append(json.RawMessage(nil), override.value...)
	}

	return indexed, nil
}
