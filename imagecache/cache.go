package imagecache

import (
	"container/list"
	"context"
	"net/url"
	"slices"
	"sync"
	"time"

	"github.com/auron-labs/mygo-framework/v2/httpclient"

	"github.com/egoist/mygo/ui"
	"golang.org/x/sync/singleflight"
)

const (
	workerCount          = 6
	maxConcurrentDecodes = 2
	abandonedAfter       = 2 * time.Second
	recentUseProtection  = 2 * time.Second
	failureRetryAfter    = 10 * time.Second
	maxTrackedEntries    = 2048
)

// cacheKey identifies one decoded representation. Different MaxSide values of
// one URL are separate memory entries, while byte fetches deduplicate by URL.
type cacheKey struct {
	url     string
	maxSide int
}

type entry struct {
	key cacheKey

	source      ui.ImageSource
	memoryBytes int64
	err         error

	used   time.Time
	came   time.Time
	failed time.Time

	loading   bool
	queued    bool
	forgotten bool
	waiters   []*ui.Context
	lru       *list.Element
}

type diskToken struct {
	clear uint64
	url   uint64
}

// Cache owns the workers, memory entries, disk entries, and invalidation
// waiters for one application-configured remote image cache.
type Cache struct {
	client           *httpclient.Client
	dir              string
	memoryEntryLimit int
	memoryByteLimit  int64
	diskByteLimit    int64
	defaultFreshness time.Duration

	ctx    context.Context //nolint:containedctx // The cache lifetime parents every fetch.
	cancel context.CancelFunc

	shutdownOnce sync.Once
	workers      sync.WaitGroup
	fetchGroup   singleflight.Group

	// diskMu serializes files written or removed by cache workers with Forget
	// and Clear. Cache mutex state is checked again while diskMu is held so a
	// privacy deletion cannot be followed by a stale in-flight write. diskIndex
	// is valid only while diskMu is held.
	diskMu    sync.Mutex
	diskIndex diskIndex

	// decodeSlots bounds simultaneous full-size raster decodes. Compressed
	// input and pixel caps still apply to every image; this limit prevents six
	// workers from multiplying the transient decode memory spike.
	decodeSlots chan struct{}

	mu          sync.Mutex
	queueCond   *sync.Cond
	stopped     bool
	entries     map[cacheKey]*entry
	queue       []*entry
	lruList     *list.List
	memoryBytes int64
	clearEpoch  uint64
	urlEpochs   map[string]uint64
	retryTimer  *time.Timer
	retryAt     time.Time

	// These durations are fields rather than constants so focused tests can
	// shorten them before the first request.
	abandonAfter time.Duration
	protectFor   time.Duration
	retryAfter   time.Duration
	now          func() time.Time
}

func newCacheState(parent context.Context, client *httpclient.Client, dir string, memoryEntries int, memoryBytes, diskBytes int64, freshness time.Duration) *Cache {
	ctx, cancel := context.WithCancel(parent)
	cache := &Cache{
		client:           client,
		dir:              dir,
		memoryEntryLimit: memoryEntries,
		memoryByteLimit:  memoryBytes,
		diskByteLimit:    diskBytes,
		defaultFreshness: freshness,
		ctx:              ctx,
		cancel:           cancel,
		entries:          map[cacheKey]*entry{},
		lruList:          list.New(),
		urlEpochs:        map[string]uint64{},
		decodeSlots:      make(chan struct{}, maxConcurrentDecodes),
		abandonAfter:     abandonedAfter,
		protectFor:       recentUseProtection,
		retryAfter:       failureRetryAfter,
		now:              time.Now,
	}
	cache.queueCond = sync.NewCond(&cache.mu)
	context.AfterFunc(ctx, func() {
		cache.mu.Lock()
		cache.queueCond.Broadcast()
		cache.mu.Unlock()
	})
	return cache
}

func (c *Cache) load(uiContext *ui.Context, rawURL string, maxSide int) (ui.ImageSource, error) {
	if err := validateRemoteURL(rawURL); err != nil {
		return nil, err
	}
	key := cacheKey{url: rawURL, maxSide: maxSide}
	now := c.now()

	c.mu.Lock()
	if c.stopped || c.ctx.Err() != nil {
		c.mu.Unlock()
		return nil, ErrShutdown
	}
	e := c.entries[key]
	if e == nil {
		c.sweepLocked(now)
		e = &entry{key: key}
		c.entries[key] = e
	}
	e.used = now
	if e.source != nil {
		if e.lru != nil {
			c.lruList.MoveToFront(e.lru)
		}
		c.evictLocked(nil, now)
		source := e.source
		c.mu.Unlock()
		return source, nil
	}
	if uiContext != nil && !slices.Contains(e.waiters, uiContext) {
		e.waiters = append(e.waiters, uiContext)
	}
	if e.err != nil && now.Sub(e.failed) < c.retryAfter {
		err := e.err
		c.mu.Unlock()
		return nil, err
	}
	if !e.loading {
		e.err = nil
		e.loading = true
		e.queued = true
		c.queue = append(c.queue, e)
		c.queueCond.Broadcast()
	} else if e.queued {
		c.promoteQueuedLocked(e)
	}
	c.mu.Unlock()
	return nil, nil //nolint:nilnil // A nil image with a nil error is the package's documented loading state.
}

func validateRemoteURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ErrInvalidURL
	}
	return nil
}

// promoteQueuedLocked moves a queued entry to the newest end of the LIFO
// queue when it is requested again, so scheduling follows request recency.
func (c *Cache) promoteQueuedLocked(e *entry) {
	for i, queued := range c.queue {
		if queued == e {
			copy(c.queue[i:], c.queue[i+1:])
			c.queue[len(c.queue)-1] = nil
			c.queue = c.queue[:len(c.queue)-1]
			break
		}
	}
	c.queue = append(c.queue, e)
}

func (c *Cache) worker() {
	defer c.workers.Done()
	for {
		c.mu.Lock()
		for len(c.queue) == 0 && c.ctx.Err() == nil {
			c.queueCond.Wait()
		}
		if c.ctx.Err() != nil {
			c.mu.Unlock()
			return
		}
		e := c.queue[len(c.queue)-1]
		c.queue[len(c.queue)-1] = nil
		c.queue = c.queue[:len(c.queue)-1]
		e.queued = false
		abandoned := !e.forgotten && c.now().Sub(e.used) > c.abandonAfter
		c.mu.Unlock()

		if e.forgotten || abandoned {
			c.abandon(e, abandoned)
			continue
		}
		c.process(e)
	}
}

// abandon drops queued work. A row that simply scrolled past invalidates its
// waiters so anything still visible asks again on its next build instead of
// waiting forever. A forgotten row does not: that invalidation could make a
// privacy deletion immediately requeue the image it just removed.
func (c *Cache) abandon(e *entry, invalidate bool) {
	c.mu.Lock()
	if c.entries[e.key] == e {
		delete(c.entries, e.key)
	}
	e.loading = false
	var waiters []*ui.Context
	if invalidate {
		waiters = append(waiters, e.waiters...)
	}
	e.waiters = nil
	c.mu.Unlock()
	invalidateAll(waiters)
}

func (c *Cache) process(e *entry) {
	result, err := c.fetchBytes(c.ctx, e, false)
	var source ui.ImageSource
	var memoryBytes int64
	if err == nil {
		source, memoryBytes, err = c.decodeSafely(result.data, e.key.maxSide)
		if err != nil && result.fallback != nil {
			if fallbackSource, fallbackBytes, fallbackErr := c.decodeSafely(result.fallback, e.key.maxSide); fallbackErr == nil {
				// A captive portal or application error body must not replace a
				// decodable stale disk copy, and must never be written over it.
				source, memoryBytes = fallbackSource, fallbackBytes
				result.store = false
				err = nil
			}
		}
		if err != nil && result.fromDisk {
			// Disk is untrusted. A corrupt fresh or revalidated body is removed
			// and refetched without validators in the same worker turn.
			c.removeDiskFiles(result)
			result, err = c.fetchBytes(c.ctx, e, true)
			if err == nil {
				source, memoryBytes, err = c.decodeSafely(result.data, e.key.maxSide)
			}
		}
	}

	if err == nil && result.store {
		if result.shared != nil {
			result.shared.storeOnce.Do(func() { c.storeIfCurrent(e, result) })
		} else {
			c.storeIfCurrent(e, result)
		}
	}

	c.mu.Lock()
	current := c.entries[e.key] == e && !e.forgotten
	e.loading = false
	e.queued = false
	now := c.now()
	if current {
		if err != nil {
			e.err = err
			e.failed = now
			c.scheduleRetryInvalidationLocked(e)
		} else {
			e.source = source
			e.err = nil
			e.memoryBytes = memoryBytes
			e.came = now
			if e.lru == nil {
				c.insertLRULocked(e)
			} else {
				c.lruList.MoveToFront(e.lru)
			}
			c.memoryBytes += memoryBytes
			c.evictLocked(e, now)
		}
	}
	var waiters []*ui.Context
	if current {
		waiters = append(waiters, e.waiters...)
	}
	e.waiters = nil
	c.mu.Unlock()
	invalidateAll(waiters)
}

// scheduleRetryInvalidationLocked wakes views whose visible images are in the
// negative-cache backoff. One cache-level timer avoids a timer per broken
// avatar in a large list. The timer does not retry by itself; the rebuild it
// causes observes that retryAfter has elapsed and queues the retry normally.
func (c *Cache) scheduleRetryInvalidationLocked(e *entry) {
	if c.stopped || c.retryAfter <= 0 {
		return
	}
	expires := e.failed.Add(c.retryAfter)
	if !c.retryAt.IsZero() && !expires.Before(c.retryAt) {
		return
	}
	c.retryAt = expires
	delay := expires.Sub(c.now())
	if c.retryTimer == nil {
		c.retryTimer = time.AfterFunc(delay, c.retryBackoffExpired)
		return
	}
	c.retryTimer.Reset(delay)
}

func (c *Cache) retryBackoffExpired() {
	c.mu.Lock()
	if c.stopped {
		c.retryAt = time.Time{}
		c.mu.Unlock()
		return
	}
	now := c.now()
	var waiters []*ui.Context
	seen := map[*ui.Context]bool{}
	var next time.Time
	for _, e := range c.entries {
		if e.err == nil || e.loading {
			continue
		}
		expires := e.failed.Add(c.retryAfter)
		if expires.After(now) {
			if next.IsZero() || expires.Before(next) {
				next = expires
			}
			continue
		}
		for _, waiter := range e.waiters {
			if !seen[waiter] {
				seen[waiter] = true
				waiters = append(waiters, waiter)
			}
		}
		e.waiters = nil
	}
	if next.IsZero() {
		c.retryAt = time.Time{}
	} else {
		c.retryAt = next
		c.retryTimer.Reset(next.Sub(now))
	}
	c.mu.Unlock()
	invalidateAll(waiters)
}

// insertLRULocked inserts a completed entry in request-recency order. Workers
// finish out of order, so completion order alone is not LRU order.
func (c *Cache) insertLRULocked(e *entry) {
	for position := c.lruList.Front(); position != nil; position = position.Next() {
		other, _ := position.Value.(*entry)
		if other == nil || e.used.After(other.used) {
			e.lru = c.lruList.InsertBefore(e, position)
			return
		}
	}
	e.lru = c.lruList.PushBack(e)
}

func invalidateAll(waiters []*ui.Context) {
	for _, waiter := range waiters {
		waiter.Invalidate()
	}
}

func (c *Cache) decodeSafely(data []byte, maxSide int) (ui.ImageSource, int64, error) {
	select {
	case c.decodeSlots <- struct{}{}:
		defer func() { <-c.decodeSlots }()
		return decodeSafely(data, maxSide)
	case <-c.ctx.Done():
		return nil, 0, c.ctx.Err() //nolint:wrapcheck // The cache context's cancellation is the precise worker state.
	}
}

// evictLocked evicts least-recently-used decoded entries past both memory
// limits. Entries used within protectFor are retained even when that exceeds a
// limit briefly, preventing a dense visible list from entering an eviction and
// reload loop; they become evictable once the window passes.
func (c *Cache) evictLocked(keep *entry, now time.Time) {
	for c.lruList.Len() > c.memoryEntryLimit || c.memoryBytes > c.memoryByteLimit {
		var victim *entry
		protected := false
		for position := c.lruList.Back(); position != nil; position = position.Prev() {
			candidate, _ := position.Value.(*entry)
			if candidate == nil {
				return
			}
			if candidate == keep {
				// A slow request can complete after newer rows and therefore sit at
				// the LRU back. Do not evict the image that just arrived, but do not
				// let it shield every older-on-screen entry either.
				continue
			}
			if now.Sub(candidate.used) < c.protectFor {
				protected = true
			}
			victim = candidate
			break
		}
		if victim == nil || protected {
			return
		}
		c.removeEntryLocked(victim.key, victim)
	}
}

func (c *Cache) removeEntryLocked(key cacheKey, e *entry) {
	e.forgotten = true
	if e.lru != nil {
		c.lruList.Remove(e.lru)
		e.lru = nil
		c.memoryBytes -= e.memoryBytes
		e.memoryBytes = 0
		e.source = nil
	}
	if c.entries[key] == e {
		delete(c.entries, key)
	}
}

// sweepLocked bounds bookkeeping for negative and abandoned entries. Decoded
// entries have their own stricter LRU bounds; loading entries are left alone.
func (c *Cache) sweepLocked(now time.Time) {
	if len(c.entries) <= maxTrackedEntries {
		return
	}
	for key, e := range c.entries {
		if len(c.entries) <= maxTrackedEntries {
			return
		}
		if e.source == nil && !e.loading && !e.failed.IsZero() && now.Sub(e.failed) >= c.retryAfter {
			delete(c.entries, key)
		}
	}
	if len(c.entries) <= maxTrackedEntries {
		return
	}
	var oldest *entry
	for _, e := range c.entries {
		if e.source == nil && !e.loading && (oldest == nil || e.used.Before(oldest.used)) {
			oldest = e
		}
	}
	if oldest != nil {
		delete(c.entries, oldest.key)
	}
}

func (c *Cache) diskToken(rawURL string) diskToken {
	c.mu.Lock()
	defer c.mu.Unlock()
	return diskToken{clear: c.clearEpoch, url: c.urlEpochs[rawURL]}
}

func (c *Cache) diskTokenCurrent(rawURL string, token diskToken) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return token.clear == c.clearEpoch && token.url == c.urlEpochs[rawURL]
}

// Forget removes every memory representation and the disk copy of url. A
// fetch already in flight is allowed to finish for its existing waiters, but
// its result is neither retained nor written. This performs disk I/O and must
// not be called from a view builder; call it from logout or privacy workflows.
func (c *Cache) Forget(rawURL string) error {
	if err := validateRemoteURL(rawURL); err != nil {
		return err
	}
	c.mu.Lock()
	c.urlEpochs[rawURL]++
	for key, e := range c.entries {
		if key.url == rawURL {
			c.removeEntryLocked(key, e)
		}
	}
	c.mu.Unlock()

	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	err := removeDiskFiles(c.dir, rawURL)
	if err == nil {
		c.noteDiskRemoveLocked(rawURL)
	}
	return err
}

// Clear removes every decoded entry and every cache-owned disk file. It does
// not remove foreign files from the configured directory, although the
// directory should still be dedicated to the cache. This performs disk I/O and
// must not be called from a view builder; call it from logout or privacy
// workflows. In-flight results are neither retained nor written.
func (c *Cache) Clear() error {
	c.mu.Lock()
	c.clearEpoch++
	for key, e := range c.entries {
		c.removeEntryLocked(key, e)
	}
	c.mu.Unlock()

	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	err := clearDiskFiles(c.dir)
	// Even when a removal failed, the next mutation must inspect the real
	// directory again rather than trusting entries deleted before the failure.
	c.diskIndex = diskIndex{}
	return err
}
