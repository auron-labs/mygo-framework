// Package secrets defines framework-neutral access to OS-protected secrets.
package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound means no secret is stored for a key.
var ErrNotFound = errors.New("secret not found")

// SecretStore is the injectable boundary for protected secret storage.
// Implementations must be safe for concurrent use and return caller-owned bytes.
type SecretStore interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte) error
	Delete(context.Context, string) error
}

// ValidateKey rejects keys that cannot be represented consistently by the
// supported secret stores.
func ValidateKey(key string) error {
	if key == "" {
		return errors.New("secret key is required")
	}
	if strings.ContainsRune(key, 0) {
		return fmt.Errorf("secret key %q contains NUL", key)
	}

	return nil
}
