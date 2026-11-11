package errorreport

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/getsentry/sentry-go"
)

// reset returns the process to its state before Init.
func reset(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		dsn.Store(nil)
		disabled.Store(false)
		buildDSN = ""
		sentry.CurrentHub().BindClient(nil)
	})
}

type collector struct {
	mu     sync.Mutex
	bodies []string
}

func (c *collector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	c.bodies = append(c.bodies, r.URL.Path+"\n"+string(body))
	c.mu.Unlock()
	_, _ = w.Write([]byte("{}"))
}

func (c *collector) all() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.bodies, "\n")
}

func TestDeliversErrorsAndPanicsToTheConfiguredDSN(t *testing.T) {
	reset(t)
	events := &collector{}
	server := httptest.NewServer(events)
	t.Cleanup(server.Close)
	t.Setenv(dsnEnvironment, fmt.Sprintf("http://public@%s/42", server.Listener.Addr()))

	if err := Init("1.2.3"); err != nil {
		t.Fatal(err)
	}
	if DSN() == "" {
		t.Fatal("DSN is empty after Init")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	Capture(fmt.Errorf("disk exploded: open %s: denied", filepath.Join(home, "data", "app.db")), "import.run", "jobs")
	CapturePanic("index out of range", "import.run", "jobs")
	Flush()

	sent := events.all()
	// Stack frames name the build machine's files; only the error text holds the user's.
	if strings.Contains(sent, filepath.Join(home, "data")) {
		t.Errorf("delivered events contain a path in the home directory")
	}
	for _, want := range []string{"/api/42/envelope/", "disk exploded: open ~", "index out of range", `"release":"1.2.3"`, "import.run"} {
		if !strings.Contains(sent, want) {
			t.Errorf("delivered events do not contain %q:\n%s", want, sent)
		}
	}
}

func TestSendsNothingWithoutDSN(t *testing.T) {
	reset(t)
	t.Setenv(dsnEnvironment, "")

	if err := Init("1.2.3"); err != nil {
		t.Fatal(err)
	}
	Capture(errors.New("ignored"), "op", "component")
	CapturePanic("ignored", "op", "component")
	Flush()

	// The SDK's default client has no DSN and so no transport.
	if client := sentry.CurrentHub().Client(); DSN() != "" || (client != nil && client.Options().Dsn != "") {
		t.Fatal("error reporting started without a DSN")
	}
}

func TestInvalidDSNLeavesReportingOff(t *testing.T) {
	reset(t)
	t.Setenv(dsnEnvironment, "not a dsn")

	if err := Init("1.2.3"); err == nil {
		t.Fatal("invalid DSN was accepted")
	}
	if DSN() != "" {
		t.Fatal("DSN is set after a failed Init")
	}
}

func TestBuildDSNIsUsedWhenTheEnvironmentHasNone(t *testing.T) {
	reset(t)
	t.Setenv(dsnEnvironment, "")
	buildDSN = "http://public@127.0.0.1:1/7"

	if err := Init("1.2.3"); err != nil {
		t.Fatal(err)
	}
	if DSN() != buildDSN {
		t.Fatalf("DSN = %q, want the build-time DSN", DSN())
	}

	t.Setenv(dsnEnvironment, "http://public@127.0.0.1:1/8")
	if err := Init("1.2.3"); err != nil {
		t.Fatal(err)
	}
	if DSN() != "http://public@127.0.0.1:1/8" {
		t.Fatalf("DSN = %q, want the environment to override the build-time DSN", DSN())
	}
}

func TestDisabledReportingSendsNothing(t *testing.T) {
	reset(t)
	events := &collector{}
	server := httptest.NewServer(events)
	t.Cleanup(server.Close)
	t.Setenv(dsnEnvironment, fmt.Sprintf("http://public@%s/42", server.Listener.Addr()))
	if err := Init("1.2.3"); err != nil {
		t.Fatal(err)
	}

	SetEnabled(false)
	Capture(errors.New("while off"), "op", "component")
	CapturePanic("while off", "op", "component")
	Flush()
	if DSN() != "" || !Configured() {
		t.Fatalf("DSN = %q, Configured = %v, want no DSN but still configured", DSN(), Configured())
	}
	if sent := events.all(); sent != "" {
		t.Fatalf("a disabled client sent %q", sent)
	}

	SetEnabled(true)
	Capture(errors.New("while on"), "op", "component")
	Flush()
	if sent := events.all(); !strings.Contains(sent, "while on") || strings.Contains(sent, "while off") {
		t.Fatalf("after re-enabling, sent = %q", sent)
	}
}
