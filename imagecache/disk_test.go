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

func TestDiskHitProtectsFrequentlyUsedEntryFromPrune(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oldURL := "https://example.test/frequently-shown.png"
	middleURL := "https://example.test/unused.png"
	newURL := "https://example.test/new.png"
	freshUntil := time.Now().Add(time.Hour).Truncate(time.Second)
	for _, rawURL := range []string{oldURL, middleURL} {
		if err := writeDiskFiles(dir, rawURL, []byte("1234567890"), diskMetadata{FreshUntil: freshUntil}); err != nil {
			t.Fatal(err)
		}
	}
	for i, rawURL := range []string{oldURL, middleURL} {
		body, meta := diskPaths(dir, rawURL)
		at := time.Now().Add(time.Duration(i-2) * time.Hour)
		for _, path := range []string{body, meta} {
			if err := os.Chtimes(path, at, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	body, meta := diskPaths(dir, oldURL)
	bodyInfo, err := os.Stat(body)
	if err != nil {
		t.Fatal(err)
	}
	metaInfo, err := os.Stat(meta)
	if err != nil {
		t.Fatal(err)
	}
	pairSize := bodyInfo.Size() + metaInfo.Size()
	cache := newTestCache(t, dir, Options{DiskBytes: 2*pairSize + 1})

	// A disk hit is use, even though the body bytes do not change. The write of
	// a third entry must prune the untouched middle entry rather than this one.
	// Scan first so the hit must update the live in-memory index, not only the
	// filesystem mtimes observed by a later scan.
	cache.diskMu.Lock()
	cache.ensureDiskIndexLocked()
	cache.diskMu.Unlock()
	cache.touchDiskFiles(oldURL, cache.diskToken(oldURL))
	time.Sleep(20 * time.Millisecond)
	key := cacheKey{url: newURL, maxSide: 8}
	e := &entry{key: key}
	cache.mu.Lock()
	cache.entries[key] = e
	cache.mu.Unlock()
	cache.storeIfCurrent(e, fetched{
		url:      newURL,
		data:     []byte("1234567890"),
		metadata: diskMetadata{FreshUntil: freshUntil},
		token:    cache.diskToken(newURL),
	})

	oldBody, _ := diskPaths(dir, oldURL)
	if _, err := os.Stat(oldBody); err != nil {
		t.Fatalf("recently used body was pruned: %v", err)
	}
	middleBody, _ := diskPaths(dir, middleURL)
	if _, err := os.Stat(middleBody); !os.IsNotExist(err) {
		t.Fatalf("untouched middle body remains or stat failed: %v", err)
	}
	newBody, _ := diskPaths(dir, newURL)
	if _, err := os.Stat(newBody); err != nil {
		t.Fatalf("newly written body was pruned: %v", err)
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
