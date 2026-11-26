// Package imagecache loads remote images for native views without doing
// network, disk, or decode work in a view builder.
//
// A Cache owns a bounded on-disk byte cache, a bounded in-memory LRU of decoded
// images, and a small worker pool. Load and Image are safe to call while
// building a view: they return immediately with nil or a placeholder and
// invalidate the view when the image is ready.
package imagecache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/auron-labs/mygo-framework/v2/httpclient"

	"github.com/egoist/mygo/ui"
)

const (
	defaultMemoryEntries = 256
	defaultMemoryBytes   = 128 << 20
	defaultDiskBytes     = 256 << 20
	defaultFreshness     = 7 * 24 * time.Hour
	defaultMaxSide       = 2048
)

var (
	// ErrInvalidURL is returned for a URL that is not absolute HTTP or HTTPS.
	ErrInvalidURL = errors.New("imagecache: URL must be absolute HTTP or HTTPS")
	// ErrShutdown is returned when work is requested after Shutdown.
	ErrShutdown = errors.New("imagecache: cache is shut down")
)

// Options configure a Cache. Client and Dir are required; the directory must
// be dedicated to this cache because entries are created and pruned inside it.
type Options struct {
	Client *httpclient.Client
	Dir    string

	// MemoryEntries bounds the number of decoded images retained. Zero uses 256.
	MemoryEntries int
	// MemoryBytes bounds decoded bitmap memory, in bytes. Zero uses 128 MiB.
	MemoryBytes int64
	// DiskBytes bounds cache-owned files on disk, in bytes. Zero uses 256 MiB.
	DiskBytes int64
	// DefaultFreshness is how long a response is trusted when it sends no
	// Cache-Control or Expires freshness. Zero uses seven days.
	DefaultFreshness time.Duration
}

// LoadOptions customize one decoded image.
type LoadOptions struct {
	// MaxSide bounds the longest side of a decoded raster image, in pixels.
	// It does not enlarge smaller images. Zero uses 2048. Choose about two or
	// three times the DIP size at which the image is displayed, matching the
	// device pixels per DIP; a 40-DIP avatar normally needs 80-120, not 2048.
	MaxSide int
}

// ImageOptions customize an Image element.
type ImageOptions struct {
	// MaxSide bounds the longest side of a decoded raster image, in pixels.
	// Choose about two or three times the DIP display size. Zero uses 2048.
	MaxSide int
	// Fit says how the image fills its element. The zero value is ui.Contain.
	Fit ui.Fit
	// Placeholder fills the element while the image loads and after failure.
	// The zero value uses the theme's border color.
	Placeholder ui.Color
}

// New creates a cache whose workers stop when parent is canceled or Shutdown
// is called. The directory is created with private permissions when needed.
func New(parent context.Context, options Options) (*Cache, error) {
	return newCache(parent, options, workerCount)
}

func newCache(parent context.Context, options Options, workers int) (*Cache, error) {
	if parent == nil {
		return nil, errors.New("imagecache: parent context is required")
	}
	if options.Client == nil {
		return nil, errors.New("imagecache: HTTP client is required")
	}
	if options.Dir == "" {
		return nil, errors.New("imagecache: cache directory is required")
	}
	if options.MemoryEntries < 0 || options.MemoryBytes < 0 || options.DiskBytes < 0 || options.DefaultFreshness < 0 {
		return nil, errors.New("imagecache: cache limits must not be negative")
	}
	if workers < 1 {
		return nil, errors.New("imagecache: worker count must be positive")
	}

	dir, err := filepath.Abs(options.Dir)
	if err != nil {
		return nil, fmt.Errorf("imagecache: resolve cache directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("imagecache: create cache directory: %w", err)
	}

	entries := options.MemoryEntries
	if entries == 0 {
		entries = defaultMemoryEntries
	}
	memoryBytes := options.MemoryBytes
	if memoryBytes == 0 {
		memoryBytes = defaultMemoryBytes
	}
	diskBytes := options.DiskBytes
	if diskBytes == 0 {
		diskBytes = defaultDiskBytes
	}
	freshness := options.DefaultFreshness
	if freshness == 0 {
		freshness = defaultFreshness
	}

	cache := newCacheState(parent, options.Client, dir, entries, memoryBytes, diskBytes, freshness)
	for range workers {
		cache.workers.Add(1)
		go cache.worker()
	}
	return cache, nil
}

// Load returns the decoded image at url, or nil while it is loading or after a
// failure. It never blocks the caller: a miss is queued for a worker and the
// supplied UI context is invalidated when the worker finishes. The error is
// the most recent failure while it is negative-cached; keep rendering a
// placeholder in that case. The exact URL string is the cache identity.
//
// uiContext may be nil, in which case no invalidation waiter is registered.
// Only Context.Invalidate is ever called off the UI thread.
func (c *Cache) Load(uiContext *ui.Context, rawURL string, options LoadOptions) (ui.ImageSource, error) {
	return c.load(uiContext, rawURL, effectiveMaxSide(options.MaxSide))
}

// Image returns an element for the remote image at url: a theme-colored
// placeholder while loading or after failure, and a ui.Image once decoded.
// It is safe to call from a native view builder.
func (c *Cache) Image(uiContext *ui.Context, rawURL string, options ImageOptions) ui.Element {
	source, _ := c.Load(uiContext, rawURL, LoadOptions{MaxSide: options.MaxSide})
	if uiContext == nil {
		return ui.Element{}
	}
	if source == nil {
		fill := options.Placeholder
		if fill == (ui.Color{}) {
			fill = uiContext.Theme().Border
		}
		return ui.Box(uiContext).Background(fill).Role(ui.RoleImage)
	}
	return ui.Image(uiContext, source).Fit(options.Fit).Role(ui.RoleImage)
}

// Shutdown cancels queued and in-flight work, then waits for workers. It is
// idempotent. Disk entries already written remain for the next cache instance.
func (c *Cache) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("imagecache: shutdown context is required")
	}
	c.shutdownOnce.Do(func() {
		c.mu.Lock()
		c.stopped = true
		if c.retryTimer != nil {
			c.retryTimer.Stop()
		}
		c.retryAt = time.Time{}
		c.mu.Unlock()
		c.cancel()
	})

	finished := make(chan struct{})
	go func() {
		c.workers.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("imagecache: shutdown: %w", ctx.Err())
	}
}

func effectiveMaxSide(maxSide int) int {
	if maxSide <= 0 {
		return defaultMaxSide
	}
	return maxSide
}
