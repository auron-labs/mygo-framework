package settings

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
)

func TestOverlayPrecedence(t *testing.T) {
	t.Parallel()

	persistent := newMemoryStore()
	key := NewKey("theme", "default", nil)
	persistent.put(key.Name(), `"persisted"`)
	flag := requireOverride(t, key, "flag")
	environment := requireOverride(t, key, "environment")

	tests := []struct {
		name        string
		flags       []Override
		environment []Override
		want        string
	}{
		{name: "flag", flags: []Override{flag}, environment: []Override{environment}, want: "flag"},
		{name: "environment", environment: []Override{environment}, want: "environment"},
		{name: "persistence", want: "persisted"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			overlay, err := NewOverlay(persistent, test.flags, test.environment)
			if err != nil {
				t.Fatalf("new overlay: %v", err)
			}
			values, err := New(overlay, key)
			if err != nil {
				t.Fatalf("new settings: %v", err)
			}
			got, err := Get(t.Context(), values, key)
			if err != nil || got != test.want {
				t.Fatalf("theme = %q, error = %v, want %q", got, err, test.want)
			}
		})
	}
}

func TestOverlayFallsBackToDefault(t *testing.T) {
	t.Parallel()

	key := NewKey("theme", "default", nil)
	overlay, err := NewOverlay(newMemoryStore(), nil, nil)
	if err != nil {
		t.Fatalf("new overlay: %v", err)
	}
	values, err := New(overlay, key)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}

	got, err := Get(t.Context(), values, key)
	if err != nil || got != "default" {
		t.Fatalf("theme = %q, error = %v, want default", got, err)
	}
}

func TestOverlayKeepsOverridesTransient(t *testing.T) {
	t.Parallel()

	persistent := newMemoryStore()
	key := NewKey("theme", "default", nil)
	flag := requireOverride(t, key, "flag")
	overlay, err := NewOverlay(persistent, []Override{flag}, nil)
	if err != nil {
		t.Fatalf("new overlay: %v", err)
	}
	values, err := New(overlay, key)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}
	if _, found, err := persistent.Read(t.Context(), key.Name()); err != nil || found {
		t.Fatalf("override was persisted during construction: found = %v, error = %v", found, err)
	}

	if err := Set(t.Context(), values, key, "persisted later"); err != nil {
		t.Fatalf("set through overlay: %v", err)
	}
	got, err := Get(t.Context(), values, key)
	if err != nil || got != "flag" {
		t.Fatalf("overlaid theme = %q, error = %v, want flag", got, err)
	}

	persistedValues, err := New(persistent, key)
	if err != nil {
		t.Fatalf("new persisted settings: %v", err)
	}
	got, err = Get(t.Context(), persistedValues, key)
	if err != nil || got != "persisted later" {
		t.Fatalf("persisted theme = %q, error = %v", got, err)
	}

	if err := Delete(t.Context(), values, key); err != nil {
		t.Fatalf("delete through overlay: %v", err)
	}
	got, err = Get(t.Context(), values, key)
	if err != nil || got != "flag" {
		t.Fatalf("theme after delete = %q, error = %v, want transient flag", got, err)
	}
	got, err = Get(t.Context(), persistedValues, key)
	if err != nil || got != "default" {
		t.Fatalf("persistence after delete = %q, error = %v, want default", got, err)
	}
}

func TestOverridePreservesExplicitZeroAndValidation(t *testing.T) {
	t.Parallel()

	enabled := NewKey("enabled", true, nil)
	override := requireOverride(t, enabled, false)
	overlay, err := NewOverlay(newMemoryStore(), nil, []Override{override})
	if err != nil {
		t.Fatalf("new overlay: %v", err)
	}
	values, err := New(overlay, enabled)
	if err != nil {
		t.Fatalf("new settings: %v", err)
	}
	got, err := Get(t.Context(), values, enabled)
	if err != nil || got {
		t.Fatalf("enabled = %v, error = %v, want false", got, err)
	}

	count := NewKey("count", 0, func(value int) error {
		if value < 0 {
			return errors.New("must be non-negative")
		}
		return nil
	})
	if _, err := NewOverride(count, -1); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("validation error = %v, want ErrInvalidValue", err)
	}
	ratio := NewKey("ratio", 1.0, nil)
	if _, err := NewOverride(ratio, math.NaN()); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("encoding error = %v, want ErrInvalidValue", err)
	}
}

func TestNewOverlayRejectsInvalidSources(t *testing.T) {
	t.Parallel()

	key := NewKey("theme", "system", nil)
	override := requireOverride(t, key, "dark")

	if _, err := NewOverlay(nil, nil, nil); err == nil {
		t.Fatal("NewOverlay accepted a nil persistent store")
	}
	if _, err := NewOverlay(newMemoryStore(), []Override{{}}, nil); err == nil {
		t.Fatal("NewOverlay accepted a zero override")
	}
	if _, err := NewOverlay(newMemoryStore(), []Override{override, override}, nil); err == nil {
		t.Fatal("NewOverlay accepted duplicate flag overrides")
	}
	if _, err := NewOverlay(newMemoryStore(), nil, []Override{override, override}); err == nil {
		t.Fatal("NewOverlay accepted duplicate environment overrides")
	}
	if _, err := NewOverlay(newMemoryStore(), []Override{override}, []Override{override}); err != nil {
		t.Fatalf("NewOverlay rejected intentional cross-source precedence: %v", err)
	}
}

func TestOverlayCopiesValuesHonorsCancellationAndSupportsConcurrency(t *testing.T) {
	t.Parallel()

	key := NewKey("theme", "system", nil)
	override := requireOverride(t, key, "dark")
	overlay, err := NewOverlay(newMemoryStore(), []Override{override}, nil)
	if err != nil {
		t.Fatalf("new overlay: %v", err)
	}
	override.value[0] = '['

	raw, found, err := overlay.Read(t.Context(), key.Name())
	if err != nil || !found || string(raw) != `"dark"` {
		t.Fatalf("read override = %q, %v, %v", raw, found, err)
	}
	raw[0] = '['
	raw, _, err = overlay.Read(t.Context(), key.Name())
	if err != nil || string(raw) != `"dark"` {
		t.Fatalf("reread override = %q, %v, want independent bytes", raw, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := overlay.Read(ctx, key.Name()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read error = %v", err)
	}

	const readers = 64
	errorsFound := make(chan error, readers)
	var wait sync.WaitGroup
	for range readers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value, found, err := overlay.Read(t.Context(), key.Name())
			if err != nil || !found || string(value) != `"dark"` {
				errorsFound <- errors.New("unexpected concurrent override read")
			}
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Error(err)
	}
}

func requireOverride[T any](t *testing.T, key Key[T], value T) Override {
	t.Helper()
	override, err := NewOverride(key, value)
	if err != nil {
		t.Fatalf("new override: %v", err)
	}
	return override
}
