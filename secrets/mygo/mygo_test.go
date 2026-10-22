package mygo

import (
	"errors"
	"mygo-framework/internal/secretstest"
	"mygo-framework/secrets"
	"strings"
	"sync"
	"testing"

	mygoplatform "github.com/egoist/mygo"
)

type fakePlatformSecrets struct {
	mu     sync.RWMutex
	values map[string][]byte
	err    error
}

func newFakePlatformSecrets() *fakePlatformSecrets {
	return &fakePlatformSecrets{values: make(map[string][]byte)}
}

func (f *fakePlatformSecrets) Get(key string) ([]byte, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.err != nil {
		return nil, f.err
	}
	value, found := f.values[key]
	if !found {
		return nil, mygoplatform.ErrSecretNotFound
	}
	return value, nil
}

func (f *fakePlatformSecrets) Set(key string, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.values[key] = value
	return nil
}

func (f *fakePlatformSecrets) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	delete(f.values, key)
	return nil
}

func TestStoreContract(t *testing.T) {
	t.Parallel()
	secretstest.Run(t, func(*testing.T) secrets.SecretStore {
		return newStore(newFakePlatformSecrets())
	})
}

func TestNewUsesStableApplicationEnvironmentNamespace(t *testing.T) {
	t.Parallel()

	namespace, err := serviceNamespace("com.example.notes", "development")
	if err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	if namespace != "com.example.notes/development" {
		t.Fatalf("namespace = %q", namespace)
	}
	if _, err := New("com.example.notes", "development"); err != nil {
		t.Fatalf("new store: %v", err)
	}

	invalid := [][2]string{
		{"", "development"},
		{"com.example.notes", ""},
		{" com.example.notes", "development"},
		{"com.example/notes", "development"},
		{"com.example.notes", "dev/test"},
		{"com.example.notes", "dev\x00test"},
	}
	for _, parts := range invalid {
		if _, err := New(parts[0], parts[1]); err == nil {
			t.Errorf("New accepted application %q and environment %q", parts[0], parts[1])
		}
	}
}

func TestStorePreservesMissingAndUnavailableErrors(t *testing.T) {
	t.Parallel()

	platform := newFakePlatformSecrets()
	store := newStore(platform)
	_, err := store.Get(t.Context(), "missing")
	if !errors.Is(err, secrets.ErrNotFound) || !errors.Is(err, mygoplatform.ErrSecretNotFound) {
		t.Fatalf("missing error = %v, want both missing sentinels", err)
	}

	platform.err = errors.ErrUnsupported
	for operation, err := range map[string]error{
		"get":    func() error { _, err := store.Get(t.Context(), "token"); return err }(),
		"set":    store.Set(t.Context(), "token", []byte("secret")),
		"delete": store.Delete(t.Context(), "token"),
	} {
		if !errors.Is(err, errors.ErrUnsupported) {
			t.Errorf("%s error = %v, want errors.ErrUnsupported", operation, err)
		}
		if !strings.Contains(err.Error(), "OS secret") {
			t.Errorf("%s error lacks operation context: %v", operation, err)
		}
	}
}

func TestDeleteTreatsPlatformMissingAsSuccess(t *testing.T) {
	t.Parallel()

	platform := newFakePlatformSecrets()
	platform.err = mygoplatform.ErrSecretNotFound
	store := newStore(platform)
	if err := store.Delete(t.Context(), "missing"); err != nil {
		t.Fatalf("delete missing secret: %v", err)
	}
}
