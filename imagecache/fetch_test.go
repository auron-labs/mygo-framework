package imagecache

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"image/color"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/httpclient"
)

func expireDiskMetadata(t *testing.T, dir, rawURL string) {
	t.Helper()
	_, metaPath := diskPaths(dir, rawURL)
	raw, err := os.ReadFile(metaPath) // #nosec G304 -- test path is a SHA-256 sidecar inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	var meta diskMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	meta.FreshUntil = time.Now().Add(-time.Hour)
	raw, err = json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readCachedBody(t *testing.T, dir, rawURL string) []byte {
	t.Helper()
	body, _ := diskPaths(dir, rawURL)
	data, err := os.ReadFile(body) // #nosec G304 -- test path is a SHA-256 body inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStaleEntryRevalidatesWithValidatorsAndAccepts304(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 16, 16, color.NRGBA{R: 200, A: 255})
	var requests atomic.Int32
	var sawETag atomic.Bool
	var sawImageAccept atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("If-None-Match") == `"v1"` {
			sawETag.Store(true)
			w.Header().Set("Cache-Control", "max-age=3600")
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if strings.Contains(r.Header.Get("Accept"), "image/png") {
			sawImageAccept.Store(true)
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	rawURL := server.URL + "/revalidate.png"

	first := newTestCache(t, dir, Options{})
	_, _ = first.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "initial decode", func() bool { return hasDecoded(first, rawURL, 16) })
	expireDiskMetadata(t, dir, rawURL)

	second := newTestCache(t, dir, Options{})
	_, _ = second.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "revalidated decode", func() bool { return hasDecoded(second, rawURL, 16) })
	if got := requests.Load(); got != 2 {
		t.Fatalf("network requests = %d, want initial + revalidation", got)
	}
	if !sawETag.Load() {
		t.Fatal("revalidation did not send If-None-Match")
	}
	if !sawImageAccept.Load() {
		t.Fatal("image request did not send an image Accept header")
	}
	if !bytes.Equal(readCachedBody(t, dir, rawURL), picture) {
		t.Fatal("304 revalidation changed cached body")
	}
}

func TestStaleEntryIsReplacedOn200(t *testing.T) {
	t.Parallel()

	firstPicture := testPNG(t, 16, 16, color.NRGBA{R: 200, A: 255})
	secondPicture := testPNG(t, 16, 16, color.NRGBA{G: 200, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := requests.Add(1)
		if request == 1 {
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write(firstPicture)
			return
		}
		if r.Header.Get("If-None-Match") != `"v1"` {
			t.Errorf("If-None-Match = %q, want v1", r.Header.Get("If-None-Match"))
		}
		w.Header().Set("ETag", `"v2"`)
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write(secondPicture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	rawURL := server.URL + "/replace.png"

	first := newTestCache(t, dir, Options{})
	_, _ = first.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "initial decode", func() bool { return hasDecoded(first, rawURL, 16) })
	expireDiskMetadata(t, dir, rawURL)

	second := newTestCache(t, dir, Options{})
	_, _ = second.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "replacement decode", func() bool { return hasDecoded(second, rawURL, 16) })
	if got := requests.Load(); got != 2 {
		t.Fatalf("network requests = %d, want 2", got)
	}
	if !bytes.Equal(readCachedBody(t, dir, rawURL), secondPicture) {
		t.Fatal("disk body was not replaced by the 200 response")
	}
}

func TestStaleEntryIsUsedOnServerFailure(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 16, 16, color.NRGBA{B: 200, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write(picture)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	rawURL := server.URL + "/flaky.png"

	first := newTestCache(t, dir, Options{})
	_, _ = first.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "initial decode", func() bool { return hasDecoded(first, rawURL, 16) })
	expireDiskMetadata(t, dir, rawURL)

	second := newTestCache(t, dir, Options{})
	_, _ = second.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "stale fallback decode", func() bool { return hasDecoded(second, rawURL, 16) })
	if got := requests.Load(); got != 2 {
		t.Fatalf("network requests = %d, want 2", got)
	}
}

func TestCaptivePortalDoesNotReplaceUsableStaleDiskCopy(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 16, 16, color.NRGBA{R: 160, A: 255})
	var portal atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if portal.Load() {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body>captive portal</body></html>"))
			return
		}
		w.Header().Set("Cache-Control", "max-age=0")
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	rawURL := server.URL + "/portal.png"

	first := newTestCache(t, dir, Options{})
	_, _ = first.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "initial decode", func() bool { return hasDecoded(first, rawURL, 16) })
	portal.Store(true)

	second := newTestCache(t, dir, Options{})
	_, _ = second.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "stale image survives portal", func() bool { return hasDecoded(second, rawURL, 16) })
	if !bytes.Equal(readCachedBody(t, dir, rawURL), picture) {
		t.Fatal("captive portal body replaced the usable disk image")
	}
}

func TestTransportErrorAfterHeadersUsesStaleDiskCopy(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 16, 16, color.NRGBA{R: 110, A: 255})
	dir := t.TempDir()
	rawURL := "https://images.example.test/interrupted.png"
	if err := writeDiskFiles(dir, rawURL, picture, diskMetadata{
		ETag:       `"v1"`,
		FreshUntil: time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	transport := imagecacheRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Cache-Control": {"max-age=3600"}},
			Body:       &errorAfterByteReader{},
			Request:    request,
		}, nil
	})
	client, err := httpclient.New(httpclient.Options{
		Timeout:   2 * time.Second,
		Logger:    slog.New(slog.DiscardHandler),
		Transport: transport,
	})
	if err != nil {
		t.Fatal(err)
	}
	cache := newTestCache(t, dir, Options{Client: client})

	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "stale fallback after interrupted body", func() bool { return hasDecoded(cache, rawURL, 16) })
	if !bytes.Equal(readCachedBody(t, dir, rawURL), picture) {
		t.Fatal("interrupted response replaced the usable disk image")
	}
}

func TestNoStoreResponseRemovesDiskCopy(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 16, 16, color.NRGBA{R: 180, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			_, _ = w.Write(picture)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	rawURL := server.URL + "/private.png"

	first := newTestCache(t, dir, Options{})
	_, _ = first.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "initial decode", func() bool { return hasDecoded(first, rawURL, 16) })
	expireDiskMetadata(t, dir, rawURL)

	second := newTestCache(t, dir, Options{})
	_, _ = second.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "no-store decode", func() bool { return hasDecoded(second, rawURL, 16) })
	body, meta := diskPaths(dir, rawURL)
	if _, err := os.Stat(body); !os.IsNotExist(err) {
		t.Fatalf("no-store body remains or stat failed: %v", err)
	}
	if _, err := os.Stat(meta); !os.IsNotExist(err) {
		t.Fatalf("no-store metadata remains or stat failed: %v", err)
	}
}

func TestCorruptFreshDiskBodyIsRefetched(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 16, 16, color.NRGBA{R: 90, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		if r := requests.Load(); r != 1 {
			t.Errorf("unexpected request count %d", r)
		}
		_, _ = w.Write(picture)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	cache := newTestCache(t, dir, Options{})
	rawURL := server.URL + "/corrupt.png"
	if err := writeDiskFiles(dir, rawURL, []byte("not an image"), diskMetadata{FreshUntil: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	_, _ = cache.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "refetched decode", func() bool { return hasDecoded(cache, rawURL, 16) })
	if got := requests.Load(); got != 1 {
		t.Fatalf("network requests = %d, want 1", got)
	}
	if !bytes.Equal(readCachedBody(t, dir, rawURL), picture) {
		t.Fatal("corrupt disk body was not replaced")
	}
}

func TestMetadataFromHeaders(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name      string
		header    http.Header
		freshness time.Duration
		want      time.Time
	}{
		{name: "default", header: http.Header{}, freshness: time.Hour, want: now.Add(time.Hour)},
		{name: "max age", header: http.Header{"Cache-Control": {"public, max-age=30"}}, freshness: time.Hour, want: now.Add(30 * time.Second)},
		{name: "zero max age", header: http.Header{"Cache-Control": {"max-age=0"}}, freshness: time.Hour, want: now},
		{name: "no cache", header: http.Header{"Cache-Control": {"no-cache"}}, freshness: time.Hour, want: now},
		{name: "expires", header: http.Header{"Expires": {now.Add(2 * time.Hour).UTC().Format(http.TimeFormat)}}, freshness: time.Hour, want: now.Add(2 * time.Hour)},
		{name: "max age beats expires", header: http.Header{"Cache-Control": {"max-age=30"}, "Expires": {now.Add(2 * time.Hour).UTC().Format(http.TimeFormat)}}, freshness: time.Hour, want: now.Add(30 * time.Second)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := metadataFromHeaders(tt.header, tt.freshness, now)
			if !got.FreshUntil.Equal(tt.want) {
				t.Fatalf("FreshUntil = %s, want %s", got.FreshUntil, tt.want)
			}
		})
	}

	if !cacheNoStore(http.Header{"Cache-Control": {"public, no-store"}}) {
		t.Fatal("no-store was not recognized")
	}
	if cacheNoStore(http.Header{"Cache-Control": {"max-age=0"}}) {
		t.Fatal("ordinary cache-control recognized as no-store")
	}
}

func TestDecodeSafetyCapsAndSVG(t *testing.T) {
	t.Parallel()

	if _, _, err := decodeSafely(nil, 32); err == nil {
		t.Fatal("empty input decoded")
	}
	if _, _, err := decodeSafely(make([]byte, maxImageBytes+1), 32); !errors.Is(err, errImageTooLarge) {
		t.Fatalf("oversized input error = %v", err)
	}

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="8" height="8"><rect width="8" height="8" fill="red"/></svg>`)
	source, memory, err := decodeSafely(svg, 32)
	if err != nil || source == nil {
		t.Fatalf("SVG decode = %v, %v", source, err)
	}
	if memory != int64(len(svg)) {
		t.Fatalf("SVG memory = %d, want input size %d", memory, len(svg))
	}
}

func TestOversizedPixelDimensionsRejectedBeforeDecode(t *testing.T) {
	t.Parallel()

	data := pngHeaderOnly(8192, 8193) // 67,125,248 pixels, over the 16 Mi pixel cap.
	if _, _, err := decodeSafely(data, 32); !errors.Is(err, errImageTooManyPixels) {
		t.Fatalf("oversized dimensions error = %v", err)
	}
}

// pngHeaderOnly builds enough of a PNG for image.DecodeConfig to read its
// dimensions. It is never a complete image, so reaching the raster decoder in
// this test would fail rather than allocate the hostile dimensions.
func pngHeaderOnly(width, height int) []byte {
	var out bytes.Buffer
	out.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))  // #nosec G115 -- test dimensions are positive and far below MaxUint32.
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height)) // #nosec G115 -- test dimensions are positive and far below MaxUint32.
	ihdr[8] = 8                                           // bit depth
	ihdr[9] = 6                                           // RGBA
	appendUint32(&out, 13)
	out.WriteString("IHDR")
	out.Write(ihdr)
	crc := crc32.NewIEEE()
	_, _ = crc.Write([]byte("IHDR"))
	_, _ = crc.Write(ihdr)
	appendUint32(&out, crc.Sum32())
	appendUint32(&out, 0)
	out.WriteString("IEND")
	crc = crc32.NewIEEE()
	_, _ = crc.Write([]byte("IEND"))
	appendUint32(&out, crc.Sum32())
	return out.Bytes()
}

func appendUint32(out *bytes.Buffer, value uint32) {
	var raw [4]byte
	binary.BigEndian.PutUint32(raw[:], value)
	out.Write(raw[:])
}

type imagecacheRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn imagecacheRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type errorAfterByteReader struct {
	done bool
}

func (reader *errorAfterByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if reader.done {
		return 0, errors.New("imagecache test: interrupted response body")
	}
	reader.done = true
	p[0] = 'x'
	return 1, nil
}

func (reader *errorAfterByteReader) Close() error {
	return nil
}

func TestInvalidRemoteURLs(t *testing.T) {
	t.Parallel()

	cache := newTestCache(t, t.TempDir(), Options{})
	for _, rawURL := range []string{"", "relative.png", "ftp://example.com/a.png", "https:///missing-host"} {
		if _, err := cache.Load(nil, rawURL, LoadOptions{}); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("Load(%q) error = %v, want ErrInvalidURL", rawURL, err)
		}
		if err := cache.Forget(rawURL); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("Forget(%q) error = %v, want ErrInvalidURL", rawURL, err)
		}
	}
}

func TestStaleEntryIsUsedOffline(t *testing.T) {
	t.Parallel()

	picture := testPNG(t, 16, 16, color.NRGBA{R: 120, A: 255})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(picture)
	}))
	dir := t.TempDir()
	rawURL := server.URL + "/offline.png"

	first := newTestCache(t, dir, Options{})
	_, _ = first.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "initial decode", func() bool { return hasDecoded(first, rawURL, 16) })
	expireDiskMetadata(t, dir, rawURL)
	server.Close()

	second := newTestCache(t, dir, Options{})
	_, _ = second.Load(nil, rawURL, LoadOptions{MaxSide: 16})
	waitFor(t, "offline stale decode", func() bool { return hasDecoded(second, rawURL, 16) })
	if got := requests.Load(); got != 1 {
		t.Fatalf("network requests = %d, want only the initial request", got)
	}
}
