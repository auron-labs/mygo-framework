package desktop

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestQuitStageCancelsShutsDownAndResumesOnce(t *testing.T) {
	t.Parallel()

	var cancellations atomic.Int32
	var shutdowns atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	resumed := make(chan struct{}, 1)
	stage := NewQuitStage(
		func() { cancellations.Add(1) },
		func(context.Context) error {
			shutdowns.Add(1)
			close(started)
			<-release

			return nil
		},
		func() { resumed <- struct{}{} },
		nil,
		time.Second,
	)

	first := &mygo.QuitEvent{}
	stage.BeforeQuit(first)
	if !first.DefaultPrevented() {
		t.Fatal("first quit was not prevented")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runtime shutdown did not start")
	}
	second := &mygo.QuitEvent{}
	stage.BeforeQuit(second)
	if !second.DefaultPrevented() {
		t.Fatal("quit while cleanup runs was not prevented")
	}
	if got := cancellations.Load(); got != 1 {
		t.Fatalf("application cancellations = %d, want 1", got)
	}
	if got := shutdowns.Load(); got != 1 {
		t.Fatalf("runtime shutdowns = %d, want 1", got)
	}

	close(release)
	select {
	case <-resumed:
	case <-time.After(time.Second):
		t.Fatal("quit did not resume after shutdown")
	}
	resolved := &mygo.QuitEvent{}
	stage.BeforeQuit(resolved)
	if resolved.DefaultPrevented() {
		t.Fatal("resolved quit was prevented")
	}
	if got := shutdowns.Load(); got != 1 {
		t.Fatalf("runtime shutdowns after re-entry = %d, want 1", got)
	}
}

func TestQuitStageResumesAtDeadline(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	resumed := make(chan struct{}, 1)
	reported := make(chan error, 1)
	stage := NewQuitStage(
		func() {},
		func(context.Context) error {
			<-release

			return nil
		},
		func() { resumed <- struct{}{} },
		func(err error) { reported <- err },
		20*time.Millisecond,
	)

	event := &mygo.QuitEvent{}
	stage.BeforeQuit(event)
	if !event.DefaultPrevented() {
		t.Fatal("quit was not prevented during bounded shutdown")
	}
	select {
	case <-resumed:
	case <-time.After(time.Second):
		t.Fatal("quit did not resume at shutdown deadline")
	}
	select {
	case err := <-reported:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("reported error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown deadline was not reported")
	}
	close(release)
}

func TestQuitStageRespectsEarlierCancellation(t *testing.T) {
	t.Parallel()

	var cancellations atomic.Int32
	stage := NewQuitStage(
		func() { cancellations.Add(1) },
		func(context.Context) error {
			t.Fatal("shutdown ran for an already canceled quit")

			return nil
		},
		func() { t.Fatal("already canceled quit resumed") },
		nil,
		time.Second,
	)
	event := &mygo.QuitEvent{}
	event.PreventDefault()
	stage.BeforeQuit(event)
	if got := cancellations.Load(); got != 0 {
		t.Fatalf("application cancellations = %d, want 0", got)
	}
}
