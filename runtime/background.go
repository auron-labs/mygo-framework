package runtime

import (
	"context"
	"fmt"
	"sync"
)

// LifetimeWorker is the single app-lifetime background operation.
type LifetimeWorker struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// NewLifetimeWorker starts work that remains alive until the app context or
// shutdown is canceled.
func NewLifetimeWorker(parent context.Context) *LifetimeWorker {
	ctx, cancel := context.WithCancel(parent) //nolint:gosec // Shutdown owns the app-lifetime cancel function.
	worker := &LifetimeWorker{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(worker.done)
		<-ctx.Done()
	}()

	return worker
}

// Done closes after the worker has stopped.
func (w *LifetimeWorker) Done() <-chan struct{} {
	return w.done
}

// Shutdown cancels the operation and waits for it within ctx.
func (w *LifetimeWorker) Shutdown(ctx context.Context) error {
	w.once.Do(w.cancel)
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop app-lifetime worker: %w", ctx.Err())
	}
}
