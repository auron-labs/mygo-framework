// Package settingstest defines the common persistence contract for settings adapters.
package settingstest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mygo-framework/settings"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// Open constructs a store at location and returns its sole cleanup function.
type Open func(t *testing.T, location string) (settings.Store, func())

// Run exercises behavior shared by every persistent settings adapter.
func Run(t *testing.T, open Open) {
	t.Helper()

	t.Run("persistence and ownership", func(t *testing.T) {
		location := filepath.Join(t.TempDir(), "settings.db")
		first, closeFirst := open(t, location)
		firstClosed := false
		t.Cleanup(func() {
			if !firstClosed {
				closeFirst()
			}
		})

		if value, found, err := first.Read(t.Context(), "missing"); err != nil || found || len(value) != 0 {
			t.Fatalf("missing read = %q, %v, %v; want nil, false, nil", value, found, err)
		}

		want := json.RawMessage(`{"enabled":false,"count":0,"label":""}`)
		input := append(json.RawMessage(nil), want...)
		if err := first.Write(t.Context(), "display", input); err != nil {
			t.Fatalf("write setting: %v", err)
		}
		input[0] = '['

		got, found, err := first.Read(t.Context(), "display")
		if err != nil || !found || !bytes.Equal(got, want) {
			t.Fatalf("first read = %q, %v, %v; want %q, true, nil", got, found, err, want)
		}
		got[0] = '['
		got, found, err = first.Read(t.Context(), "display")
		if err != nil || !found || !bytes.Equal(got, want) {
			t.Fatalf("second read = %q, %v, %v; want independent %q", got, found, err, want)
		}

		overwrite := json.RawMessage(`["updated"]`)
		if err := first.Write(t.Context(), "display", overwrite); err != nil {
			t.Fatalf("overwrite setting: %v", err)
		}
		closeFirst()
		firstClosed = true

		second, closeSecond := open(t, location)
		t.Cleanup(closeSecond)
		got, found, err = second.Read(t.Context(), "display")
		if err != nil || !found || !bytes.Equal(got, overwrite) {
			t.Fatalf("reopened read = %q, %v, %v; want %q, true, nil", got, found, err, overwrite)
		}
		if err := second.Delete(t.Context(), "display"); err != nil {
			t.Fatalf("delete setting: %v", err)
		}
		if err := second.Delete(t.Context(), "display"); err != nil {
			t.Fatalf("delete missing setting: %v", err)
		}
		if _, found, err := second.Read(t.Context(), "display"); err != nil || found {
			t.Fatalf("read deleted setting: found = %v, error = %v", found, err)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		store, closeStore := open(t, filepath.Join(t.TempDir(), "settings.db"))
		t.Cleanup(closeStore)

		if err := store.Write(t.Context(), "broken", json.RawMessage("[")); err == nil {
			t.Fatal("Write accepted invalid JSON")
		}
		if _, found, err := store.Read(t.Context(), "broken"); err != nil || found {
			t.Fatalf("invalid value persisted: found = %v, error = %v", found, err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		store, closeStore := open(t, filepath.Join(t.TempDir(), "settings.db"))
		t.Cleanup(closeStore)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		if _, _, err := store.Read(ctx, "key"); !errors.Is(err, context.Canceled) {
			t.Fatalf("Read error = %v, want context canceled", err)
		}
		if err := store.Write(ctx, "key", json.RawMessage("0")); !errors.Is(err, context.Canceled) {
			t.Fatalf("Write error = %v, want context canceled", err)
		}
		if err := store.Delete(ctx, "key"); !errors.Is(err, context.Canceled) {
			t.Fatalf("Delete error = %v, want context canceled", err)
		}
	})

	t.Run("concurrent use", func(t *testing.T) {
		store, closeStore := open(t, filepath.Join(t.TempDir(), "settings.db"))
		t.Cleanup(closeStore)

		const workers = 32
		errorsFound := make(chan error, workers)
		var wait sync.WaitGroup
		for worker := range workers {
			wait.Add(1)
			go func(index int) {
				defer wait.Done()
				name := fmt.Sprintf("key-%d", index)
				want := json.RawMessage(strconv.Itoa(index))
				if err := store.Write(t.Context(), name, want); err != nil {
					errorsFound <- fmt.Errorf("write %s: %w", name, err)
					return
				}
				got, found, err := store.Read(t.Context(), name)
				if err != nil {
					errorsFound <- fmt.Errorf("read %s: %w", name, err)
					return
				}
				if !found || !bytes.Equal(got, want) {
					errorsFound <- fmt.Errorf("read %s = %q, %v", name, got, found)
				}
			}(worker)
		}
		wait.Wait()
		close(errorsFound)

		for err := range errorsFound {
			t.Error(err)
		}
	})
}
