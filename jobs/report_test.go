package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mygo-framework/errorreport"
	"mygo-framework/jobs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestFailuresAndPanicsAreReportedButCancellationIsNot(t *testing.T) {
	var mu sync.Mutex
	var sent strings.Builder
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		sent.Write(body)
		mu.Unlock()
		_, _ = w.Write([]byte("{}"))
	}))
	t.Cleanup(server.Close)
	t.Setenv("SENTRY_DSN", fmt.Sprintf("http://public@%s/7", server.Listener.Addr()))
	if err := errorreport.Init("test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sentry.CurrentHub().BindClient(nil) })

	m := jobs.New(t.Context())
	collect(t, m, "fails", func(context.Context, jobs.Report) error { return errors.New("disk full") })
	collect(t, m, "panics", func(context.Context, jobs.Report) error { panic("exploded") })
	collect(t, m, "stops", func(context.Context, jobs.Report) error { return fmt.Errorf("stopped: %w", context.Canceled) })
	errorreport.Flush()

	mu.Lock()
	defer mu.Unlock()
	for _, want := range []string{"disk full", "exploded", "job.fails", "job.panics"} {
		if !strings.Contains(sent.String(), want) {
			t.Errorf("reported events do not contain %q", want)
		}
	}
	if strings.Contains(sent.String(), "job.stops") || strings.Contains(sent.String(), "stopped") {
		t.Error("a canceled job was reported")
	}
}
