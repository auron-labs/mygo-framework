package imagecache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDiskFilesAreHashedPrivateAndAtomic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rawURL := "https://example.test/private/avatar.png?token=secret"
	data := []byte("picture bytes")
	meta := diskMetadata{ETag: `"v1"`, FreshUntil: time.Now().Add(time.Hour)}
	if err := writeDiskFiles(dir, rawURL, data, meta); err != nil {
		t.Fatal(err)
	}
	bodyPath, metaPath := diskPaths(dir, rawURL)
	for _, path := range []string{bodyPath, metaPath} {
		base := filepath.Base(path)
		if strings.Contains(base, "avatar") || strings.Contains(base, "secret") || strings.Contains(base, "example.test") {
			t.Fatalf("cache file name leaks URL: %s", base)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o600 {
				t.Fatalf("%s mode = %o, want 600", path, got)
			}
		}
	}
	got, gotMeta, ok := readDiskFiles(dir, rawURL, maxImageBytes)
	if !ok || string(got) != string(data) || gotMeta.ETag != meta.ETag {
		t.Fatalf("read disk = %q, %+v, %v", got, gotMeta, ok)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range entries {
		if isCacheTempName(item.Name()) {
			t.Fatalf("temporary file remains after atomic write: %s", item.Name())
		}
	}
}

func TestPruneRemovesOldestAndCacheArtifactsOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	urls := []string{
		"https://example.test/one.png",
		"https://example.test/two.png",
		"https://example.test/three.png",
	}
	freshUntil := time.Now().Add(time.Hour).Truncate(time.Second)
	for _, rawURL := range urls {
		if err := writeDiskFiles(dir, rawURL, []byte("1234567890"), diskMetadata{FreshUntil: freshUntil}); err != nil {
			t.Fatal(err)
		}
	}
	for i, rawURL := range urls {
		body, meta := diskPaths(dir, rawURL)
		at := time.Now().Add(time.Duration(i-3) * time.Hour)
		for _, path := range []string{body, meta} {
			if err := os.Chtimes(path, at, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	foreign := filepath.Join(dir, "foreign.txt")
	if err := os.WriteFile(foreign, []byte("do not remove"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "imagecache-interrupted.tmp"), []byte("temp"), 0o600); err != nil {
		t.Fatal(err)
	}
	orphanMeta := filepath.Join(dir, strings.Repeat("a", cacheFileHexChars)+".json")
	if raw, err := json.Marshal(diskMetadata{FreshUntil: time.Now()}); err == nil {
		if err := os.WriteFile(orphanMeta, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	newestBody, newestMeta := diskPaths(dir, urls[2])
	bodyInfo, err := os.Stat(newestBody)
	if err != nil {
		t.Fatal(err)
	}
	metaInfo, err := os.Stat(newestMeta)
	if err != nil {
		t.Fatal(err)
	}
	pairSize := bodyInfo.Size() + metaInfo.Size()
	if err := pruneDiskFiles(dir, 2*pairSize+1); err != nil {
		t.Fatal(err)
	}

	oldest, _ := diskPaths(dir, urls[0])
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("oldest body remains or stat failed: %v", err)
	}
	for _, rawURL := range urls[1:] {
		body, _ := diskPaths(dir, rawURL)
		if _, err := os.Stat(body); err != nil {
			t.Fatalf("newer body missing: %v", err)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign file was pruned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "imagecache-interrupted.tmp")); !os.IsNotExist(err) {
		t.Fatalf("temporary file remains or stat failed: %v", err)
	}
	if _, err := os.Stat(orphanMeta); !os.IsNotExist(err) {
		t.Fatalf("orphan metadata remains or stat failed: %v", err)
	}
}

func TestPruneRemovesCorruptSidecarButKeepsBody(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rawURL := "https://example.test/corrupt-sidecar.png"
	if err := writeDiskFiles(dir, rawURL, []byte("image"), diskMetadata{FreshUntil: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	body, meta := diskPaths(dir, rawURL)
	if err := os.WriteFile(meta, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := pruneDiskFiles(dir, defaultDiskBytes); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(meta); !os.IsNotExist(err) {
		t.Fatalf("corrupt metadata remains or stat failed: %v", err)
	}
	if _, err := os.Stat(body); err != nil {
		t.Fatalf("body was removed with corrupt metadata: %v", err)
	}
}

func TestReadDiskRejectsOversizedUntrustedBody(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rawURL := "https://example.test/large.png"
	body, _ := diskPaths(dir, rawURL)
	if err := os.WriteFile(body, make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := readDiskFiles(dir, rawURL, 31); ok {
		t.Fatal("oversized disk body was accepted")
	}
}
