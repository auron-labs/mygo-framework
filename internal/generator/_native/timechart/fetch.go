package timechart

import (
	"context"
	"sync"
)

// Provider returns the points of one series in from to to (in x units),
// decimated for a plot pixelWidth wide, in ascending x order. It runs off
// the UI thread and may be slow; it should return once ctx is canceled,
// which happens when the view moves on. Decimate serves in-memory samples.
// Report failures to the application's log: a failed window keeps the
// previous points on screen and is asked for again only after the view
// leaves it.
type Provider func(ctx context.Context, from, to float64, pixelWidth int) (Points, error)

const (
	// A fetched window reaches this fraction of the visible span beyond each
	// edge, so small pans need no new fetch.
	fetchMargin = .25
	// A fetched window serves the view until it is this many times wider than
	// the view, when zooming in has made it too coarse.
	fetchCoarse = 2.25
)

type window struct {
	from, to float64
}

func (w window) covers(lo, hi float64) bool {
	return w.from <= lo && w.to >= hi && (w.to-w.from) <= fetchCoarse*(hi-lo)
}

// fetcher keeps the last points of a series and replaces them in the
// background as the view moves.
type fetcher struct {
	mu       sync.Mutex
	notify   func()
	have     Points
	haveWin  window // the window have was fetched for
	loaded   bool
	asked    window // newest window requested, in flight or failed
	hasAsked bool
	version  uint64
	cancel   context.CancelFunc
}

// points returns the points to draw for the view lo to hi, pixelWidth
// plot pixels wide, and starts a fetch when they do not cover it. It never
// blocks on the provider.
func (f *fetcher) points(provider Provider, notify func(), lo, hi float64, pixelWidth int) Points {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notify = notify
	if f.hasAsked && f.asked.covers(lo, hi) {
		return f.have
	}
	margin := (hi - lo) * fetchMargin
	f.asked = window{lo - margin, hi + margin}
	f.hasAsked = true
	if f.cancel != nil {
		f.cancel()
	}
	f.version++
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	// The window is 1+2*margin times the view, with as many columns per pixel.
	width := int(float64(pixelWidth) * (1 + 2*fetchMargin))
	go f.fetch(ctx, f.version, provider, f.asked, width)
	return f.have
}

func (f *fetcher) fetch(ctx context.Context, version uint64, provider Provider, w window, width int) {
	points, err := provider(ctx, w.from, w.to, width)
	f.mu.Lock()
	// A newer fetch owns the series; this answer is for a view gone by.
	if version != f.version || ctx.Err() != nil || err != nil {
		f.mu.Unlock()
		return
	}
	f.have, f.haveWin, f.loaded = points, w, true
	notify := f.notify
	f.mu.Unlock()
	notify()
}

// ready reports whether the points held cover the view lo to hi.
func (f *fetcher) ready(lo, hi float64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loaded && f.haveWin.covers(lo, hi)
}

// close stops the fetch in flight.
func (f *fetcher) close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.version++
	if f.cancel != nil {
		f.cancel()
	}
}
