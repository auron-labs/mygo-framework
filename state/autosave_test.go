package state_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/state"
)

type recorder struct {
	mu    sync.Mutex
	saved []int
	fail  atomic.Bool
}

func (r *recorder) save(_ context.Context, v int) error {
	if r.fail.Load() {
		return errors.New("disk full")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = append(r.saved, v)
	return nil
}

func (r *recorder) last() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.saved) == 0 {
		return -1, 0
	}
	return r.saved[len(r.saved)-1], len(r.saved)
}

func TestAutosaveCoalescesBurstsAndFlushWaits(t *testing.T) {
	cell := state.NewCell(0)
	rec := &recorder{}
	saver := state.Autosave(cell, 20*time.Millisecond, rec.save)
	for i := 1; i <= 50; i++ {
		cell.Set(i)
	}
	if err := saver.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if last, _ := rec.last(); last != 50 {
		t.Fatalf("last saved = %d, want 50", last)
	}
	if err := saver.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, n := rec.last(); n > 5 {
		t.Fatalf("%d saves for one burst", n)
	}
}

func TestAutosaveSavesAfterDelayWithoutFlush(t *testing.T) {
	cell := state.NewCell(0)
	rec := &recorder{}
	saver := state.Autosave(cell, time.Millisecond, rec.save)
	defer func() { _ = saver.Close(t.Context()) }()
	cell.Set(7)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if last, _ := rec.last(); last == 7 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("change was never saved")
}

func TestAutosaveFailureIsRetriedOnFlushAndReportedByErr(t *testing.T) {
	cell := state.NewCell(0)
	rec := &recorder{}
	rec.fail.Store(true)
	saver := state.Autosave(cell, time.Hour, rec.save)
	defer func() { _ = saver.Close(t.Context()) }()
	errChanged := make(chan struct{}, 4)
	saver.Err().Subscribe(func(error) { errChanged <- struct{}{} })

	cell.Set(1)
	if err := saver.Flush(t.Context()); err == nil {
		t.Fatal("Flush hid the save failure")
	}
	<-errChanged
	if saver.Err().Get() == nil {
		t.Fatal("Err is nil after a failed save")
	}

	rec.fail.Store(false)
	if err := saver.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if last, _ := rec.last(); last != 1 {
		t.Fatalf("last saved = %d, want 1", last)
	}
	<-errChanged
	if saver.Err().Get() != nil {
		t.Fatal("Err not cleared by a successful save")
	}
}

func TestAutosaveChangeDuringSaveTriggersFollowUp(t *testing.T) {
	cell := state.NewCell(0)
	started := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	var saved []int
	saver := state.Autosave(cell, time.Hour, func(_ context.Context, v int) error {
		if v == 1 {
			close(started)
			<-release
		}
		mu.Lock()
		saved = append(saved, v)
		mu.Unlock()
		return nil
	})
	cell.Set(1)
	flushed := make(chan error, 1)
	go func() { flushed <- saver.Flush(t.Context()) }()
	<-started
	cell.Set(2)
	close(release)
	if err := <-flushed; err != nil {
		t.Fatal(err)
	}
	if err := saver.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(saved) != 2 || saved[1] != 2 {
		t.Fatalf("saved = %v, want [1 2]", saved)
	}
}

func TestAutosaveCloseFlushesAndIsIdempotent(t *testing.T) {
	cell := state.NewCell(0)
	rec := &recorder{}
	saver := state.Autosave(cell, time.Hour, rec.save)
	cell.Set(3)

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Go(func() { errs[i] = saver.Close(t.Context()) })
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if last, n := rec.last(); last != 3 || n != 1 {
		t.Fatalf("saved %d in %d saves", last, n)
	}
	cell.Set(4)
	if _, n := rec.last(); n != 1 {
		t.Fatal("saved after Close")
	}
}

func TestAutosaveCloseReturnsWhenContextExpires(t *testing.T) {
	cell := state.NewCell(0)
	canceled := make(chan struct{})
	saver := state.Autosave(cell, time.Hour, func(ctx context.Context, _ int) error {
		<-ctx.Done()
		close(canceled)
		return ctx.Err()
	})
	cell.Set(1)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := saver.Close(ctx); err == nil {
		t.Fatal("Close returned nil although the save never finished")
	}
	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight save was not canceled")
	}
}
