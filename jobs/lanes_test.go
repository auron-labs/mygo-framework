package jobs_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/jobs"
)

func newLanes(t *testing.T, limits map[string]int) *jobs.Manager {
	t.Helper()
	m := jobs.New(t.Context())
	for name, limit := range limits {
		if err := m.SetLane(name, limit); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

// gate is a job body that reports when it starts and finishes when released.
type gate struct {
	started chan string
	release chan struct{}
}

func newGate() *gate { return &gate{started: make(chan string, 16), release: make(chan struct{})} }

func (g *gate) job(name string) jobs.Func {
	return func(ctx context.Context, _ jobs.Report) error {
		g.started <- name
		select {
		case <-g.release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func waitStarted(t *testing.T, g *gate, want string) {
	t.Helper()
	select {
	case got := <-g.started:
		if got != want {
			t.Fatalf("started %q, want %q", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("job %q did not start", want)
	}
}

func state(t *testing.T, m *jobs.Manager, name string) jobs.State {
	t.Helper()
	s, ok := m.Status(name)
	if !ok {
		t.Fatalf("no status for %q", name)
	}
	return s.State
}

func eventually(t *testing.T, m *jobs.Manager, name string, want jobs.State) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s, ok := m.Status(name); ok && s.State == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("job %q state = %v, want %v", name, state(t, m, name), want)
}

func TestLaneQueuesAndRunsInOrder(t *testing.T) {
	m := newLanes(t, map[string]int{"installs": 1})
	var mu sync.Mutex
	var events []jobs.Status
	m.Subscribe(func(s jobs.Status) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, s)
	})
	g := newGate()
	for _, name := range []string{"a", "b", "c"} {
		if err := m.StartIn("installs", name, g.job(name)); err != nil {
			t.Fatal(err)
		}
	}
	waitStarted(t, g, "a")
	if state(t, m, "b") != jobs.Queued || state(t, m, "c") != jobs.Queued {
		t.Fatal("b and c should be queued")
	}
	g.release <- struct{}{}
	waitStarted(t, g, "b")
	if state(t, m, "c") != jobs.Queued {
		t.Fatal("c should still be queued")
	}
	g.release <- struct{}{}
	waitStarted(t, g, "c")
	close(g.release)
	if err := m.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	var forB []jobs.State
	for _, e := range events {
		if e.Name == "b" {
			if e.Lane != "installs" {
				t.Errorf("lane = %q", e.Lane)
			}
			forB = append(forB, e.State)
		}
	}
	want := []jobs.State{jobs.Queued, jobs.Running, jobs.Done}
	if len(forB) != len(want) {
		t.Fatalf("b events = %v", forB)
	}
	for i := range want {
		if forB[i] != want[i] {
			t.Fatalf("b events = %v, want %v", forB, want)
		}
	}
}

func TestLaneLimitBoundsConcurrency(t *testing.T) {
	m := newLanes(t, map[string]int{"checks": 3})
	var cur, peak atomic.Int32
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		name := string(rune('a' + i))
		err := m.StartIn("checks", name, func(context.Context, jobs.Report) error {
			defer wg.Done()
			n := cur.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			<-release
			cur.Add(-1)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	queued := 0
	for _, s := range m.List() {
		if s.State == jobs.Queued {
			queued++
		}
	}
	if queued != 5 {
		t.Fatalf("queued = %d, want 5", queued)
	}
	close(release)
	wg.Wait()
	if p := peak.Load(); p > 3 || p < 1 {
		t.Fatalf("peak concurrency = %d", p)
	}
}

func TestLanesAreIndependent(t *testing.T) {
	m := newLanes(t, map[string]int{"installs": 1, "checks": 1})
	g := newGate()
	defer close(g.release)
	for _, c := range [][2]string{{"installs", "i"}, {"checks", "c"}} {
		if err := m.StartIn(c[0], c[1], g.job(c[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Start("free", g.job("free")); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for range 3 {
		select {
		case n := <-g.started:
			seen[n] = true
		case <-time.After(5 * time.Second):
			t.Fatalf("only %v started", seen)
		}
	}
	if !seen["i"] || !seen["c"] || !seen["free"] {
		t.Fatalf("started = %v", seen)
	}
}

func TestLaneErrors(t *testing.T) {
	m := newLanes(t, map[string]int{"l": 1})
	if err := m.StartIn("nope", "a", nil); !errors.Is(err, jobs.ErrUnknownLane) {
		t.Fatalf("unknown lane = %v", err)
	}
	if err := m.StartIn("", "a", nil); !errors.Is(err, jobs.ErrUnknownLane) {
		t.Fatalf("empty lane = %v", err)
	}
	if err := m.SetLane("x", 0); !errors.Is(err, jobs.ErrInvalidLimit) {
		t.Fatalf("limit 0 = %v", err)
	}
	g := newGate()
	defer close(g.release)
	if err := m.StartIn("l", "a", g.job("a")); err != nil {
		t.Fatal(err)
	}
	if err := m.StartIn("l", "b", g.job("b")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b"} {
		if err := m.StartIn("l", name, g.job(name)); !errors.Is(err, jobs.ErrRunning) {
			t.Fatalf("duplicate %q = %v", name, err)
		}
	}
}

func TestCancelQueuedJob(t *testing.T) {
	m := newLanes(t, map[string]int{"l": 1})
	g := newGate()
	var ran atomic.Bool
	_ = m.StartIn("l", "a", g.job("a"))
	_ = m.StartIn("l", "b", func(context.Context, jobs.Report) error { ran.Store(true); return nil })
	_ = m.StartIn("l", "c", g.job("c"))
	waitStarted(t, g, "a")
	if !m.Cancel("b") {
		t.Fatal("queued job not found")
	}
	if state(t, m, "b") != jobs.Canceled {
		t.Fatalf("b = %v", state(t, m, "b"))
	}
	if m.Cancel("b") {
		t.Fatal("canceled job canceled twice")
	}
	g.release <- struct{}{}
	waitStarted(t, g, "c")
	if ran.Load() {
		t.Fatal("canceled queued job ran")
	}
	close(g.release)
	if err := m.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCancelRunningPromotesNext(t *testing.T) {
	m := newLanes(t, map[string]int{"l": 1})
	g := newGate()
	defer close(g.release)
	_ = m.StartIn("l", "a", g.job("a"))
	_ = m.StartIn("l", "b", g.job("b"))
	waitStarted(t, g, "a")
	if !m.Cancel("a") {
		t.Fatal("running job not found")
	}
	waitStarted(t, g, "b")
	eventually(t, m, "a", jobs.Canceled)
}

func TestFailureAndPanicFreeSlot(t *testing.T) {
	m := newLanes(t, map[string]int{"l": 1})
	g := newGate()
	defer close(g.release)
	_ = m.StartIn("l", "bad", func(context.Context, jobs.Report) error { return errors.New("boom") })
	_ = m.StartIn("l", "panic", func(context.Context, jobs.Report) error { panic("oops") })
	_ = m.StartIn("l", "ok", g.job("ok"))
	waitStarted(t, g, "ok")
	eventually(t, m, "bad", jobs.Failed)
	eventually(t, m, "panic", jobs.Failed)
}

func TestSetLaneRaisingLimitStartsQueued(t *testing.T) {
	m := newLanes(t, map[string]int{"l": 1})
	g := newGate()
	defer close(g.release)
	_ = m.StartIn("l", "a", g.job("a"))
	_ = m.StartIn("l", "b", g.job("b"))
	waitStarted(t, g, "a")
	if err := m.SetLane("l", 2); err != nil {
		t.Fatal(err)
	}
	waitStarted(t, g, "b")
}

func TestListReportsEveryJobInOrder(t *testing.T) {
	m := newLanes(t, map[string]int{"l": 1})
	g := newGate()
	_ = m.Start("plain", func(context.Context, jobs.Report) error { return nil })
	_ = m.StartIn("l", "a", g.job("a"))
	_ = m.StartIn("l", "b", g.job("b"))
	waitStarted(t, g, "a")
	eventually(t, m, "plain", jobs.Done)
	got := m.List()
	want := []jobs.Status{
		{Name: "plain", State: jobs.Done},
		{Name: "a", Lane: "l", State: jobs.Running},
		{Name: "b", Lane: "l", State: jobs.Queued},
	}
	if len(got) != len(want) {
		t.Fatalf("list = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("list[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	close(g.release)
	_ = m.Shutdown(t.Context())
}

func TestShutdownCancelsQueue(t *testing.T) {
	m := newLanes(t, map[string]int{"l": 1})
	g := newGate()
	var ran atomic.Bool
	_ = m.StartIn("l", "a", g.job("a"))
	_ = m.StartIn("l", "b", func(context.Context, jobs.Report) error { ran.Store(true); return nil })
	waitStarted(t, g, "a")
	if err := m.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if ran.Load() {
		t.Fatal("queued job ran during Shutdown")
	}
	if state(t, m, "a") != jobs.Canceled || state(t, m, "b") != jobs.Canceled {
		t.Fatalf("states = %v, %v", state(t, m, "a"), state(t, m, "b"))
	}
	if err := m.StartIn("l", "c", nil); !errors.Is(err, jobs.ErrStopped) {
		t.Fatalf("StartIn after Shutdown = %v", err)
	}
}

func TestParentCancelDoesNotRunQueuedJobs(t *testing.T) {
	parent, cancel := context.WithCancel(t.Context())
	m := jobs.New(parent)
	_ = m.SetLane("l", 1)
	g := newGate()
	var ran atomic.Bool
	_ = m.StartIn("l", "a", g.job("a"))
	_ = m.StartIn("l", "b", func(context.Context, jobs.Report) error { ran.Store(true); return nil })
	waitStarted(t, g, "a")
	cancel()
	eventually(t, m, "a", jobs.Canceled)
	eventually(t, m, "b", jobs.Canceled)
	if ran.Load() {
		t.Fatal("queued job ran after parent cancel")
	}
	if err := m.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
}
