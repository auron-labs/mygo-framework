package bbolt

import (
	"errors"
	"mygo-framework/internal/settingstest"
	"mygo-framework/settings"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"
)

func TestStoreContract(t *testing.T) {
	t.Parallel()
	settingstest.Run(t, openStore)
}

func TestOpenCreatesOnlyNamespacedSettingsBucket(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	buckets := 0
	if err := store.database.View(func(transaction *bolt.Tx) error {
		return transaction.ForEach(func(name []byte, _ *bolt.Bucket) error {
			buckets++
			if string(name) != string(settingsBucket) {
				t.Errorf("bucket = %q, want %q", name, settingsBucket)
			}
			return nil
		})
	}); err != nil {
		t.Fatalf("inspect buckets: %v", err)
	}
	if buckets != 1 {
		t.Fatalf("bucket count = %d, want 1", buckets)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat database: %v", err)
	}
	if permissions := info.Mode().Perm(); runtime.GOOS != "windows" && permissions&0o077 != 0 {
		t.Fatalf("database permissions = %o, want no group or world access", permissions)
	}
}

func TestOpenReturnsPromptActionableBusyError(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.db")
	first, err := Open(path)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	t.Cleanup(func() {
		if err := first.Close(); err != nil {
			t.Errorf("close first store: %v", err)
		}
	})

	startedAt := time.Now()
	second, err := Open(path)
	elapsed := time.Since(startedAt)
	if second != nil {
		_ = second.Close()
		t.Fatal("second Open succeeded while the database was locked")
	}
	if !errors.Is(err, berrors.ErrTimeout) {
		t.Fatalf("second Open error = %v, want bbolt timeout", err)
	}
	if !strings.Contains(err.Error(), "database is busy") {
		t.Fatalf("second Open error is not actionable: %v", err)
	}
	if elapsed > openTimeout+time.Second {
		t.Fatalf("lock failure took %s, want at most %s", elapsed, openTimeout+time.Second)
	}
}

func TestCloseIsIdempotentAndReleasesLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "settings.db")
	first, err := Open(path)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first store again: %v", err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close reopened store: %v", err)
	}
}

func TestOpenRejectsEmptyPath(t *testing.T) {
	t.Parallel()
	if _, err := Open(""); err == nil {
		t.Fatal("Open accepted an empty path")
	}
}

func openStore(t *testing.T, location string) (settings.Store, func()) {
	t.Helper()
	store, err := Open(location)
	if err != nil {
		t.Fatalf("open bbolt settings: %v", err)
	}

	return store, func() {
		if err := store.Close(); err != nil {
			t.Errorf("close bbolt settings: %v", err)
		}
	}
}
