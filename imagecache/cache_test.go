package imagecache

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/httpclient"
)

func testPNG(t *testing.T, width, height int, fill color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.NewUniform(fill), image.Point{}, draw.Src)
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func testHTTPClient(t *testing.T) *httpclient.Client {
	t.Helper()
	client, err := httpclient.New(httpclient.Options{
		Timeout: 2 * time.Second,
		Logger:  slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func newTestCache(t *testing.T, dir string, options Options) *Cache {
	t.Helper()
	return newTestCacheWorkers(t, dir, options, 1)
}

func newTestCacheWorkers(t *testing.T, dir string, options Options, workers int) *Cache {
	t.Helper()
	if options.Client == nil {
		options.Client = testHTTPClient(t)
	}
	if options.Dir == "" {
		options.Dir = dir
	}
	cache, err := newCache(context.Background(), options, workers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := cache.Shutdown(ctx); err != nil {
			t.Errorf("shutdown cache: %v", err)
		}
	})
	return cache
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !condition() {
		t.Fatalf("timed out waiting for %s", what)
	}
}

func hasEntry(cache *Cache, rawURL string, maxSide int) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	_, ok := cache.entries[cacheKey{url: rawURL, maxSide: effectiveMaxSide(maxSide)}]
	return ok
}

func hasDecoded(cache *Cache, rawURL string, maxSide int) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	e := cache.entries[cacheKey{url: rawURL, maxSide: effectiveMaxSide(maxSide)}]
	return e != nil && e.source != nil
}

func cacheError(cache *Cache, rawURL string, maxSide int) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	e := cache.entries[cacheKey{url: rawURL, maxSide: effectiveMaxSide(maxSide)}]
	if e == nil {
		return nil
	}
	return e.err
}

func TestLoadDeduplicatesConcurrentRequests(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 96, 48, color.NRGBA{R: 220, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		time.Sleep(20 * time.Millisecond)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})

	rawURL := server.URL + "/avatar.png"
	const callers = 16
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			source, err := cache.Load(nil, rawURL, LoadOptions{MaxSide: 32})
			if err != nil || source != nil {
				t.Errorf("initial load = %v, %v", source, err)
			}
		}()
	}
	wg.Wait()
	waitFor(t, "decoded image", func() bool { return hasDecoded(cache, rawURL, 32) })

	first, err := cache.Load(nil, rawURL, LoadOptions{MaxSide: 32})
	if err != nil || first == nil {
		t.Fatalf("loaded source = %v, %v", first, err)
	}
	for range 4 {
		source, err := cache.Load(nil, rawURL, LoadOptions{MaxSide: 32})
		if err != nil || source != first {
			t.Fatalf("repeated source = %v, %v; want cached %v", source, err, first)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("network requests = %d, want 1", got)
	}
	bitmap, ok := first.(interface{ Size() (int, int) })
	if !ok {
		t.Fatalf("source type %T has no size", first)
	}
	if w, h := bitmap.Size(); w != 32 || h != 16 {
		t.Fatalf("scaled size = %dx%d, want 32x16", w, h)
	}
}

func TestDifferentSizesShareOneByteFetch(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 96, 48, color.NRGBA{G: 200, A: 255})
	var requests atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			close(started)
			w.Header().Set("Cache-Control", "no-store")
			<-release
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	cache := newTestCacheWorkers(t, t.TempDir(), Options{}, 2)
	rawURL := server.URL + "/shared.png"

	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 32})
	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 64})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("shared request did not start")
	}
	// Let the second worker reach singleflight before the first response is
	// allowed to finish. no-store means a later non-overlapping size would have
	// to fetch again rather than silently passing through the disk cache.
	time.Sleep(100 * time.Millisecond)
	close(release)
	waitFor(t, "both decoded sizes", func() bool {
		return hasDecoded(cache, rawURL, 32) && hasDecoded(cache, rawURL, 64)
	})
	if got := requests.Load(); got != 1 {
		t.Fatalf("network requests = %d, want 1", got)
	}
}

func TestNewCacheInstanceUsesFreshDiskBytes(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 20, 20, color.NRGBA{B: 220, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	rawURL := server.URL + "/fresh.png"

	first := newTestCache(t, dir, Options{})
	_, _ = first.Load(nil, rawURL, LoadOptions{MaxSide: 20})
	waitFor(t, "first cache decode", func() bool { return hasDecoded(first, rawURL, 20) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := first.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	cancel()

	second := newTestCache(t, dir, Options{})
	_, _ = second.Load(nil, rawURL, LoadOptions{MaxSide: 20})
	waitFor(t, "second cache decode", func() bool { return hasDecoded(second, rawURL, 20) })
	if got := requests.Load(); got != 1 {
		t.Fatalf("network requests = %d, want fresh disk reuse without another request", got)
	}
}

func TestFailureIsNegativeCachedUntilRetryAfter(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})
	cache.retryAfter = time.Hour
	rawURL := server.URL + "/broken.png"

	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 20})
	waitFor(t, "failure", func() bool { return cacheError(cache, rawURL, 20) != nil })
	for range 5 {
		if _, err := cache.Load(nil, rawURL, LoadOptions{MaxSide: 20}); err == nil {
			t.Fatal("negative-cached load returned no error")
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("network requests during negative cache = %d, want 1", got)
	}

	cache.mu.Lock()
	if e := cache.entries[cacheKey{url: rawURL, maxSide: 20}]; e != nil {
		e.failed = e.failed.Add(-cache.retryAfter)
	}
	cache.mu.Unlock()
	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 20})
	waitFor(t, "retry", func() bool { return requests.Load() == 2 })
}

func TestInvalidHTMLIsRejectedAndNotCached(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>captive portal</body></html>"))
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cache := newTestCache(t, dir, Options{})
	rawURL := server.URL + "/private/avatar?token=secret"

	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 20})
	waitFor(t, "decode failure", func() bool { return cacheError(cache, rawURL, 20) != nil })
	if err := cacheError(cache, rawURL, 20); err == nil || err.Error() == "" || strings.Contains(err.Error(), rawURL) || strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "captive portal") {
		t.Fatalf("error is missing or leaks private details: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range entries {
		if isCacheBodyName(item.Name()) {
			t.Fatalf("undecodable body was cached as %s", item.Name())
		}
	}
}

func TestNewestQueuedRequestRunsFirst(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 8, 8, color.NRGBA{R: 255, A: 255})
	release := make(chan struct{})
	blocking := make(chan struct{})
	started := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- r.URL.Path
		if r.URL.Path == "/block.png" {
			close(blocking)
			<-release
		}
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})

	_, _ = cache.Load(nil, server.URL+"/block.png", LoadOptions{MaxSide: 8})
	select {
	case <-blocking:
	case <-time.After(time.Second):
		t.Fatal("blocking request did not start")
	}
	_, _ = cache.Load(nil, server.URL+"/old.png", LoadOptions{MaxSide: 8})
	_, _ = cache.Load(nil, server.URL+"/new.png", LoadOptions{MaxSide: 8})
	close(release)

	waitFor(t, "queued images", func() bool {
		return hasDecoded(cache, server.URL+"/old.png", 8) && hasDecoded(cache, server.URL+"/new.png", 8)
	})
	close(started)
	var order []string
	for path := range started {
		order = append(order, path)
	}
	if len(order) != 3 || order[1] != "/new.png" || order[2] != "/old.png" {
		t.Fatalf("request order = %v, want block, new, old", order)
	}
}

func TestQueuedEntryNotRecentlyRequestedIsAbandoned(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 8, 8, color.NRGBA{R: 255, A: 255})
	release := make(chan struct{})
	blocking := make(chan struct{})
	var staleRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stale.png" {
			staleRequests.Add(1)
		}
		if r.URL.Path == "/block.png" {
			close(blocking)
			<-release
		}
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})
	cache.abandonAfter = 30 * time.Millisecond

	_, _ = cache.Load(nil, server.URL+"/block.png", LoadOptions{MaxSide: 8})
	select {
	case <-blocking:
	case <-time.After(time.Second):
		t.Fatal("blocking request did not start")
	}
	staleURL := server.URL + "/stale.png"
	_, _ = cache.Load(nil, staleURL, LoadOptions{MaxSide: 8})
	time.Sleep(60 * time.Millisecond)
	close(release)

	waitFor(t, "abandoned entry removal", func() bool { return !hasEntry(cache, staleURL, 8) })
	if got := staleRequests.Load(); got != 0 {
		t.Fatalf("abandoned URL reached server %d times", got)
	}
}

func TestSlowArrivalDoesNotShieldLRUEviction(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 8, 8, color.NRGBA{R: 255, A: 255})
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow.png" {
			close(started)
			<-release
		}
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	cache := newTestCacheWorkers(t, t.TempDir(), Options{MemoryEntries: 2}, 3)
	cache.protectFor = 0
	slowURL := server.URL + "/slow.png"
	firstURL := server.URL + "/first.png"
	secondURL := server.URL + "/second.png"

	_, _ = cache.Load(nil, slowURL, LoadOptions{MaxSide: 8})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("slow request did not start")
	}
	_, _ = cache.Load(nil, firstURL, LoadOptions{MaxSide: 8})
	_, _ = cache.Load(nil, secondURL, LoadOptions{MaxSide: 8})
	waitFor(t, "newer images", func() bool {
		return hasDecoded(cache, firstURL, 8) && hasDecoded(cache, secondURL, 8)
	})
	close(release)
	waitFor(t, "slow image", func() bool { return hasDecoded(cache, slowURL, 8) })

	if hasEntry(cache, firstURL, 8) {
		t.Fatal("oldest completed image was not evicted past MemoryEntries")
	}
	if !hasDecoded(cache, slowURL, 8) || !hasDecoded(cache, secondURL, 8) {
		t.Fatal("wrong images retained after slow arrival eviction")
	}
}

func TestForgetWhileFetchIsInFlightKeepsNothing(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 8, 8, color.NRGBA{R: 255, A: 255})
	release := make(chan struct{})
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cache := newTestCache(t, dir, Options{})
	rawURL := server.URL + "/forget.png"

	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 8})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	if err := cache.Forget(rawURL); err != nil {
		t.Fatal(err)
	}
	close(release)
	waitFor(t, "in-flight completion", func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return len(cache.entries) == 0
	})
	body, meta := diskPaths(dir, rawURL)
	if _, err := os.Stat(body); !os.IsNotExist(err) {
		t.Fatalf("forgotten body exists or stat failed: %v", err)
	}
	if _, err := os.Stat(meta); !os.IsNotExist(err) {
		t.Fatalf("forgotten metadata exists or stat failed: %v", err)
	}
}

func TestClearRemovesOnlyCacheOwnedFiles(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 8, 8, color.NRGBA{R: 255, A: 255})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cache := newTestCache(t, dir, Options{})
	rawURL := server.URL + "/clear.png"
	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 8})
	waitFor(t, "decoded image", func() bool { return hasDecoded(cache, rawURL, 8) })

	foreign := dir + string(os.PathSeparator) + "foreign.txt"
	if err := os.WriteFile(foreign, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cache.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign file was removed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range entries {
		if item.Name() != "foreign.txt" {
			t.Fatalf("cache-owned file remains after Clear: %s", item.Name())
		}
	}
	cache.mu.Lock()
	count := len(cache.entries)
	held := cache.memoryBytes
	cache.mu.Unlock()
	if count != 0 || held != 0 {
		t.Fatalf("entries=%d memory=%d after Clear", count, held)
	}
}

func TestShutdownCancelsInFlightWork(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	cache := newTestCache(t, t.TempDir(), Options{})
	_, _ = cache.Load(nil, server.URL+"/slow.png", LoadOptions{MaxSide: 8})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("in-flight request did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := cache.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown did not cancel in-flight work: %v", err)
	}
	if _, err := cache.Load(nil, server.URL+"/slow.png", LoadOptions{MaxSide: 8}); !errors.Is(err, ErrShutdown) {
		t.Fatalf("load after shutdown error = %v, want ErrShutdown", err)
	}
}

func TestLRUEvictsByCountAndBytesLeastRecentlyUsedFirst(t *testing.T) {
	t.Parallel()

	cache := newTestCache(t, t.TempDir(), Options{MemoryEntries: 2, MemoryBytes: 1 << 20})
	cache.protectFor = 0
	source, _, err := decodeSafely(testPNG(t, 4, 4, color.NRGBA{A: 255}), 4)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now()
	entries := []*entry{
		{key: cacheKey{url: "https://example.test/a.png", maxSide: 4}, source: source, memoryBytes: 100, used: base},
		{key: cacheKey{url: "https://example.test/b.png", maxSide: 4}, source: source, memoryBytes: 100, used: base.Add(2 * time.Second)},
		{key: cacheKey{url: "https://example.test/c.png", maxSide: 4}, source: source, memoryBytes: 100, used: base.Add(time.Second)},
	}
	// Complete them out of request order. The LRU must still order by use time.
	for _, e := range []*entry{entries[0], entries[2], entries[1]} {
		cache.mu.Lock()
		cache.entries[e.key] = e
		cache.insertLRULocked(e)
		cache.memoryBytes += e.memoryBytes
		cache.mu.Unlock()
	}
	cache.mu.Lock()
	front := cache.lruList.Front().Value.(*entry)
	back := cache.lruList.Back().Value.(*entry)
	if front != entries[1] || back != entries[0] {
		t.Fatalf("LRU order front=%s back=%s", front.key.url, back.key.url)
	}
	cache.evictLocked(nil, base.Add(3*time.Second))
	cache.mu.Unlock()

	if hasEntry(cache, entries[0].key.url, 4) {
		t.Fatal("count eviction did not remove the least recently used entry")
	}
	if !hasEntry(cache, entries[1].key.url, 4) || !hasEntry(cache, entries[2].key.url, 4) {
		t.Fatal("count eviction removed newer entries")
	}

	cache.mu.Lock()
	cache.memoryByteLimit = 150
	cache.evictLocked(nil, base.Add(3*time.Second))
	held := cache.memoryBytes
	cache.mu.Unlock()
	if hasEntry(cache, entries[2].key.url, 4) {
		t.Fatal("byte eviction did not remove the next least recently used entry")
	}
	if !hasEntry(cache, entries[1].key.url, 4) || held != 100 {
		t.Fatalf("byte eviction retained wrong entry or memory: held=%d", held)
	}
}

func TestLRURecentUseProtection(t *testing.T) {
	t.Parallel()

	cache := newTestCache(t, t.TempDir(), Options{MemoryEntries: 1, MemoryBytes: 1 << 20})
	cache.protectFor = time.Hour
	source, _, err := decodeSafely(testPNG(t, 4, 4, color.NRGBA{A: 255}), 4)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cache.mu.Lock()
	for _, suffix := range []string{"a", "b"} {
		e := &entry{key: cacheKey{url: "https://example.test/" + suffix + ".png", maxSide: 4}, source: source, memoryBytes: 100, used: now}
		cache.entries[e.key] = e
		cache.insertLRULocked(e)
		cache.memoryBytes += e.memoryBytes
	}
	cache.evictLocked(nil, now.Add(30*time.Minute))
	count := cache.lruList.Len()
	cache.mu.Unlock()
	if count != 2 {
		t.Fatalf("protected entries evicted: LRU length = %d, want 2", count)
	}
}
