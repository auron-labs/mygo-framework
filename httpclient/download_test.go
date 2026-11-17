package httpclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/jobs"
)

type fileServer struct {
	content    []byte
	etag       string
	noRange    bool
	status416  bool
	badRange   bool
	truncateAt int // send this many bytes then drop the connection (first full response only)
	chunked    bool
	blockAfter int // send this many bytes then wait for the request context
	slow       time.Duration
	blocked    chan struct{} // closed once blockAfter bytes are flushed

	mu     sync.Mutex
	ranges []string
	ifs    []string
}

func (f *fileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	blockAfter, blocked := f.blockAfter, f.blocked
	f.ranges = append(f.ranges, r.Header.Get("Range"))
	f.ifs = append(f.ifs, r.Header.Get("If-Range"))
	f.mu.Unlock()

	if f.status416 && r.Header.Get("Range") != "" {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	body, start := f.content, 0
	w.Header().Set("ETag", f.etag)
	if rng := r.Header.Get("Range"); rng != "" && !f.noRange && r.Header.Get("If-Range") == f.etag {
		start, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rng, "bytes="), "-"))
		body = body[start:]
		first := start
		if f.badRange {
			first = 0
		}
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", first, len(f.content)-1, len(f.content)))
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(http.StatusPartialContent)
	} else if !f.chunked {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	send := body
	if f.truncateAt > 0 && start == 0 {
		send = body[:f.truncateAt]
	}
	if blockAfter > 0 {
		send = body[:blockAfter]
	}
	if f.slow > 0 {
		for _, b := range send {
			_, _ = w.Write([]byte{b})
			w.(http.Flusher).Flush()
			time.Sleep(f.slow)
		}
	} else {
		_, _ = w.Write(send)
	}
	w.(http.Flusher).Flush()
	if blockAfter > 0 {
		close(blocked)
		<-r.Context().Done()
		return
	}
	if len(send) < len(body) {
		panic(http.ErrAbortHandler)
	}
}

func (f *fileServer) block(after int) chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blockAfter, f.blocked = after, make(chan struct{})
	return f.blocked
}

func newFileServer(t *testing.T, size int) (*fileServer, *Client, string) {
	t.Helper()
	content := bytes.Repeat([]byte("0123456789abcdef"), size/16)
	f := &fileServer{content: content, etag: `"v1"`}
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	client, err := New(Options{BaseURL: server.URL, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	return f, client, filepath.Join(t.TempDir(), "sub", "file.bin")
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func seedPart(t *testing.T, dest string, data []byte, meta string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest+".part", data, 0o600); err != nil {
		t.Fatal(err)
	}
	if meta != "" {
		if err := os.WriteFile(dest+".part.json", []byte(meta), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func assertFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path) // #nosec G304 -- test temp path.
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("file %s mismatch (err %v, %d bytes, want %d)", path, err, len(got), len(want))
	}
}

func assertNoLeftovers(t *testing.T, dest string) {
	t.Helper()
	for _, p := range []string{dest + ".part", dest + ".part.json"} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s should not exist (err %v)", p, err)
		}
	}
}

func TestDownloadStreamsHashesAndReportsProgress(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 1<<20)
	var last [2]int64
	result, err := client.Download(t.Context(), "test.dl", "/f", dest, DownloadOptions{
		Progress:       func(done, total int64) { last = [2]int64{done, total} },
		ExpectedSHA256: strings.ToUpper(digest(f.content)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SHA256 != digest(f.content) || result.Size != int64(len(f.content)) || result.Resumed || result.Path != dest {
		t.Fatalf("result = %+v", result)
	}
	if last != [2]int64{int64(len(f.content)), int64(len(f.content))} {
		t.Fatalf("last progress = %v", last)
	}
	assertFile(t, dest, f.content)
	assertNoLeftovers(t, dest)
	if f.ranges[0] != "" {
		t.Fatalf("fresh download sent Range %q", f.ranges[0])
	}
}

func TestDownloadUnknownTotalReportsZero(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 4096)
	f.chunked = true
	var totals []int64
	if _, err := client.Download(t.Context(), "", "/f", dest, DownloadOptions{Progress: func(_, total int64) { totals = append(totals, total) }}); err != nil {
		t.Fatal(err)
	}
	if totals[0] != 0 {
		t.Fatalf("first total = %d, want 0", totals[0])
	}
	assertFile(t, dest, f.content)
}

func TestDownloadReplacesExistingAndKeepsItOnFailure(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 1024)
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	var re *ResponseError
	if _, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: strings.Repeat("0", 64)}); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v", err)
	}
	assertFile(t, dest, []byte("old"))
	assertNoLeftovers(t, dest)

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "secret body", http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)
	bad, _ := New(Options{BaseURL: failing.URL, Logger: slog.New(slog.DiscardHandler)})
	_, err := bad.Download(t.Context(), "t", "/f", dest, DownloadOptions{})
	if !errors.As(err, &re) || re.StatusCode != http.StatusInternalServerError || strings.Contains(err.Error(), "secret") {
		t.Fatalf("err = %v", err)
	}
	assertFile(t, dest, []byte("old"))

	if _, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, dest, f.content)
}

func TestDownloadCancelKeepsPartialFileForResume(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 4096)
	blocked := f.block(1000)
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-blocked
		for { // Cancel only once the client has stored the flushed bytes.
			if info, err := os.Stat(dest + ".part"); err == nil && info.Size() >= 1000 {
				cancel()
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()
	_, err := client.Download(ctx, "t", "/f", dest, DownloadOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	info, statErr := os.Stat(dest + ".part")
	if statErr != nil || info.Size() == 0 {
		t.Fatalf("part file: %v", statErr)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dest must not exist after cancel")
	}

	f.block(0)
	result, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: digest(f.content)})
	if err != nil || !result.Resumed {
		t.Fatalf("resume = %+v, %v", result, err)
	}
	assertFile(t, dest, f.content)
}

func TestDownloadResumeSendsRangeAndIfRange(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 2048)
	seedPart(t, dest, f.content[:700], `{"etag":"\"v1\"","total":2048}`)
	result, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: digest(f.content)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Resumed || result.SHA256 != digest(f.content) {
		t.Fatalf("result = %+v", result)
	}
	if f.ranges[0] != "bytes=700-" || f.ifs[0] != `"v1"` {
		t.Fatalf("Range %q If-Range %q", f.ranges[0], f.ifs[0])
	}
	assertFile(t, dest, f.content)
	assertNoLeftovers(t, dest)
}

func TestDownloadRestartsCleanly(t *testing.T) {
	t.Parallel()
	garbage := bytes.Repeat([]byte("x"), 700)
	tests := []struct {
		name  string
		setup func(*fileServer)
		meta  string
	}{
		{"server ignores range", func(f *fileServer) { f.noRange = true }, `{"etag":"\"v1\""}`},
		{"validator changed", func(f *fileServer) { f.etag = `"v2"` }, `{"etag":"\"v1\""}`},
		{"416", func(f *fileServer) { f.status416 = true }, `{"etag":"\"v1\"","total":9999}`},
		{"bad content range", func(f *fileServer) { f.badRange = true }, `{"etag":"\"v1\""}`},
		{"no sidecar", func(*fileServer) {}, ``},
		{"weak etag", func(*fileServer) {}, `{"etag":"W/\"v1\""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, client, dest := newFileServer(t, 2048)
			tt.setup(f)
			seedPart(t, dest, garbage, tt.meta)
			result, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: digest(f.content)})
			if err != nil {
				t.Fatal(err)
			}
			if result.Resumed {
				t.Fatal("restart reported as resumed")
			}
			assertFile(t, dest, f.content)
			assertNoLeftovers(t, dest)
		})
	}
}

func TestDownload416WithCompletePartFinishes(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 1024)
	f.status416 = true
	seedPart(t, dest, f.content, `{"etag":"\"v1\"","total":1024}`)
	result, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: digest(f.content)})
	if err != nil || !result.Resumed {
		t.Fatalf("result = %+v, %v", result, err)
	}
	assertFile(t, dest, f.content)
}

func TestDownloadChecksumMismatchDiscardsPartial(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 1024)
	_, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: strings.Repeat("ab", 32)})
	if !errors.Is(err, ErrChecksumMismatch) || !strings.Contains(err.Error(), digest(f.content)) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(dest); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dest exists")
	}
	assertNoLeftovers(t, dest)

	if _, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: "nothex"}); err == nil {
		t.Fatal("invalid digest accepted")
	}
}

func TestDownloadTruncatedBodyKeepsPartThenResumes(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 4096)
	f.truncateAt = 1500
	if _, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{}); err == nil {
		t.Fatal("truncated download succeeded")
	}
	info, err := os.Stat(dest + ".part")
	if err != nil || info.Size() != 1500 {
		t.Fatalf("part = %v, %v", info, err)
	}
	result, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{ExpectedSHA256: digest(f.content)})
	if err != nil || !result.Resumed {
		t.Fatalf("result = %+v, %v", result, err)
	}
	assertFile(t, dest, f.content)
}

func TestDownloadIgnoresClientTimeout(t *testing.T) {
	t.Parallel()
	f, _, dest := newFileServer(t, 64)
	f.slow = 10 * time.Millisecond
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	client, err := New(Options{BaseURL: server.URL, Timeout: 100 * time.Millisecond, Logger: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Download(t.Context(), "t", "/f", dest, DownloadOptions{}); err != nil {
		t.Fatal(err)
	}
	assertFile(t, dest, f.content)
}

func TestDownloadRejectsBadInput(t *testing.T) {
	t.Parallel()
	_, client, dest := newFileServer(t, 16)
	if _, err := client.Download(t.Context(), "t", "/f", " ", DownloadOptions{}); err == nil {
		t.Fatal("empty dest accepted")
	}
	if _, err := client.Download(t.Context(), "t", "file:///etc/passwd", dest, DownloadOptions{}); err == nil {
		t.Fatal("file URL accepted")
	}
}

func TestJobProgress(t *testing.T) {
	t.Parallel()
	var done, total int
	var msg string
	report := JobProgress(func(d, t int, m string) { done, total, msg = d, t, m }, "Downloading")
	report(5, 10)
	if done != 5 || total != 10 || msg != "Downloading" {
		t.Fatalf("got %d %d %q", done, total, msg)
	}
	report(5, -1)
	if total != 0 {
		t.Fatalf("unknown total = %d", total)
	}
}

func TestDownloadAsJob(t *testing.T) {
	t.Parallel()
	f, client, dest := newFileServer(t, 8192)
	manager := jobs.New(t.Context())
	finished := make(chan jobs.Status, 1)
	var last jobs.Status
	manager.Subscribe(func(s jobs.Status) {
		if s.State == jobs.Running {
			last = s
			return
		}
		finished <- s
	})
	err := manager.Start("dl", func(ctx context.Context, report jobs.Report) error {
		_, err := client.Download(ctx, "t", "/f", dest, DownloadOptions{Progress: JobProgress(report, "Downloading")})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if s := <-finished; s.State != jobs.Done || s.Done != len(f.content) || s.Total != len(f.content) || s.Fraction() != 1 || last.Message != "Downloading" {
		t.Fatalf("status = %+v (last running %+v)", s, last)
	}

	// A canceled job ends as Canceled and leaves the partial file.
	blocked := f.block(1000)
	dest2 := dest + "2"
	err = manager.Start("dl2", func(ctx context.Context, report jobs.Report) error {
		_, err := client.Download(ctx, "t", "/f", dest2, DownloadOptions{Progress: JobProgress(report, "Downloading")})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	<-blocked
	for {
		if info, err := os.Stat(dest2 + ".part"); err == nil && info.Size() >= 1000 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	manager.Cancel("dl2")
	if s := <-finished; s.State != jobs.Canceled {
		t.Fatalf("status = %+v", s)
	}
}
