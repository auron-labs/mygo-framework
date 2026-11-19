// Package jobs runs named background jobs with progress, cancellation, and
// completion events. Jobs can share a named lane with a concurrency limit;
// jobs over the limit wait as Queued.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/auron-labs/mygo-framework/errorreport"
)

// State is where a job is in its lifecycle.
type State string

// Job states. Queued and Running are the non-terminal states; a job only
// becomes Queued when its lane is full.
const (
	Queued   State = "queued"
	Running  State = "running"
	Done     State = "done"
	Canceled State = "canceled"
	Failed   State = "failed"
)

var (
	// ErrRunning is returned by Start when a job with the same name is still running.
	ErrRunning = errors.New("job is already running")
	// ErrUnknownLane is returned by StartIn for a lane that SetLane never defined.
	ErrUnknownLane = errors.New("unknown job lane")
	// ErrEmptyLane is returned by SetLane for an empty lane name.
	ErrEmptyLane = errors.New("lane name must not be empty")
	// ErrInvalidLimit is returned by SetLane for a limit below one.
	ErrInvalidLimit = errors.New("lane limit must be at least 1")
	// ErrStopped is returned by Start after Shutdown.
	ErrStopped = errors.New("job manager is stopped")

	errPanicked = errors.New("job panicked")
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
	// Lane is the lane the job runs in, or empty for a job started with Start.
	Lane string
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

// Manager runs at most one job per name. Jobs started with StartIn also
// respect their lane's concurrency limit.
type Manager struct {
	ctx    context.Context //nolint:containedctx // The manager's lifetime context parents every job.
	cancel context.CancelFunc

	mu        sync.Mutex
	wg        sync.WaitGroup
	stopped   bool
	jobs      map[string]*job
	listeners map[int]func(Status)
	nextID    int
	lanes     map[string]*lane
	nextSeq   int
	pending   []pendingEvent
	draining  bool
	emitted   int64
	delivered int64
	flushed   *sync.Cond
}

type pendingEvent struct {
	listeners []func(Status)
	status    Status
}

// lane limits how many of its jobs run at once; the rest wait in queue, oldest first.
type lane struct {
	limit   int
	running int
	queue   []*job
}

type job struct {
	status Status
	ctx    context.Context //nolint:containedctx // Created when the job is enqueued so cancellation reaches queued jobs.
	cancel context.CancelFunc
	fn     Func
	lane   *lane
	seq    int
	// startEvent is the number of the event that announced the job as Running.
	startEvent int64
}

// New returns a manager whose jobs stop when parent is canceled or Shutdown is called.
func New(parent context.Context) *Manager {
	ctx, cancel := context.WithCancel(parent)
	m := &Manager{ctx: ctx, cancel: cancel, jobs: map[string]*job{}, listeners: map[int]func(Status){}, lanes: map[string]*lane{}}
	m.flushed = sync.NewCond(&m.mu)
	// Queued jobs must not outlive a canceled parent, even if no job is running to free a slot.
	context.AfterFunc(ctx, m.cancelQueued)
	return m
}

// cancelQueued ends every queued job as Canceled without running it.
func (m *Manager) cancelQueued() {
	m.mu.Lock()
	for _, l := range m.lanes {
		for _, j := range l.queue {
			m.cancelQueuedLocked(j)
		}
		l.queue = nil
	}
	m.mu.Unlock()
	m.drain()
}

// waitDelivered blocks until event number n has been published to listeners.
func (m *Manager) waitDelivered(n int64) {
	m.mu.Lock()
	for m.delivered < n {
		m.flushed.Wait()
	}
	m.mu.Unlock()
}

// flush blocks until every event emitted so far has been published.
func (m *Manager) flush() {
	m.mu.Lock()
	n := m.emitted
	m.mu.Unlock()
	m.waitDelivered(n)
}

// SetLane defines the lane name, or changes its limit, so at most limit of its
// jobs run at once. Raising the limit starts queued jobs; lowering it never
// interrupts running ones.
func (m *Manager) SetLane(name string, limit int) error {
	if name == "" {
		return fmt.Errorf("set lane: %w", ErrEmptyLane)
	}
	if limit < 1 {
		return fmt.Errorf("set lane %q: %w", name, ErrInvalidLimit)
	}
	m.mu.Lock()
	l := m.lanes[name]
	if l == nil {
		l = &lane{}
		m.lanes[name] = l
	}
	l.limit = limit
	started := m.promoteLocked(l)
	m.mu.Unlock()
	m.launch(started)
	return nil
}

// Start runs fn in a new goroutine, outside any lane. It returns ErrRunning if
// a job named name has not finished yet, so callers decide whether to cancel
// or wait; a finished job's name can be reused. A panic in fn ends the job as
// Failed.
func (m *Manager) Start(name string, fn Func) error {
	return m.start("", name, fn)
}

// StartIn is like Start but runs the job in the named lane. When the lane
// already has its limit of running jobs the job is Queued and starts, in the
// order queued, as running ones finish. The name must not belong to a queued or
// running job. It returns ErrUnknownLane if SetLane never defined the lane.
func (m *Manager) StartIn(laneName, name string, fn Func) error {
	if laneName == "" {
		return fmt.Errorf("start job %q: %w", name, ErrUnknownLane)
	}
	return m.start(laneName, name, fn)
}

func (m *Manager) start(laneName, name string, fn Func) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return ErrStopped
	}
	var l *lane
	if laneName != "" {
		if l = m.lanes[laneName]; l == nil {
			m.mu.Unlock()
			return fmt.Errorf("start job %q in lane %q: %w", name, laneName, ErrUnknownLane)
		}
	}
	if existing := m.jobs[name]; existing != nil && (existing.status.State == Running || existing.status.State == Queued) {
		m.mu.Unlock()
		return fmt.Errorf("start job %q: %w", name, ErrRunning)
	}
	ctx, cancel := context.WithCancel(m.ctx)
	j := &job{
		status: Status{Name: name, Lane: laneName, State: Running},
		ctx:    ctx, cancel: cancel, fn: fn, lane: l, seq: m.nextSeq,
	}
	m.nextSeq++
	m.jobs[name] = j
	m.wg.Add(1)
	if l != nil && l.running >= l.limit {
		j.status.State = Queued
		l.queue = append(l.queue, j)
	} else if l != nil {
		l.running++
	}
	// Queued under the lock, so no other goroutine can publish a later event first.
	ev := m.emitLocked(j.status)
	state := j.status.State
	if state == Running {
		j.startEvent = ev
	}
	m.mu.Unlock()

	m.drain()
	if state == Running {
		go m.run(j)
	}
	return nil
}

// promoteLocked moves queued jobs into free slots of l. It returns the jobs to
// start; their status events are queued for publication. Jobs whose context is
// already canceled finish as Canceled without running.
func (m *Manager) promoteLocked(l *lane) (started []*job) {
	for !m.stopped && l.running < l.limit && len(l.queue) > 0 {
		j := l.queue[0]
		l.queue = l.queue[1:]
		if j.ctx.Err() != nil {
			j.status.State = Canceled
			j.cancel()
			m.wg.Done()
			m.emitLocked(j.status)
			continue
		}
		j.status.State = Running
		l.running++
		started = append(started, j)
		j.startEvent = m.emitLocked(j.status)
	}
	return started
}

// launch publishes pending events and starts the promoted jobs.
func (m *Manager) launch(started []*job) {
	m.drain()
	for _, j := range started {
		go m.run(j)
	}
}

func (m *Manager) run(j *job) {
	defer m.wg.Done()
	// Runs before wg.Done, so Shutdown returns only after every event is published.
	defer m.flush()
	// The job's own Running event reaches listeners before its work begins.
	m.waitDelivered(j.startEvent)
	defer j.cancel()
	name := j.status.Name
	err := call(j.ctx, name, j.fn, func(done, total int, message string) {
		m.update(j, func(s *Status) { s.Done, s.Total, s.Message = done, total, message })
	})
	// call has already reported a panic with its stack.
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, errPanicked) {
		errorreport.Capture(err, "job."+name, "jobs")
	}
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
	if j.lane == nil {
		return
	}
	// Free the slot only after the terminal event, then hand it to the next queued job.
	m.mu.Lock()
	j.lane.running--
	started := m.promoteLocked(j.lane)
	m.mu.Unlock()
	m.launch(started)
}

func call(ctx context.Context, name string, fn Func, report Report) (err error) {
	defer func() {
		if r := recover(); r != nil {
			errorreport.CapturePanic(r, "job."+name, "jobs")
			err = fmt.Errorf("%w: %v", errPanicked, r)
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
	m.emitLocked(j.status)
	m.mu.Unlock()
	m.drain()
}

// Cancel asks the named job to stop. A queued job is removed from its lane and
// ends as Canceled without running. It reports whether a queued or running job
// was found.
func (m *Manager) Cancel(name string) bool {
	m.mu.Lock()
	j := m.jobs[name]
	if j == nil || (j.status.State != Running && j.status.State != Queued) {
		m.mu.Unlock()
		return false
	}
	if j.status.State == Running {
		j.cancel()
		m.mu.Unlock()
		return true
	}
	j.lane.queue = slices.DeleteFunc(j.lane.queue, func(q *job) bool { return q == j })
	m.cancelQueuedLocked(j)
	m.mu.Unlock()
	m.drain()
	return true
}

// cancelQueuedLocked ends a job that was removed from its queue without running.
func (m *Manager) cancelQueuedLocked(j *job) {
	j.status.State = Canceled
	j.cancel()
	m.wg.Done()
	m.emitLocked(j.status)
}

// List returns the latest status of every job, including finished ones, in the
// order they were started.
func (m *Manager) List() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make([]*job, 0, len(m.jobs))
	for _, j := range m.jobs {
		all = append(all, j)
	}
	slices.SortFunc(all, func(a, b *job) int { return a.seq - b.seq })
	out := make([]Status, len(all))
	for i, j := range all {
		out[i] = j.status
	}
	return out
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

// Subscribe calls fn for every status change, so fn must not block; native
// views hand the status to Window.Update. Events are delivered one at a time,
// in the order the changes happened, by whichever goroutine is publishing,
// which may not be the one that caused the change. A job's work starts only
// after its Running event is delivered, and Shutdown returns only after every
// event has been delivered. The returned function stops delivery.
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

// emitLocked queues a status event. Events are queued under m.mu, at the
// moment the state changes, so every job's events are published in order.
func (m *Manager) emitLocked(status Status) int64 {
	m.pending = append(m.pending, pendingEvent{listeners: m.listenersLocked(), status: status})
	m.emitted++
	return m.emitted
}

// drain publishes queued events in order. Only one goroutine publishes at a
// time; one that finds another publishing leaves its events to it.
func (m *Manager) drain() {
	m.mu.Lock()
	if m.draining {
		m.mu.Unlock()
		return
	}
	m.draining = true
	for len(m.pending) > 0 {
		e := m.pending[0]
		m.pending = m.pending[1:]
		m.mu.Unlock()
		for _, fn := range e.listeners {
			fn(e.status)
		}
		m.mu.Lock()
		m.delivered++
		m.flushed.Broadcast()
	}
	m.draining = false
	m.mu.Unlock()
}

// Shutdown rejects new jobs, cancels queued jobs without running them, cancels
// running ones, and waits for them within ctx.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	m.cancelQueued()
	m.cancel()
	finished := make(chan struct{})
	go func() {
		m.wg.Wait()
		m.flush()
		close(finished)
	}()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop background jobs: %w", ctx.Err())
	}
}
