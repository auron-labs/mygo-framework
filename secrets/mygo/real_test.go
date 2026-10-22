package mygo

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"mygo-framework/secrets"
	"os"
	"testing"
)

const realSecretTestEnvironment = "MYGO_REAL_SECRET_TEST"

func TestRealCredentialStore(t *testing.T) {
	if os.Getenv(realSecretTestEnvironment) != "1" {
		t.Skip("set " + realSecretTestEnvironment + "=1 to exercise the OS credential store")
	}

	store, err := New("com.example.mygo-framework", "acceptance")
	if err != nil {
		t.Fatalf("new real secret store: %v", err)
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("create probe key: %v", err)
	}
	key := "probe-" + hex.EncodeToString(suffix[:])
	value := []byte("credential-store-smoke")

	if err := store.Set(t.Context(), key, value); err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			t.Skipf("OS credential store unavailable: %v", err)
		}
		t.Fatalf("set real secret: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Delete(context.Background(), key); err != nil {
			t.Errorf("clean up real secret: %v", err)
		}
	})

	got, err := store.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("get real secret: %v", err)
	}
	if !bytes.Equal(got, value) {
		t.Fatal("real credential store returned a different value")
	}
	if err := store.Delete(t.Context(), key); err != nil {
		t.Fatalf("delete real secret: %v", err)
	}
	if _, err := store.Get(t.Context(), key); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("get deleted real secret: %v, want ErrNotFound", err)
	}
}
