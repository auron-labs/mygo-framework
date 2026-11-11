package state

import "sync"

// Derived is a value computed from a [Source]. It is lazy: Get recomputes only
// when the source has changed, and a Derived holds no subscription of its own,
// so it needs no cleanup. It implements [Source], so selections can be chained.
type Derived[V any] struct {
	compute   func() (V, uint64)
	subscribe func(func(V)) func()
	equal     func(a, b V) bool

	mu         sync.Mutex
	have       bool
	value      V
	version    uint64
	sourceSeen uint64
}

// Select derives fn(src) and treats values as unchanged when they are equal.
func Select[T any, V comparable](src Source[T], fn func(T) V) *Derived[V] {
	return SelectFunc(src, fn, func(a, b V) bool { return a == b })
}

// SelectFunc is like [Select] for values that are not comparable, using equal
// to decide whether the selected value changed.
func SelectFunc[T, V any](src Source[T], fn func(T) V, equal func(a, b V) bool) *Derived[V] {
	d := &Derived[V]{equal: equal}
	d.compute = func() (V, uint64) {
		t, sourceVersion := src.snapshot()
		d.mu.Lock()
		defer d.mu.Unlock()
		if !d.have || sourceVersion > d.sourceSeen {
			next := fn(t)
			if !d.have || !d.equal(d.value, next) {
				d.value = next
				d.version++
			}
			d.have = true
			d.sourceSeen = sourceVersion
		}
		return d.value, d.version
	}
	d.subscribe = func(deliver func(V)) func() {
		last, _ := d.compute()
		return src.Subscribe(func(t T) {
			next := fn(t)
			if equal(last, next) {
				return
			}
			last = next
			deliver(next)
		})
	}
	return d
}

// Get returns the selected value.
func (d *Derived[V]) Get() V {
	v, _ := d.compute()
	return v
}

func (d *Derived[V]) snapshot() (V, uint64) { return d.compute() }

// Subscribe calls fn when the selected value differs from the last value
// delivered to this subscription, starting from the value at subscribe time.
// fn runs on the goroutine that changed the source.
func (d *Derived[V]) Subscribe(fn func(V)) (stop func()) {
	return d.subscribe(fn)
}
