// Package mygo adapts MyGo Secrets to the framework-neutral SecretStore.
package mygo

import (
	"context"
	"errors"
	"fmt"
	"mygo-framework/secrets"
	"strings"

	mygoplatform "github.com/egoist/mygo"
)

type platformSecrets interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}

// Store delegates exclusively to the OS credential store selected by MyGo.
type Store struct {
	platform platformSecrets
}

var _ secrets.SecretStore = (*Store)(nil)

// New creates a store isolated by stable application and environment names.
func New(application, environment string) (*Store, error) {
	namespace, err := serviceNamespace(application, environment)
	if err != nil {
		return nil, err
	}

	return newStore(mygoplatform.Secrets.Service(namespace)), nil
}

func newStore(platform platformSecrets) *Store {
	return &Store{platform: platform}
}

// Get reads a copied secret and preserves missing and unsupported errors.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("get OS secret %q: %w", key, err)
	}
	if err := secrets.ValidateKey(key); err != nil {
		return nil, fmt.Errorf("get OS secret: %w", err)
	}

	value, err := s.platform.Get(key)
	if err != nil {
		if errors.Is(err, mygoplatform.ErrSecretNotFound) {
			return nil, fmt.Errorf("get OS secret %q: %w: %w", key, secrets.ErrNotFound, err)
		}
		return nil, fmt.Errorf("get OS secret %q: %w", key, err)
	}

	return append([]byte(nil), value...), nil
}

// Set writes a copied secret to the OS credential store.
func (s *Store) Set(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("set OS secret %q: %w", key, err)
	}
	if err := secrets.ValidateKey(key); err != nil {
		return fmt.Errorf("set OS secret: %w", err)
	}
	if err := s.platform.Set(key, append([]byte(nil), value...)); err != nil {
		return fmt.Errorf("set OS secret %q: %w", key, err)
	}

	return nil
}

// Delete removes a secret without any plaintext fallback.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("delete OS secret %q: %w", key, err)
	}
	if err := secrets.ValidateKey(key); err != nil {
		return fmt.Errorf("delete OS secret: %w", err)
	}
	if err := s.platform.Delete(key); err != nil && !errors.Is(err, mygoplatform.ErrSecretNotFound) {
		return fmt.Errorf("delete OS secret %q: %w", key, err)
	}

	return nil
}

func serviceNamespace(application, environment string) (string, error) {
	for label, value := range map[string]string{
		"application": application,
		"environment": environment,
	} {
		if value == "" || value != strings.TrimSpace(value) {
			return "", fmt.Errorf("secret %s namespace is required without surrounding whitespace", label)
		}
		if strings.ContainsAny(value, "/\x00") {
			return "", fmt.Errorf("secret %s namespace %q contains a reserved character", label, value)
		}
	}

	return application + "/" + environment, nil
}
