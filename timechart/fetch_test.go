package timechart

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestFetcherDiscardsStaleResults(t *testing.T) {
	type call struct {
		from    float64
		release chan struct{}
	}
	calls := make(chan call, 4)
	provider := func(_ context.Context, from, _ float64, _ int) (Points, error) {
		c := call{from, make(chan struct{})}
		calls <- c
		<-c.release // answers arrive when the test says, whatever the context
		return Points{X: []float64{from}, Y: []float64{from}}, nil
	}
	var notified atomic.Int32
	f := &fetcher{}
	notify := func() { notified.Add(1) }

	f.points(provider, notify, 0, 10, 100)
	first := <-calls
	f.points(provider, notify, 1000, 1010, 100) // the view moved on
	second := <-calls
	close(second.release)
	eventually(t, "the current answer", func() bool { return notified.Load() == 1 })
	close(first.release) // the old answer arrives late
	time.Sleep(20 * time.Millisecond)

	got := f.points(provider, notify, 1000, 1010, 100)
	if len(got.X) != 1 || got.X[0] != second.from || notified.Load() != 1 {
		t.Fatalf("points %v, notified %d: the stale answer replaced the current one", got, notified.Load())
	}
}

func TestFetcherKeepsDrawingLastResultWhilePending(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	provider := func(_ context.Context, from, _ float64, _ int) (Points, error) {
		if calls.Add(1) == 2 {
			<-release
		}
		return Points{X: []float64{from}, Y: []float64{1}}, nil
	}
	var notified atomic.Int32
	f := &fetcher{}
	notify := func() { notified.Add(1) }
	f.points(provider, notify, 0, 10, 100)
	eventually(t, "the first answer", func() bool { return notified.Load() == 1 })

	start := time.Now()
	got := f.points(provider, notify, 500, 510, 100)
	if time.Since(start) > 500*time.Millisecond || len(got.X) != 1 || got.X[0] >= 0 {
		t.Fatalf("pending fetch blocked or lost the last result: %v after %v", got, time.Since(start))
	}
	close(release)
	eventually(t, "the second answer", func() bool { return notified.Load() == 2 })
}

func TestFetcherAsksOnlyWhenTheViewLeavesTheWindow(t *testing.T) {
	var calls atomic.Int32
	provider := func(context.Context, float64, float64, int) (Points, error) {
		calls.Add(1)
		return Points{}, nil
	}
	f := &fetcher{}
	notify := func() {}
	f.points(provider, notify, 100, 200, 500)
	f.points(provider, notify, 110, 210, 500) // pan inside the margin
	f.points(provider, notify, 105, 195, 500) // slight zoom in
	eventually(t, "the first call", func() bool { return calls.Load() == 1 })
	f.points(provider, notify, 200, 300, 500) // pan out of it
	f.points(provider, notify, 200, 210, 500) // zoom far in: window too coarse
	eventually(t, "new calls", func() bool { return calls.Load() == 3 })
}

func TestFetcherDoesNotRetryAFailedWindow(t *testing.T) {
	var calls atomic.Int32
	provider := func(context.Context, float64, float64, int) (Points, error) {
		calls.Add(1)
		return Points{}, errors.New("unavailable")
	}
	f := &fetcher{}
	for range 5 {
		f.points(provider, func() {}, 0, 10, 100)
		time.Sleep(5 * time.Millisecond)
	}
	if calls.Load() != 1 {
		t.Fatalf("%d calls", calls.Load())
	}
}

func TestFetcherRequestsPixelDensityForTheWholeWindow(t *testing.T) {
	widths := make(chan int, 1)
	f := &fetcher{}
	f.points(func(_ context.Context, _, _ float64, w int) (Points, error) {
		widths <- w
		return Points{}, nil
	}, func() {}, 0, 10, 800)
	if w := <-widths; w != 1200 {
		t.Fatalf("pixel width %d, want 1200 for a window 1.5 times the view", w)
	}
}
