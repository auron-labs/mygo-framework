package state

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Saver persists a [Source] after it changes. Create one with [Autosave].
type Saver[T any] struct {
	src     Source[T]
	delay   time.Duration
	save    func(context.Context, T) error
	err     *Cell[error]
	wake    chan struct{}
	stopSub func()
	stopRun context.CancelFunc
	exited  chan struct{}

	mu       sync.Mutex
	saved    uint64
	inflight *saveCall

	closeOnce sync.Once
	closeErr  error
}

type saveCall struct {
	version uint64
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
}

// Autosave calls save with the latest value of src once it has stayed
// unchanged for delay. The value present when Autosave is called counts as
// already saved.
//
// Saves run on a separate goroutine, never on the writer's, and never overlap.
// Changes made during a save cause one follow-up save of the latest value. A
// failed save stays pending and is retried on the next change or [Saver.Flush];
// there is no retry loop. Failures are reported through [Saver.Err].
func Autosave[T any](src Source[T], delay time.Duration, save func(context.Context, T) error) *Saver[T] {
	_, version := src.snapshot()
	runCtx, stopRun := context.WithCancel(context.Background()) //nolint:gosec // Close cancels it.
	s := &Saver[T]{
		src: src, delay: delay, save: save,
		err:     NewCell[error](nil),
		wake:    make(chan struct{}, 1),
		stopRun: stopRun,
		exited:  make(chan struct{}),
		saved:   version,
	}
	s.stopSub = src.Subscribe(func(T) {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	})
	go s.run(runCtx)
	return s
}

// Err reports the outcome of the latest save: nil after a successful save,
// otherwise the latest failure.
func (s *Saver[T]) Err() Source[error] { return s.err }

func (s *Saver[T]) run(ctx context.Context) {
	defer close(s.exited)
	for {
		select {
		case <-s.wake:
		case <-ctx.Done():
			return
		}
		timer := time.NewTimer(s.delay)
	debounce:
		for {
			select {
			case <-s.wake:
				timer.Reset(s.delay)
			case <-timer.C:
				break debounce
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
		// A failure is reported through Err.
		_ = s.Flush(ctx)
	}
}

// Flush saves the latest value now if it is newer than the last saved one, and
// waits for that save or for ctx. Overlapping flushes share one save. If ctx
// ends first, the save keeps running.
func (s *Saver[T]) Flush(ctx context.Context) error {
	for {
		value, version := s.src.snapshot()
		s.mu.Lock()
		if version <= s.saved {
			s.mu.Unlock()
			return nil
		}
		call := s.inflight
		if call == nil {
			call = s.begin(value, version) //nolint:contextcheck // The save must outlive a caller that stops waiting.
		}
		s.mu.Unlock()
		select {
		case <-call.done:
		case <-ctx.Done():
			return fmt.Errorf("wait for save: %w", ctx.Err())
		}
		if call.version >= version {
			return call.err
		}
	}
}

// begin starts a save of value; the caller holds s.mu.
func (s *Saver[T]) begin(value T, version uint64) *saveCall {
	ctx, cancel := context.WithCancel(context.Background())
	call := &saveCall{version: version, cancel: cancel, done: make(chan struct{})}
	s.inflight = call
	go func() {
		defer cancel()
		err := s.save(ctx, value)
		s.mu.Lock()
		if err == nil {
			s.saved = max(s.saved, version)
		}
		s.inflight = nil
		call.err = err
		s.mu.Unlock()
		if err != nil || s.err.Get() != nil {
			s.err.Set(err)
		}
		close(call.done)
	}()
	return call
}

// Close stops watching the source, saves any unsaved change, and stops the
// background goroutine. If ctx ends first, the save in progress is canceled
// and Close returns the error. Repeated and concurrent calls return the result
// of the first.
func (s *Saver[T]) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		s.stopSub()
		s.stopRun()
		<-s.exited
		s.closeErr = s.Flush(ctx)
		if s.closeErr != nil {
			s.mu.Lock()
			if s.inflight != nil {
				s.inflight.cancel()
			}
			s.mu.Unlock()
		}
	})
	return s.closeErr
}
