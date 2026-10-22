// Package secretstest defines the common SecretStore behavior checks.
package secretstest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mygo-framework/secrets"
	"strconv"
	"sync"
	"testing"
)

// Factory returns an isolated secret store for one subtest.
type Factory func(t *testing.T) secrets.SecretStore

// Run exercises behavior shared by secret store implementations.
func Run(t *testing.T, factory Factory) {
	t.Helper()

	t.Run("lifecycle and byte ownership", func(t *testing.T) {
		store := factory(t)
		if _, err := store.Get(t.Context(), "token"); !errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("missing Get error = %v, want ErrNotFound", err)
		}

		want := []byte("s3cr\x00t")
		input := append([]byte(nil), want...)
		if err := store.Set(t.Context(), "token", input); err != nil {
			t.Fatalf("set secret: %v", err)
		}
		input[0] = 'X'
		got, err := store.Get(t.Context(), "token")
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("Get = %q, %v; want %q, nil", got, err, want)
		}
		got[0] = 'X'
		got, err = store.Get(t.Context(), "token")
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("second Get = %q, %v; want independent %q", got, err, want)
		}

		if err := store.Set(t.Context(), "token", nil); err != nil {
			t.Fatalf("set empty secret: %v", err)
		}
		if got, err := store.Get(t.Context(), "token"); err != nil || len(got) != 0 {
			t.Fatalf("empty Get = %q, %v", got, err)
		}
		if err := store.Delete(t.Context(), "token"); err != nil {
			t.Fatalf("delete secret: %v", err)
		}
		if err := store.Delete(t.Context(), "token"); err != nil {
			t.Fatalf("delete missing secret: %v", err)
		}
		if _, err := store.Get(t.Context(), "token"); !errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("Get after Delete error = %v, want ErrNotFound", err)
		}
	})

	t.Run("validation and cancellation", func(t *testing.T) {
		store := factory(t)
		for _, key := range []string{"", "bad\x00key"} {
			if err := store.Set(t.Context(), key, nil); err == nil {
				t.Errorf("Set accepted key %q", key)
			}
		}

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := store.Get(ctx, "token"); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Get error = %v", err)
		}
		if err := store.Set(ctx, "token", nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Set error = %v", err)
		}
		if err := store.Delete(ctx, "token"); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Delete error = %v", err)
		}
	})

	t.Run("concurrent use", func(t *testing.T) {
		store := factory(t)
		const workers = 32
		errorsFound := make(chan error, workers)
		var wait sync.WaitGroup
		for worker := range workers {
			wait.Add(1)
			go func(index int) {
				defer wait.Done()
				key := "token-" + strconv.Itoa(index)
				want := []byte(key)
				if err := store.Set(t.Context(), key, want); err != nil {
					errorsFound <- fmt.Errorf("set %s: %w", key, err)
					return
				}
				got, err := store.Get(t.Context(), key)
				if err != nil {
					errorsFound <- fmt.Errorf("get %s: %w", key, err)
					return
				}
				if !bytes.Equal(got, want) {
					errorsFound <- fmt.Errorf("get %s = %q", key, got)
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
