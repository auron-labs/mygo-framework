package runtime

import (
	"context"
	"testing"
	"time"
)

func TestLifetimeWorkerStopsWithApplicationContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	worker := NewLifetimeWorker(ctx)
	cancel()
	waitCtx, stopWaiting := context.WithTimeout(t.Context(), time.Second)
	defer stopWaiting()

	select {
	case <-worker.Done():
	case <-waitCtx.Done():
		t.Fatal("worker did not stop with application context")
	}
	if err := worker.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown stopped worker: %v", err)
	}
}

func TestLifetimeWorkerShutdownIsIdempotent(t *testing.T) {
	t.Parallel()

	worker := NewLifetimeWorker(t.Context())
	if err := worker.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown worker: %v", err)
	}
	if err := worker.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown worker again: %v", err)
	}
}
