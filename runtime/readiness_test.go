package runtime

import (
	"context"
	"errors"
	"mygo-framework/problem"
	"sync"
	"testing"
)

func TestReadinessReleasesConcurrentWaiters(t *testing.T) {
	t.Parallel()

	readiness := NewReadiness()
	const waiterCount = 32

	results := make(chan error, waiterCount)
	var waiters sync.WaitGroup
	waiters.Add(waiterCount)
	for range waiterCount {
		go func() {
			defer waiters.Done()
			results <- readiness.Wait(t.Context())
		}()
	}

	if err := readiness.Succeed(); err != nil {
		t.Fatalf("publish readiness: %v", err)
	}
	waiters.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Errorf("wait for ready runtime: %v", err)
		}
	}
	if got := readiness.State(); got != StateReady {
		t.Fatalf("state = %d, want ready", got)
	}
}

func TestReadinessWaitHonorsCancellation(t *testing.T) {
	t.Parallel()

	readiness := NewReadiness()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := readiness.Wait(ctx)
	var got *problem.Problem
	if !errors.As(err, &got) {
		t.Fatalf("wait error type = %T, want *problem.Problem", err)
	}
	if got.Code != problem.CodeCanceled || !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %#v", got)
	}
}

func TestReadinessPreservesStartupFailure(t *testing.T) {
	t.Parallel()

	readiness := NewReadiness()
	cause := errors.New("open database")
	if err := readiness.Fail(cause); err != nil {
		t.Fatalf("publish startup failure: %v", err)
	}

	err := readiness.Wait(t.Context())
	var got *problem.Problem
	if !errors.As(err, &got) {
		t.Fatalf("wait error type = %T, want *problem.Problem", err)
	}
	if got.Code != problem.CodeUnavailable || !errors.Is(err, cause) {
		t.Fatalf("wait error = %#v", got)
	}
	if got := readiness.State(); got != StateFailed {
		t.Fatalf("state = %d, want failed", got)
	}
}

func TestReadinessRejectsRequestsDuringShutdown(t *testing.T) {
	t.Parallel()

	readiness := NewReadiness()
	if err := readiness.Succeed(); err != nil {
		t.Fatalf("publish readiness: %v", err)
	}
	if !readiness.BeginShutdown() {
		t.Fatal("first BeginShutdown returned false")
	}
	if readiness.BeginShutdown() {
		t.Fatal("second BeginShutdown returned true")
	}

	err := readiness.Wait(t.Context())
	var got *problem.Problem
	if !errors.As(err, &got) || got.Code != problem.CodeUnavailable {
		t.Fatalf("wait during shutdown = %#v", err)
	}
	if err := readiness.FinishShutdown(); err != nil {
		t.Fatalf("finish shutdown: %v", err)
	}
	if got := readiness.State(); got != StateStopped {
		t.Fatalf("state = %d, want stopped", got)
	}
}

func TestShutdownInterruptsStartupWaiters(t *testing.T) {
	t.Parallel()

	readiness := NewReadiness()
	result := make(chan error, 1)
	go func() {
		result <- readiness.Wait(t.Context())
	}()

	if !readiness.BeginShutdown() {
		t.Fatal("BeginShutdown returned false")
	}
	err := <-result
	var got *problem.Problem
	if !errors.As(err, &got) || got.Code != problem.CodeUnavailable {
		t.Fatalf("interrupted wait = %#v", err)
	}
}

func TestReadinessRejectsInvalidTransitions(t *testing.T) {
	t.Parallel()

	readiness := NewReadiness()
	if err := readiness.Fail(nil); err == nil {
		t.Fatal("Fail accepted a nil cause")
	}
	if err := readiness.Succeed(); err != nil {
		t.Fatalf("publish readiness: %v", err)
	}
	if err := readiness.Succeed(); !errors.Is(err, ErrStartupResolved) {
		t.Fatalf("second readiness error = %v", err)
	}
	if err := readiness.FinishShutdown(); !errors.Is(err, ErrShutdownNotStarted) {
		t.Fatalf("premature shutdown completion error = %v", err)
	}
}
