// Package state provides observable values for UI and application state:
// cells that notify subscribers, derived selections, and debounced autosave.
package state

import "sync"

// Source is a readable, observable value. Only [Cell] and [Derived] implement it.
type Source[T any] interface {
	Get() T
	// Subscribe registers fn for later changes; it is not called with the
	// current value. The returned stop function is idempotent.
	Subscribe(fn func(T)) (stop func())
	// snapshot returns the value and its version atomically.
	snapshot() (T, uint64)
}

// Cell holds a value and notifies subscribers when it changes. Stored values
// are treated as immutable: replace slices and maps instead of mutating them.
// A Cell is safe for use by multiple goroutines and must not be copied.
//
// Subscribers run outside the cell's lock, on the goroutine that wrote. Only
// one goroutine delivers at a time: a writer that commits while another
// goroutine is delivering returns immediately, and the delivering goroutine
// passes the latest value on. So callbacks of one cell never run concurrently,
// never see an older value than one they already saw, and may call Update on
// the same cell. Set and Update can return before their subscribers have run.
// Subscribers must not block.
type Cell[T any] struct {
	mu      sync.RWMutex
	value   T
	version uint64

	subMu  sync.Mutex
	subs   []*subscription[T]
	nextID uint64

	deliveryMu sync.Mutex
	delivering bool
	pending    bool
	// delivered is the last version passed to subscribers; only the
	// delivering goroutine touches it.
	delivered uint64
}

type subscription[T any] struct {
	id      uint64
	fn      func(T)
	stopped bool
}

// NewCell returns a Cell holding initial.
func NewCell[T any](initial T) *Cell[T] {
	return &Cell[T]{value: initial}
}

// Get returns the current value.
func (c *Cell[T]) Get() T {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

func (c *Cell[T]) snapshot() (T, uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value, c.version
}

// Set replaces the value and notifies subscribers.
func (c *Cell[T]) Set(v T) {
	c.mu.Lock()
	c.value = v
	c.version++
	c.mu.Unlock()
	c.deliver()
}

// Update runs fn on a shallow copy of the value while holding the cell's write
// lock, then commits the copy. If fn panics, nothing is committed. fn must not
// call methods of the same cell, and must replace rather than mutate slices and
// maps reachable from the value.
func (c *Cell[T]) Update(fn func(*T)) {
	c.commit(fn)
	c.deliver()
}

func (c *Cell[T]) commit(fn func(*T)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := c.value
	fn(&next)
	c.value = next
	c.version++
}

// Subscribe calls fn after each change, with the latest value at the time of
// delivery. Several rapid changes may be delivered as one. Calling stop is
// idempotent and safe from inside a callback.
func (c *Cell[T]) Subscribe(fn func(T)) (stop func()) {
	c.subMu.Lock()
	c.nextID++
	s := &subscription[T]{id: c.nextID, fn: fn}
	c.subs = append(c.subs, s)
	c.subMu.Unlock()
	return func() {
		c.subMu.Lock()
		defer c.subMu.Unlock()
		s.stopped = true
		for i, other := range c.subs {
			if other.id == s.id {
				c.subs = append(c.subs[:i:i], c.subs[i+1:]...)
				return
			}
		}
	}
}

func (c *Cell[T]) deliver() {
	c.deliveryMu.Lock()
	c.pending = true
	if c.delivering {
		c.deliveryMu.Unlock()
		return
	}
	c.delivering = true
	c.deliveryMu.Unlock()
	// A panicking subscriber must not leave the cell permanently "delivering".
	finished := false
	defer func() {
		if !finished {
			c.deliveryMu.Lock()
			c.delivering = false
			c.deliveryMu.Unlock()
		}
	}()
	for c.takePending() {
		v, version := c.snapshot()
		if version > c.delivered {
			c.delivered = version
			c.notify(v)
		}
	}
	finished = true
}

// takePending reports whether a commit is waiting for delivery. When none is,
// it also ends the delivery in the same critical section, so a concurrent
// commit either is seen here or starts its own delivery.
func (c *Cell[T]) takePending() bool {
	c.deliveryMu.Lock()
	defer c.deliveryMu.Unlock()
	if !c.pending {
		c.delivering = false
		return false
	}
	c.pending = false
	return true
}

func (c *Cell[T]) notify(v T) {
	c.subMu.Lock()
	subs := append([]*subscription[T](nil), c.subs...)
	c.subMu.Unlock()
	for _, s := range subs {
		c.subMu.Lock()
		stopped := s.stopped
		c.subMu.Unlock()
		if !stopped {
			s.fn(v)
		}
	}
}
