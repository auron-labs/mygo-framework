package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/auron-labs/mygo-framework/errorreport"
)

const (
	defaultPollInterval = time.Minute
	defaultRetryDelay   = time.Minute
)

var (
	// ErrUnknownTask is returned by RunNow for a name that was never scheduled.
	ErrUnknownTask = errors.New("unknown scheduled task")
	// ErrInvalidSchedule is returned by Schedule for an empty name, a
	// non-positive interval, a nil function, or a name already scheduled.
	ErrInvalidSchedule = errors.New("invalid schedule")
)

// LastRunStore persists when each scheduled task last succeeded, so an
// interval survives restarts. Implementations must be safe for concurrent use.
type LastRunStore interface {
	// LastRun returns the stored time, or false when the task never ran.
	LastRun(ctx context.Context, name string) (time.Time, bool, error)
	SetLastRun(ctx context.Context, name string, at time.Time) error
}

// MemoryStore is a LastRunStore that forgets everything on exit.
type MemoryStore struct {
	mu   sync.Mutex
	runs map[string]time.Time
}

// LastRun implements LastRunStore.
func (s *MemoryStore) LastRun(_ context.Context, name string) (time.Time, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.runs[name]
	return at, ok, nil
}

// SetLastRun implements LastRunStore.
func (s *MemoryStore) SetLastRun(_ context.Context, name string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs == nil {
		s.runs = map[string]time.Time{}
	}
	s.runs[name] = at
	return nil
}

// Option configures a Scheduler.
type Option func(*Scheduler)

// WithClock replaces time.Now, mainly for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Scheduler) { s.now = now }
}

// WithPollInterval sets how often the scheduler checks for due tasks. It is
// also the longest a catch-up after sleep can take without a Nudge.
func WithPollInterval(d time.Duration) Option {
	return func(s *Scheduler) {
		if d > 0 {
			s.poll = d
		}
	}
}

// WithRetryDelay sets how long a task that failed or was canceled waits
// before it is due again, instead of waiting a full interval.
func WithRetryDelay(d time.Duration) Option {
	return func(s *Scheduler) {
		if d > 0 {
			s.retry = d
		}
	}
}

type task struct {
	name     string
	interval time.Duration
	fn       Func

	loaded  bool
	last    time.Time
	hasLast bool
	retryAt time.Time
}

// Scheduler runs tasks periodically through a Manager, so status,
// subscription and cancellation work as for any other job. The last
// successful run is persisted, and a task that came due while the machine
// slept runs once as soon as the scheduler notices.
type Scheduler struct {
	manager *Manager
	store   LastRunStore
	now     func() time.Time
	poll    time.Duration
	retry   time.Duration

	mu      sync.Mutex
	tasks   map[string]*task
	order   []*task
	started bool
	cancel  context.CancelFunc
	done    chan struct{}
	nudge   chan struct{}
	runCtx  context.Context //nolint:containedctx // Detached context for persisting results after shutdown starts.
}

// NewScheduler returns a scheduler that starts tasks on m and records runs in
// store. A nil store keeps last-run times in memory only.
func NewScheduler(m *Manager, store LastRunStore, opts ...Option) *Scheduler {
	if store == nil {
		store = &MemoryStore{}
	}
	s := &Scheduler{
		manager: m,
		store:   store,
		now:     time.Now,
		poll:    defaultPollInterval,
		retry:   defaultRetryDelay,
		tasks:   map[string]*task{},
		nudge:   make(chan struct{}, 1),
		runCtx:  context.Background(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Schedule registers fn to run every interval as the job called name. A task
// with no stored last run is due immediately once the scheduler starts.
func (s *Scheduler) Schedule(name string, interval time.Duration, fn Func) error {
	if name == "" || interval <= 0 || fn == nil {
		return fmt.Errorf("schedule %q: %w", name, ErrInvalidSchedule)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.tasks[name]; dup {
		return fmt.Errorf("schedule %q: already scheduled: %w", name, ErrInvalidSchedule)
	}
	t := &task{name: name, interval: interval, fn: fn}
	s.tasks[name] = t
	s.order = append(s.order, t)
	return nil
}

// Start launches the scheduling loop. It stops when ctx is canceled or
// Shutdown is called. Calling Start twice does nothing.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	loopCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.runCtx = context.WithoutCancel(ctx)
	done := s.done
	s.mu.Unlock()

	go func() {
		defer close(done)
		s.loop(loopCtx)
	}()
}

// Shutdown stops the loop and waits for it within ctx. Running tasks belong to
// the Manager; Manager.Shutdown cancels them.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop scheduler: %w", ctx.Err())
	}
}

// Nudge asks the scheduler to check for due tasks now. Call it after a network
// reconnect, a window gaining focus, or a system resume.
func (s *Scheduler) Nudge() {
	select {
	case s.nudge <- struct{}{}:
	default:
	}
}

// RunNow starts the task immediately. When it succeeds the interval restarts
// from that moment. It returns ErrRunning if the task is already running and
// ErrUnknownTask if it was never scheduled.
func (s *Scheduler) RunNow(name string) error {
	s.mu.Lock()
	t := s.tasks[name]
	s.mu.Unlock()
	if t == nil {
		return fmt.Errorf("run %q: %w", name, ErrUnknownTask)
	}
	return s.start(t)
}

// NextRun reports when the task is next due. A task that is due now reports
// the current time.
func (s *Scheduler) NextRun(name string) (time.Time, bool) {
	s.mu.Lock()
	t := s.tasks[name]
	s.mu.Unlock()
	if t == nil {
		return time.Time{}, false
	}
	s.load(t)
	now := s.wall()
	s.mu.Lock()
	defer s.mu.Unlock()
	return nextDue(t, now), true
}

func (s *Scheduler) wall() time.Time { return s.now().UTC().Round(0) }

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.poll)
	defer ticker.Stop()
	for {
		if !s.runDue(ctx) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.nudge:
		}
	}
}

// runDue starts every due task. It returns false once the manager is stopped.
func (s *Scheduler) runDue(ctx context.Context) bool {
	s.mu.Lock()
	tasks := append([]*task(nil), s.order...)
	s.mu.Unlock()
	for _, t := range tasks {
		if ctx.Err() != nil {
			return false
		}
		s.load(t)
		now := s.wall()
		s.mu.Lock()
		due := !nextDue(t, now).After(now)
		s.mu.Unlock()
		if !due {
			continue
		}
		if err := s.start(t); errors.Is(err, ErrStopped) {
			return false
		}
	}
	return true
}

// nextDue is called with s.mu held and the task loaded. It may clamp a last
// run that lies in the future.
func nextDue(t *task, now time.Time) time.Time {
	due := now
	if t.hasLast {
		if t.last.After(now) {
			// The clock moved backwards; restart the interval from now so the task cannot stall.
			t.last = now
		}
		due = t.last.Add(t.interval)
	}
	if t.retryAt.After(due) {
		due = t.retryAt
	}
	return due
}

// load reads the persisted last run once. A store failure is reported and
// the task falls back to in-memory state.
func (s *Scheduler) load(t *task) {
	s.mu.Lock()
	if t.loaded {
		s.mu.Unlock()
		return
	}
	t.loaded = true
	s.mu.Unlock()

	at, ok, err := s.store.LastRun(s.runCtx, t.name)
	if err != nil {
		errorreport.Capture(fmt.Errorf("read last run of %q: %w", t.name, err), "schedule."+t.name, "jobs")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A run that finished while loading is newer than anything stored.
	if ok && !t.hasLast {
		t.last, t.hasLast = at.UTC().Round(0), true
	}
}

func (s *Scheduler) start(t *task) error {
	s.load(t)
	return s.manager.Start(t.name, func(ctx context.Context, report Report) error {
		succeeded := false
		defer func() {
			if !succeeded {
				s.failed(t)
			}
		}()
		if err := t.fn(ctx, report); err != nil {
			return err
		}
		succeeded = true
		s.succeeded(t)
		return nil
	})
}

func (s *Scheduler) succeeded(t *task) {
	at := s.wall()
	s.mu.Lock()
	t.last, t.hasLast, t.retryAt = at, true, time.Time{}
	s.mu.Unlock()
	if err := s.store.SetLastRun(s.runCtx, t.name, at); err != nil {
		errorreport.Capture(fmt.Errorf("save last run of %q: %w", t.name, err), "schedule."+t.name, "jobs")
	}
}

func (s *Scheduler) failed(t *task) {
	at := s.wall().Add(s.retry)
	s.mu.Lock()
	t.retryAt = at
	s.mu.Unlock()
}
