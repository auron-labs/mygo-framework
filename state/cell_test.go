package state_test

import (
	"mygo-framework/state"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentWritersDeliverMonotonicValues(t *testing.T) {
	cell := state.NewCell(0)
	var last int
	var running atomic.Bool
	stop := cell.Subscribe(func(v int) {
		if !running.CompareAndSwap(false, true) {
			t.Error("subscriber ran concurrently")
		}
		defer running.Store(false)
		if v < last {
			t.Errorf("saw %d after %d", v, last)
		}
		last = v
	})
	defer stop()

	const writers, writes = 8, 200
	var wg sync.WaitGroup
	for range writers {
		wg.Go(func() {
			for range writes {
				cell.Update(func(v *int) { *v++ })
			}
		})
	}
	wg.Wait()
	if got := cell.Get(); got != writers*writes {
		t.Fatalf("value = %d, want %d", got, writers*writes)
	}
}

func TestSubscriberMayUpdateSameCell(t *testing.T) {
	cell := state.NewCell(0)
	var seen []int
	cell.Subscribe(func(v int) {
		seen = append(seen, v)
		if v < 3 {
			cell.Update(func(p *int) { *p = v + 1 })
		}
	})
	cell.Set(1)
	if cell.Get() != 3 || seen[len(seen)-1] != 3 {
		t.Fatalf("value = %d, seen = %v", cell.Get(), seen)
	}
}

func TestSubscribeSkipsCurrentValueAndStopIsIdempotent(t *testing.T) {
	cell := state.NewCell("a")
	calls := 0
	var stop func()
	stop = cell.Subscribe(func(string) {
		calls++
		stop()
		stop()
	})
	if calls != 0 {
		t.Fatal("Subscribe delivered the current value")
	}
	cell.Set("b")
	cell.Set("c")
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 after stopping inside the callback", calls)
	}
}

func TestPanicInUpdateCommitsNothing(t *testing.T) {
	type pair struct{ a, b int }
	cell := state.NewCell(pair{1, 2})
	notified := false
	cell.Subscribe(func(pair) { notified = true })
	func() {
		defer func() { _ = recover() }()
		cell.Update(func(p *pair) {
			p.a = 99
			panic("boom")
		})
	}()
	if got := cell.Get(); got != (pair{1, 2}) || notified {
		t.Fatalf("value = %+v, notified = %v", got, notified)
	}
	cell.Update(func(p *pair) { p.b = 3 })
	if got := cell.Get(); got != (pair{1, 3}) {
		t.Fatalf("cell unusable after panic: %+v", got)
	}
}
