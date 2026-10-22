// Package runtime coordinates application startup and shutdown state.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"mygo-framework/problem"
	"sync"
)

// State is the application runtime lifecycle state.
type State uint8

const (
	// StateStarting waits for startup to publish a result.
	StateStarting State = iota
	// StateReady accepts application requests.
	StateReady
	// StateFailed reports a permanent startup failure.
	StateFailed
	// StateStopping rejects new requests while cleanup runs.
	StateStopping
	// StateStopped reports completed cleanup.
	StateStopped
)

var (
	// ErrStartupResolved reports a second startup result.
	ErrStartupResolved = errors.New("runtime startup already resolved")
	// ErrShutdownNotStarted reports completion before shutdown begins.
	ErrShutdownNotStarted = errors.New("runtime shutdown has not started")
)

// Readiness synchronizes request entry with startup and shutdown.
type Readiness struct {
	mu          sync.RWMutex
	startupDone chan struct{}
	state       State
	startupErr  error
}

// NewReadiness constructs a readiness gate in the starting state.
func NewReadiness() *Readiness {
	return &Readiness{
		startupDone: make(chan struct{}),
		state:       StateStarting,
	}
}

// Wait blocks until startup resolves or ctx is canceled.
func (r *Readiness) Wait(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait for application readiness: %w", problem.FromError(err))
		}

		r.mu.RLock()
		state := r.state
		startupErr := r.startupErr
		startupDone := r.startupDone
		r.mu.RUnlock()

		switch state {
		case StateStarting:
			select {
			case <-ctx.Done():
				return fmt.Errorf("wait for application readiness: %w", problem.FromError(ctx.Err()))
			case <-startupDone:
			}
		case StateReady:
			return nil
		case StateFailed:
			return startupErr
		case StateStopping, StateStopped:
			return fmt.Errorf("wait for application readiness: %w", problem.New(
				problem.CodeUnavailable,
				"The application is shutting down.",
			))
		default:
			return fmt.Errorf("wait for application readiness: %w", problem.New(
				problem.CodeInternal,
				"The application runtime is in an invalid state.",
			))
		}
	}
}

// Succeed publishes successful startup to all waiters.
func (r *Readiness) Succeed() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateStarting {
		return fmt.Errorf("publish runtime readiness from state %d: %w", r.state, ErrStartupResolved)
	}

	r.state = StateReady
	close(r.startupDone)

	return nil
}

// Fail publishes a startup failure to all waiters.
func (r *Readiness) Fail(cause error) error {
	if cause == nil {
		return errors.New("runtime startup failure requires a cause")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateStarting {
		return fmt.Errorf("publish runtime failure from state %d: %w", r.state, ErrStartupResolved)
	}

	r.state = StateFailed
	r.startupErr = problem.Wrap(problem.CodeUnavailable, "The application could not start.", cause)
	close(r.startupDone)

	return nil
}

// BeginShutdown rejects new requests and wakes startup waiters.
func (r *Readiness) BeginShutdown() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch r.state {
	case StateStarting:
		close(r.startupDone)
	case StateReady, StateFailed:
	case StateStopping, StateStopped:
		return false
	}

	r.state = StateStopping

	return true
}

// FinishShutdown records completed cleanup.
func (r *Readiness) FinishShutdown() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateStopping {
		return fmt.Errorf("finish runtime shutdown from state %d: %w", r.state, ErrShutdownNotStarted)
	}

	r.state = StateStopped

	return nil
}

// State returns the current lifecycle state.
func (r *Readiness) State() State {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.state
}
