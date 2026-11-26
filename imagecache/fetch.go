package imagecache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/auron-labs/mygo-framework/v2/httpclient"

	"github.com/go-resty/resty/v2"
)

const imageAccept = "image/png,image/jpeg,image/gif,image/webp,image/bmp,image/svg+xml,image/*;q=0.5,*/*;q=0.1"

var errImageNetwork = errors.New("imagecache: image request failed")

// fetched is the byte-level result shared by concurrent size-specific decode
// requests for one URL.
type fetched struct {
	url      string
	data     []byte
	fallback []byte
	fromDisk bool
	store    bool
	metadata diskMetadata
	token    diskToken
}

// fetchBytes deduplicates byte fetches by URL across concurrent entries that
// request different MaxSide values. A corrupt-disk reload uses a separate key
// so it cannot join the flight that returned the corrupt disk bytes.
func (c *Cache) fetchBytes(ctx context.Context, e *entry, ignoreDisk bool) (fetched, error) {
	key := e.key.url
	if ignoreDisk {
		key += "\x00reload"
	}
	token := c.diskToken(e.key.url)
	value, err, _ := c.fetchGroup.Do(key, func() (any, error) {
		return c.fetchBytesOnce(ctx, e.key.url, ignoreDisk, token)
	})
	if err != nil {
		return fetched{}, fmt.Errorf("imagecache: fetch flight: %w", err)
	}
	result, _ := value.(fetched)
	return result, nil
}

func (c *Cache) fetchBytesOnce(ctx context.Context, rawURL string, ignoreDisk bool, token diskToken) (fetched, error) {
	var cached []byte
	var cachedMeta diskMetadata
	if !ignoreDisk {
		if data, meta, ok := readDiskFiles(c.dir, rawURL, maxImageBytes); ok {
			cached, cachedMeta = data, meta
			if c.now().Before(meta.FreshUntil) {
				return fetched{url: rawURL, data: data, fromDisk: true, token: token}, nil
			}
		}
	}

	validator := httpclient.Validator{ETag: cachedMeta.ETag, LastModified: cachedMeta.LastModified}
	if ignoreDisk {
		validator = httpclient.Validator{}
	}
	request := c.client.ConditionalRequest(ctx, "imagecache.fetch", validator).
		SetHeader("Accept", imageAccept).
		SetResponseBodyLimit(int(maxImageBytes))
	response, requestErr := request.Get(rawURL)

	if response != nil && response.RawResponse != nil {
		status := response.StatusCode()
		switch {
		case status == http.StatusNotModified && len(cached) > 0:
			next := metadataFromHeaders(response.Header(), c.defaultFreshness, c.now())
			if next.ETag == "" {
				next.ETag = cachedMeta.ETag
			}
			if next.LastModified == "" {
				next.LastModified = cachedMeta.LastModified
			}
			writeDiskMetadataIfCurrent(c, rawURL, token, next)
			return fetched{url: rawURL, data: cached, fromDisk: true, token: token}, nil

		case status == http.StatusOK:
			if requestErr != nil {
				return fetched{}, sanitizeFetchError(requestErr)
			}
			data := response.Body()
			if len(data) == 0 {
				return fetched{}, errImageEmpty
			}
			if int64(len(data)) > maxImageBytes {
				return fetched{}, errImageTooLarge
			}
			if cacheNoStore(response.Header()) {
				// no-store applies to the disk cache. A stale body already read
				// into memory can still be used as a fallback for this response,
				// but it is not left on disk after the server's instruction.
				c.removeDiskFilesIfCurrent(rawURL, token)
				return fetched{url: rawURL, data: data, fallback: cached, token: token}, nil
			}
			return fetched{
				url:      rawURL,
				data:     data,
				fallback: cached,
				store:    true,
				metadata: metadataFromHeaders(response.Header(), c.defaultFreshness, c.now()),
				token:    token,
			}, nil

		case status >= http.StatusInternalServerError && len(cached) > 0:
			return fetched{url: rawURL, data: cached, fromDisk: true, token: token}, nil
		default:
			return fetched{}, fmt.Errorf("imagecache: server returned HTTP %d", status)
		}
	}

	if requestErr != nil {
		if len(cached) > 0 {
			// Offline or otherwise unreachable: stale bytes are better than an
			// empty avatar in a list the user already has.
			return fetched{url: rawURL, data: cached, fromDisk: true, token: token}, nil
		}
		return fetched{}, sanitizeFetchError(requestErr)
	}
	return fetched{}, errImageNetwork
}

func sanitizeFetchError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, resty.ErrResponseBodyTooLarge) {
		return errImageTooLarge
	}
	return errImageNetwork
}

// metadataFromHeaders reads Cache-Control max-age/no-cache before Expires and
// otherwise applies the cache's default freshness.
func metadataFromHeaders(header http.Header, defaultFreshness time.Duration, now time.Time) diskMetadata {
	meta := diskMetadata{
		ETag:         header.Get("ETag"),
		LastModified: header.Get("Last-Modified"),
	}
	freshness := defaultFreshness
	found := false
	noCache := false
	for _, directive := range cacheControlDirectives(header) {
		if directive == "no-cache" {
			noCache = true
			continue
		}
		if value, ok := strings.CutPrefix(directive, "max-age="); ok {
			value = strings.Trim(value, `"`)
			if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
				if seconds < 0 {
					seconds = 0
				}
				freshness, found = time.Duration(seconds)*time.Second, true
			}
		}
	}
	if noCache {
		freshness, found = 0, true
	}
	if !found {
		if expires, err := http.ParseTime(header.Get("Expires")); err == nil {
			meta.FreshUntil = expires
			return meta
		}
	}
	meta.FreshUntil = now.Add(freshness)
	return meta
}

func cacheNoStore(header http.Header) bool {
	for _, directive := range cacheControlDirectives(header) {
		if directive == "no-store" {
			return true
		}
	}
	return false
}

func cacheControlDirectives(header http.Header) []string {
	var directives []string
	for _, value := range header.Values("Cache-Control") {
		for part := range strings.SplitSeq(value, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part != "" {
				directives = append(directives, part)
			}
		}
	}
	return directives
}
