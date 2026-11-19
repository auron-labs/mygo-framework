package jobs_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/jobs"
)

const hour = time.Hour

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type env struct {
	clock *fakeClock
	m     *jobs.Manager
	s     *jobs.Scheduler
	runs  chan struct{}
}

func newEnv(t *testing.T, store jobs.LastRunStore, clock *fakeClock) *env {
	t.Helper()
	if clock == nil {
		clock = newClock()
	}
	m := jobs.New(t.Context())
	s := jobs.NewScheduler(m, store,
		jobs.WithClock(clock.Now), jobs.WithPollInterval(5*time.Millisecond), jobs.WithRetryDelay(10*time.Minute))
	t.Cleanup(func() {
		_ = s.Shutdown(context.Background())
		_ = m.Shutdown(context.Background())
	})
	return &env{clock: clock, m: m, s: s, runs: make(chan struct{}, 32)}
}

func (e *env) task(t *testing.T) {
	t.Helper()
	err := e.s.Schedule("feed", hour, func(context.Context, jobs.Report) error {
		e.runs <- struct{}{}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (e *env) expectRun(t *testing.T) {
	t.Helper()
	select {
	case <-e.runs:
	case <-time.After(5 * time.Second):
		t.Fatal("task did not run")
	}
}

// expectNoRun lets several poll ticks pass.
func (e *env) expectNoRun(t *testing.T) {
	t.Helper()
	e.s.Nudge()
	select {
	case <-e.runs:
		t.Fatal("task ran unexpectedly")
	case <-time.After(60 * time.Millisecond):
	}
}

// waitIdle waits until the job has finished and the scheduler recorded it.
func (e *env) waitIdle(t *testing.T) {
	const name = "feed"
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s, ok := e.m.Status(name); ok && s.State != jobs.Running {
			time.Sleep(10 * time.Millisecond)
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not finish")
}

func TestRunsImmediatelyWhenNeverRunAndPersists(t *testing.T) {
	store := &jobs.MemoryStore{}
	e := newEnv(t, store, nil)
	e.task(t)
	e.s.Start(t.Context())
	e.expectRun(t)
	e.waitIdle(t)
	at, ok, _ := store.LastRun(t.Context(), "feed")
	if !ok || !at.Equal(e.clock.Now()) {
		t.Fatalf("stored = %v, %v", at, ok)
	}
	e.expectNoRun(t)
}

func TestWaitsForIntervalThenRuns(t *testing.T) {
	store := &jobs.MemoryStore{}
	e := newEnv(t, store, nil)
	_ = store.SetLastRun(t.Context(), "feed", e.clock.Now().Add(-10*time.Minute))
	e.task(t)
	e.s.Start(t.Context())
	e.expectNoRun(t)
	e.clock.Advance(51 * time.Minute)
	e.s.Nudge()
	e.expectRun(t)
}

func TestRestartHonorsStoredLastRun(t *testing.T) {
	store := &jobs.MemoryStore{}
	clock := newClock()
	first := newEnv(t, store, clock)
	first.task(t)
	first.s.Start(t.Context())
	first.expectRun(t)
	first.waitIdle(t)
	_ = first.s.Shutdown(t.Context())

	clock.Advance(20 * time.Minute)
	second := newEnv(t, store, clock)
	second.task(t)
	second.s.Start(t.Context())
	second.expectNoRun(t)
	clock.Advance(41 * time.Minute)
	second.s.Nudge()
	second.expectRun(t)
}

func TestCatchUpRunsOnce(t *testing.T) {
	store := &jobs.MemoryStore{}
	e := newEnv(t, store, nil)
	_ = store.SetLastRun(t.Context(), "feed", e.clock.Now())
	e.task(t)
	e.s.Start(t.Context())
	e.expectNoRun(t)
	e.clock.Advance(10 * hour)
	e.s.Nudge()
	e.expectRun(t)
	e.waitIdle(t)
	e.expectNoRun(t)
}

func TestNoOverlapAndRunNowBusy(t *testing.T) {
	e := newEnv(t, nil, nil)
	started := make(chan struct{}, 8)
	release := make(chan struct{})
	err := e.s.Schedule("slow", hour, func(context.Context, jobs.Report) error {
		started <- struct{}{}
		<-release
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	e.s.Start(t.Context())
	<-started
	for range 5 {
		e.s.Nudge()
	}
	if err := e.s.RunNow("slow"); !errors.Is(err, jobs.ErrRunning) {
		t.Fatalf("RunNow = %v, want ErrRunning", err)
	}
	time.Sleep(40 * time.Millisecond)
	if len(started) != 0 {
		t.Fatalf("task started %d extra times", len(started))
	}
	close(release)
}

func TestRunNowResetsInterval(t *testing.T) {
	store := &jobs.MemoryStore{}
	e := newEnv(t, store, nil)
	_ = store.SetLastRun(t.Context(), "feed", e.clock.Now())
	e.task(t)
	e.s.Start(t.Context())
	e.clock.Advance(40 * time.Minute)
	if err := e.s.RunNow("feed"); err != nil {
		t.Fatal(err)
	}
	e.expectRun(t)
	e.waitIdle(t)
	e.clock.Advance(30 * time.Minute) // past the original schedule, before the new one
	e.expectNoRun(t)
	next, ok := e.s.NextRun("feed")
	if want := e.clock.Now().Add(30 * time.Minute); !ok || !next.Equal(want) {
		t.Fatalf("NextRun = %v, want %v", next, want)
	}
	e.clock.Advance(31 * time.Minute)
	e.s.Nudge()
	e.expectRun(t)
}

func TestRunNowUnknown(t *testing.T) {
	e := newEnv(t, nil, nil)
	if err := e.s.RunNow("nope"); !errors.Is(err, jobs.ErrUnknownTask) {
		t.Fatalf("err = %v", err)
	}
}

func TestScheduleValidation(t *testing.T) {
	e := newEnv(t, nil, nil)
	fn := func(context.Context, jobs.Report) error { return nil }
	for name, call := range map[string]func() error{
		"empty name":  func() error { return e.s.Schedule("", hour, fn) },
		"no interval": func() error { return e.s.Schedule("a", 0, fn) },
		"nil func":    func() error { return e.s.Schedule("a", hour, nil) },
	} {
		if err := call(); !errors.Is(err, jobs.ErrInvalidSchedule) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := e.s.Schedule("a", hour, fn); err != nil {
		t.Fatal(err)
	}
	if err := e.s.Schedule("a", hour, fn); !errors.Is(err, jobs.ErrInvalidSchedule) {
		t.Errorf("duplicate: err = %v", err)
	}
}

func TestFailureDoesNotPersistAndRetries(t *testing.T) {
	for name, fail := range map[string]func() error{
		"error":    func() error { return errors.New("offline") },
		"canceled": func() error { return context.Canceled },
		"panic":    func() error { panic("boom") },
	} {
		t.Run(name, func(t *testing.T) {
			store := &jobs.MemoryStore{}
			e := newEnv(t, store, nil)
			var mu sync.Mutex
			failing := true
			err := e.s.Schedule("feed", hour, func(context.Context, jobs.Report) error {
				mu.Lock()
				defer mu.Unlock()
				e.runs <- struct{}{}
				if failing {
					return fail()
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			e.s.Start(t.Context())
			e.expectRun(t)
			e.waitIdle(t)
			if _, ok, _ := store.LastRun(t.Context(), "feed"); ok {
				t.Fatal("failed run was persisted")
			}
			e.expectNoRun(t) // retry delay has not passed
			mu.Lock()
			failing = false
			mu.Unlock()
			e.clock.Advance(11 * time.Minute)
			e.s.Nudge()
			e.expectRun(t)
			e.waitIdle(t)
			if _, ok, _ := store.LastRun(t.Context(), "feed"); !ok {
				t.Fatal("successful retry was not persisted")
			}
		})
	}
}

type brokenStore struct{}

func (brokenStore) LastRun(context.Context, string) (time.Time, bool, error) {
	return time.Time{}, false, errors.New("read failed")
}

func (brokenStore) SetLastRun(context.Context, string, time.Time) error {
	return errors.New("write failed")
}

func TestStoreErrorsDoNotStopScheduling(t *testing.T) {
	e := newEnv(t, brokenStore{}, nil)
	e.task(t)
	e.s.Start(t.Context())
	e.expectRun(t)
	e.waitIdle(t)
	e.expectNoRun(t) // in-memory last run still applies
	e.clock.Advance(2 * hour)
	e.s.Nudge()
	e.expectRun(t)
}

func TestClockMovedBackwardsDoesNotStall(t *testing.T) {
	store := &jobs.MemoryStore{}
	e := newEnv(t, store, nil)
	_ = store.SetLastRun(t.Context(), "feed", e.clock.Now().Add(100*hour))
	e.task(t)
	e.s.Start(t.Context())
	e.expectNoRun(t)
	e.clock.Advance(61 * time.Minute)
	e.s.Nudge()
	e.expectRun(t)
}

func TestScheduledRunIsObservableAndCancelable(t *testing.T) {
	e := newEnv(t, nil, nil)
	started := make(chan struct{})
	err := e.s.Schedule("feed", hour, func(ctx context.Context, _ jobs.Report) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	e.s.Start(t.Context())
	<-started
	if s, _ := e.m.Status("feed"); s.State != jobs.Running {
		t.Fatalf("status = %+v", s)
	}
	if !e.m.Cancel("feed") {
		t.Fatal("cancel found no job")
	}
	e.waitIdle(t)
	if s, _ := e.m.Status("feed"); s.State != jobs.Canceled {
		t.Fatalf("status = %+v", s)
	}
}

func TestShutdownStopsLoop(t *testing.T) {
	e := newEnv(t, nil, nil)
	e.task(t)
	e.s.Start(t.Context())
	e.expectRun(t)
	if err := e.s.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	e.waitIdle(t)
	e.clock.Advance(5 * hour)
	e.s.Nudge()
	e.expectNoRun(t)
}

func TestNextRunUnknown(t *testing.T) {
	e := newEnv(t, nil, nil)
	if _, ok := e.s.NextRun("nope"); ok {
		t.Fatal("unknown task has a next run")
	}
}
