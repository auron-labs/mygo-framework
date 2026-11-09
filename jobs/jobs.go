// Package jobs runs named background jobs with progress, cancellation, and
// completion events.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// State is where a job is in its lifecycle.
type State string

// Job states. Running is the only non-terminal state.
const (
	Running  State = "running"
	Done     State = "done"
	Canceled State = "canceled"
	Failed   State = "failed"
)

var (
	// ErrRunning is returned by Start when a job with the same name is still running.
	ErrRunning = errors.New("job is already running")
	// ErrStopped is returned by Start after Shutdown.
	ErrStopped = errors.New("job manager is stopped")
)

// Status is a snapshot of a job, delivered to subscribers on every change.
type Status struct {
	Name    string
	State   State
	Done    int
	Total   int
	Message string
	// Err is the failure text when State is Failed.
	Err string
}

// Fraction returns progress from 0 to 1, or -1 while the total is unknown,
// which progress bars show as indeterminate.
func (s Status) Fraction() float64 {
	if s.Total <= 0 {
		return -1
	}
	return min(float64(s.Done)/float64(s.Total), 1)
}

// Report records progress. A total of zero or less means the total is unknown.
type Report func(done, total int, message string)

// Func is the work of a job. It must return promptly once ctx is canceled.
// Returning an error that wraps context.Canceled ends the job as Canceled.
type Func func(ctx context.Context, report Report) error

// Manager runs at most one job per name.
type Manager struct {
	ctx    context.Context //nolint:containedctx // The manager's lifetime context parents every job.
	cancel context.CancelFunc

	mu        sync.Mutex
	wg        sync.WaitGroup
	stopped   bool
	jobs      map[string]*job
	listeners map[int]func(Status)
	nextID    int
}

type job struct {
	status Status
	cancel context.CancelFunc
}

// New returns a manager whose jobs stop when parent is canceled or Shutdown is called.
func New(parent context.Context) *Manager {
	ctx, cancel := context.WithCancel(parent)
	return &Manager{ctx: ctx, cancel: cancel, jobs: map[string]*job{}, listeners: map[int]func(Status){}}
}

// Start runs fn in a new goroutine. It returns ErrRunning if a job named name
// has not finished yet, so callers decide whether to cancel or wait; a finished
// job's name can be reused. A panic in fn ends the job as Failed.
func (m *Manager) Start(name string, fn Func) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return ErrStopped
	}
	if existing := m.jobs[name]; existing != nil && existing.status.State == Running {
		m.mu.Unlock()
		return fmt.Errorf("start job %q: %w", name, ErrRunning)
	}
	ctx, cancel := context.WithCancel(m.ctx)
	j := &job{status: Status{Name: name, State: Running}, cancel: cancel}
	m.jobs[name] = j
	m.wg.Add(1)
	listeners := m.listenersLocked()
	m.mu.Unlock()

	// Announced before the goroutine starts so Running is always the first event.
	notify(listeners, j.status)
	go m.run(ctx, j, fn)
	return nil
}

func (m *Manager) run(ctx context.Context, j *job, fn Func) {
	defer m.wg.Done()
	defer j.cancel()
	err := call(ctx, fn, func(done, total int, message string) {
		m.update(j, func(s *Status) { s.Done, s.Total, s.Message = done, total, message })
	})
	m.update(j, func(s *Status) {
		switch {
		case err == nil:
			s.State = Done
		case errors.Is(err, context.Canceled):
			s.State = Canceled
		default:
			s.State, s.Err = Failed, err.Error()
		}
	})
}

func call(ctx context.Context, fn Func, report Report) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
		}
	}()
	return fn(ctx, report)
}

// update applies change to a running job and notifies subscribers. Reports
// that arrive after the job finished are dropped.
func (m *Manager) update(j *job, change func(*Status)) {
	m.mu.Lock()
	if j.status.State != Running {
		m.mu.Unlock()
		return
	}
	change(&j.status)
	status, listeners := j.status, m.listenersLocked()
	m.mu.Unlock()
	notify(listeners, status)
}

// Cancel asks the named job to stop. It reports whether a running job was found.
func (m *Manager) Cancel(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.jobs[name]
	if j == nil || j.status.State != Running {
		return false
	}
	j.cancel()
	return true
}

// Status returns the latest status of the named job, including after it finished.
func (m *Manager) Status(name string) (Status, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := m.jobs[name]
	if j == nil {
		return Status{}, false
	}
	return j.status, true
}

// Subscribe calls fn for every status change, from the goroutine that caused
// it, so fn must not block; native views hand the status to Window.Update.
// The returned function stops delivery.
func (m *Manager) Subscribe(fn func(Status)) (unsubscribe func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	m.listeners[id] = fn
	return func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.listeners, id)
	}
}

func (m *Manager) listenersLocked() []func(Status) {
	out := make([]func(Status), 0, len(m.listeners))
	for _, fn := range m.listeners {
		out = append(out, fn)
	}
	return out
}

func notify(listeners []func(Status), status Status) {
	for _, fn := range listeners {
		fn(status)
	}
}

// Shutdown rejects new jobs, cancels running ones, and waits for them within ctx.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	m.cancel()
	finished := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop background jobs: %w", ctx.Err())
	}
}
