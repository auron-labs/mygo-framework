package jobs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/auron-labs/mygo-framework/jobs"
)

// collect subscribes before starting and returns every status of the job up to its terminal one.
func collect(t *testing.T, m *jobs.Manager, name string, fn jobs.Func) []jobs.Status {
	t.Helper()
	events := make(chan jobs.Status, 16)
	unsubscribe := m.Subscribe(func(s jobs.Status) { events <- s })
	defer unsubscribe()
	if err := m.Start(name, fn); err != nil {
		t.Fatal(err)
	}
	var got []jobs.Status
	for s := range events {
		got = append(got, s)
		if s.State != jobs.Running {
			return got
		}
	}
	return got
}

func TestJobReportsProgressAndCompletes(t *testing.T) {
	m := jobs.New(t.Context())
	got := collect(t, m, "sync", func(_ context.Context, report jobs.Report) error {
		report(1, 4, "one")
		report(4, 4, "four")
		return nil
	})
	want := []jobs.Status{
		{Name: "sync", State: jobs.Running},
		{Name: "sync", State: jobs.Running, Done: 1, Total: 4, Message: "one"},
		{Name: "sync", State: jobs.Running, Done: 4, Total: 4, Message: "four"},
		{Name: "sync", State: jobs.Done, Done: 4, Total: 4, Message: "four"},
	}
	if len(got) != len(want) {
		t.Fatalf("events = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if f := got[2].Fraction(); f != 1 {
		t.Errorf("fraction = %v", f)
	}
	if status, ok := m.Status("sync"); !ok || status.State != jobs.Done {
		t.Fatalf("status = %+v, %v", status, ok)
	}
}

func TestCancelStopsJob(t *testing.T) {
	m := jobs.New(t.Context())
	started := make(chan struct{})
	events := make(chan jobs.Status, 16)
	m.Subscribe(func(s jobs.Status) { events <- s })
	err := m.Start("export", func(ctx context.Context, _ jobs.Report) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if !m.Cancel("export") {
		t.Fatal("running job was not found")
	}
	var last jobs.Status
	for last = range events {
		if last.State != jobs.Running {
			break
		}
	}
	if last.State != jobs.Canceled || last.Err != "" {
		t.Fatalf("final = %+v", last)
	}
	if m.Cancel("export") || m.Cancel("missing") {
		t.Fatal("Cancel found a job that is not running")
	}
}

func TestPanicBecomesJobError(t *testing.T) {
	m := jobs.New(t.Context())
	got := collect(t, m, "boom", func(context.Context, jobs.Report) error { panic("exploded") })
	last := got[len(got)-1]
	if last.State != jobs.Failed || last.Err != "job panicked: exploded" {
		t.Fatalf("final = %+v", last)
	}
}

func TestErrorFailsJob(t *testing.T) {
	m := jobs.New(t.Context())
	got := collect(t, m, "bad", func(context.Context, jobs.Report) error { return errors.New("disk full") })
	if last := got[len(got)-1]; last.State != jobs.Failed || last.Err != "disk full" {
		t.Fatalf("final = %+v", last)
	}
}

func TestStartSameNameWhileRunning(t *testing.T) {
	m := jobs.New(t.Context())
	release := make(chan struct{})
	if err := m.Start("a", func(context.Context, jobs.Report) error { <-release; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := m.Start("a", func(context.Context, jobs.Report) error { return nil }); !errors.Is(err, jobs.ErrRunning) {
		t.Fatalf("second Start = %v", err)
	}
	// A different name is independent.
	if err := m.Start("b", func(context.Context, jobs.Report) error { return nil }); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := m.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := m.Start("a", nil); !errors.Is(err, jobs.ErrStopped) {
		t.Fatalf("Start after Shutdown = %v", err)
	}
}

func TestNameIsReusableAfterFinish(t *testing.T) {
	m := jobs.New(t.Context())
	fn := func(context.Context, jobs.Report) error { return nil }
	collect(t, m, "a", fn)
	collect(t, m, "a", fn)
}

func TestShutdownCancelsAndWaits(t *testing.T) {
	m := jobs.New(t.Context())
	started := make(chan struct{})
	var stopped bool
	err := m.Start("long", func(ctx context.Context, _ jobs.Report) error {
		close(started)
		<-ctx.Done()
		stopped = true
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := m.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !stopped {
		t.Fatal("Shutdown returned before the job stopped")
	}
}

func TestShutdownHonorsDeadline(t *testing.T) {
	m := jobs.New(t.Context())
	release := make(chan struct{})
	defer close(release)
	if err := m.Start("stuck", func(context.Context, jobs.Report) error { <-release; return nil }); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := m.Shutdown(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Shutdown = %v", err)
	}
}
