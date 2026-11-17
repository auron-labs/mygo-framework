package httpclient

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// ErrChecksumMismatch is returned when a download's SHA-256 digest differs from
// DownloadOptions.ExpectedSHA256. The partial file is discarded.
var ErrChecksumMismatch = errors.New("download checksum mismatch")

const (
	partSuffix         = ".part"
	metaSuffix         = ".part.json"
	progressInterval   = 100 * time.Millisecond
	downloadBufferSize = 64 << 10
)

// DownloadOptions configure Client.Download.
type DownloadOptions struct {
	// Progress is called with the bytes written so far and the total size. A
	// total of zero or less means the total is unknown. Calls are throttled and
	// the last call reports the finished size.
	Progress func(done, total int64)
	// ExpectedSHA256 is the hex digest the file must have; empty skips the check.
	ExpectedSHA256 string
}

// DownloadResult describes a finished download.
type DownloadResult struct {
	Path string
	Size int64
	// SHA256 is the lowercase hex digest of the whole file.
	SHA256 string
	// Resumed reports that an earlier partial file was continued.
	Resumed bool
}

type downloadMeta struct {
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Total        int64  `json:"total,omitempty"`
}

// validator returns the value for If-Range, or "" when the partial file cannot
// be safely continued. Weak ETags are not valid for range requests.
func (m downloadMeta) validator() string {
	if m.ETag != "" && !strings.HasPrefix(m.ETag, "W/") {
		return m.ETag
	}
	return m.LastModified
}

// JobProgress adapts a jobs.Report to DownloadOptions.Progress. It is declared
// structurally so httpclient does not import jobs.
func JobProgress(report func(done, total int, message string), label string) func(done, total int64) {
	return func(done, total int64) {
		report(clampInt(done), clampInt(max(total, 0)), label)
	}
}

func clampInt(value int64) int {
	return int(min(value, math.MaxInt))
}

// Download streams url into dest. Data goes to dest+".part" and moves to dest
// only after the whole body arrived and, if requested, matched the expected
// digest. When ctx is canceled or the connection fails the partial file stays,
// and the next call continues it with a Range request if the server's
// validator (ETag or Last-Modified) is unchanged; otherwise it starts over.
// Do not download to the same dest concurrently.
func (c *Client) Download(ctx context.Context, operation, rawURL, dest string, options DownloadOptions) (DownloadResult, error) {
	if strings.TrimSpace(dest) == "" {
		return DownloadResult{}, errors.New("download destination must not be empty")
	}
	if strings.TrimSpace(operation) == "" {
		operation = "http.download"
	}
	if parsed, err := url.Parse(rawURL); err != nil || (parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https") {
		return DownloadResult{}, fmt.Errorf("%s: unsupported download URL", operation)
	}
	expected, err := parseExpectedDigest(options.ExpectedSHA256)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%s: %w", operation, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return DownloadResult{}, fmt.Errorf("%s: create download directory: %w", operation, err)
	}

	d := &download{client: c, operation: operation, url: rawURL, dest: dest, part: dest + partSuffix, meta: dest + metaSuffix, options: options}
	result, err := d.run(ctx, expected)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("%s: %w", operation, err)
	}
	return result, nil
}

func parseExpectedDigest(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	digest, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil || len(digest) != sha256.Size {
		return nil, errors.New("expected SHA-256 must be 64 hex characters")
	}
	return digest, nil
}

type download struct {
	client    *Client
	operation string
	url       string
	dest      string
	part      string
	meta      string
	options   DownloadOptions
}

func (d *download) run(ctx context.Context, expected []byte) (DownloadResult, error) {
	offset, validator := d.resumePoint()
	for {
		response, err := d.request(ctx, offset, validator)
		if err != nil {
			return DownloadResult{}, err
		}
		status := response.RawResponse.StatusCode
		body := response.RawResponse.Body

		switch {
		case status == http.StatusRequestedRangeNotSatisfiable && offset > 0:
			_ = body.Close()
			if known := d.loadMeta().Total; known > 0 && known == offset {
				return d.finish(offset, true, expected)
			}
			offset = 0
			continue
		case status >= http.StatusBadRequest:
			_ = body.Close()
			return DownloadResult{}, &ResponseError{
				Operation:  d.operation,
				Method:     http.MethodGet,
				Host:       host(response.Request),
				StatusCode: status,
			}
		case offset > 0 && status == http.StatusPartialContent:
			if start, total, ok := parseContentRange(response.Header().Get("Content-Range")); ok && start == offset {
				return d.stream(ctx, response, offset, total, true, expected)
			}
			_ = body.Close()
			offset = 0 // The server answered with a different range: start over.
			continue
		case offset > 0 && status != http.StatusOK:
			_ = body.Close()
			offset = 0
			continue
		}
		// A plain 200 (also when a resume was requested): the whole body follows.
		total := max(response.RawResponse.ContentLength, 0)
		return d.stream(ctx, response, 0, total, false, expected)
	}
}

// resumePoint returns the size of a continuable partial file and its validator.
func (d *download) resumePoint() (int64, string) {
	info, err := os.Stat(d.part)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return 0, ""
	}
	validator := d.loadMeta().validator()
	if validator == "" {
		return 0, ""
	}
	return info.Size(), validator
}

func (d *download) loadMeta() downloadMeta {
	var meta downloadMeta
	data, err := os.ReadFile(d.meta) // #nosec G304 -- derived from the caller's destination path.
	if err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	return meta
}

func (d *download) request(ctx context.Context, offset int64, validator string) (*resty.Response, error) {
	request := d.client.download.R().
		SetContext(context.WithValue(ctx, operationKey{}, d.operation)).
		SetDoNotParseResponse(true).
		SetHeader("Accept", "*/*").
		SetHeader("Accept-Encoding", "identity") // Compressed bodies would break range offsets.
	if offset > 0 {
		request.SetHeader("Range", "bytes="+strconv.FormatInt(offset, 10)+"-").SetHeader("If-Range", validator)
	}
	response, err := request.Get(d.url)
	if err != nil {
		if response != nil && response.RawResponse != nil {
			_ = response.RawResponse.Body.Close()
		}
		return nil, fmt.Errorf("request: %w", err)
	}
	d.client.record(response.Request, response.StatusCode(), response.Time()) //nolint:contextcheck // The request carries ctx.
	return response, nil
}

func (d *download) stream(ctx context.Context, response *resty.Response, offset, total int64, resumed bool, expected []byte) (DownloadResult, error) {
	body := response.RawResponse.Body
	defer func() { _ = body.Close() }()

	sum := sha256.New()
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if resumed {
		if err := hashPrefix(sum, d.part, offset); err != nil {
			// The partial file is unreadable or shorter than expected; restart.
			_ = os.Remove(d.part)
			return d.run(ctx, expected)
		}
		flags = os.O_WRONLY | os.O_APPEND
	}
	header := response.Header()
	meta := downloadMeta{ETag: header.Get("ETag"), LastModified: header.Get("Last-Modified"), Total: total}
	if err := d.saveMeta(meta); err != nil {
		return DownloadResult{}, err
	}
	file, err := os.OpenFile(d.part, flags, 0o600) // #nosec G304 -- derived from the caller's destination path.
	if err != nil {
		return DownloadResult{}, fmt.Errorf("open partial file: %w", err)
	}

	done, err := d.copy(ctx, io.MultiWriter(file, sum), body, offset, total)
	if err != nil {
		_ = file.Close()
		return DownloadResult{}, err
	}
	if total > 0 && done != total {
		_ = file.Close()
		return DownloadResult{}, fmt.Errorf("download ended at %d of %d bytes: %w", done, total, io.ErrUnexpectedEOF)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return DownloadResult{}, fmt.Errorf("sync partial file: %w", err)
	}
	if err := file.Close(); err != nil {
		return DownloadResult{}, fmt.Errorf("close partial file: %w", err)
	}
	return d.complete(sum, done, resumed, expected)
}

// finish completes a download whose partial file already holds the full body.
func (d *download) finish(size int64, resumed bool, expected []byte) (DownloadResult, error) {
	sum := sha256.New()
	if err := hashPrefix(sum, d.part, size); err != nil {
		_ = os.Remove(d.part)
		return DownloadResult{}, fmt.Errorf("hash partial file: %w", err)
	}
	return d.complete(sum, size, resumed, expected)
}

func (d *download) complete(sum hash.Hash, size int64, resumed bool, expected []byte) (DownloadResult, error) {
	digest := sum.Sum(nil)
	if expected != nil && subtle.ConstantTimeCompare(digest, expected) != 1 {
		d.discard()
		return DownloadResult{}, fmt.Errorf("%w: got %s", ErrChecksumMismatch, hex.EncodeToString(digest))
	}
	if err := os.Rename(d.part, d.dest); err != nil {
		return DownloadResult{}, fmt.Errorf("move download into place: %w", err)
	}
	_ = os.Remove(d.meta)
	if d.options.Progress != nil {
		d.options.Progress(size, size)
	}
	return DownloadResult{Path: d.dest, Size: size, SHA256: hex.EncodeToString(digest), Resumed: resumed}, nil
}

func (d *download) discard() {
	_ = os.Remove(d.part)
	_ = os.Remove(d.meta)
}

func (d *download) saveMeta(meta downloadMeta) error {
	if meta.validator() == "" {
		_ = os.Remove(d.meta) // Nothing safe to resume against.
		return nil
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode download state: %w", err)
	}
	if err := os.WriteFile(d.meta, data, 0o600); err != nil {
		return fmt.Errorf("save download state: %w", err)
	}
	return nil
}

// copy writes body to w, reporting throttled progress, and returns the total
// bytes held (including offset).
func (d *download) copy(ctx context.Context, w io.Writer, body io.Reader, offset, total int64) (int64, error) {
	done := offset
	buffer := make([]byte, downloadBufferSize)
	last := time.Now()
	report := func(force bool) {
		if d.options.Progress == nil {
			return
		}
		if force || time.Since(last) >= progressInterval {
			last = time.Now()
			d.options.Progress(done, total)
		}
	}
	report(true)
	for {
		n, readErr := body.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return done, fmt.Errorf("write partial file: %w", err)
			}
			done += int64(n)
			report(false)
		}
		if readErr == io.EOF {
			return done, nil
		}
		if readErr != nil {
			if err := ctx.Err(); err != nil {
				return done, fmt.Errorf("download stopped: %w", err)
			}
			return done, fmt.Errorf("read body: %w", readErr)
		}
		if err := ctx.Err(); err != nil {
			return done, fmt.Errorf("download stopped: %w", err)
		}
	}
}

// hashPrefix feeds the first size bytes of path into sum and fails when the
// file is shorter.
func hashPrefix(sum hash.Hash, path string, size int64) error {
	file, err := os.Open(path) // #nosec G304 -- derived from the caller's destination path.
	if err != nil {
		return fmt.Errorf("open partial file: %w", err)
	}
	defer func() { _ = file.Close() }()
	n, err := io.Copy(sum, io.LimitReader(file, size))
	if err != nil {
		return fmt.Errorf("read partial file: %w", err)
	}
	if n != size {
		return io.ErrUnexpectedEOF
	}
	return nil
}

// parseContentRange reads "bytes start-end/total"; total is 0 when unknown.
func parseContentRange(value string) (start, total int64, ok bool) {
	rest, found := strings.CutPrefix(value, "bytes ")
	if !found {
		return 0, 0, false
	}
	span, size, found := strings.Cut(rest, "/")
	if !found {
		return 0, 0, false
	}
	first, _, found := strings.Cut(span, "-")
	if !found {
		return 0, 0, false
	}
	start, err := strconv.ParseInt(strings.TrimSpace(first), 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false
	}
	if size != "*" {
		if total, err = strconv.ParseInt(strings.TrimSpace(size), 10, 64); err != nil {
			return 0, 0, false
		}
	}
	return start, total, true
}
