// Package errorreport sends unexpected errors and panics to Sentry. Nothing is
// initialised or sent unless a DSN is configured: SENTRY_DSN in the process
// environment, or a DSN compiled in with
// -ldflags "-X github.com/auron-labs/mygo-framework/errorreport.buildDSN=...".
package errorreport

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	dsnEnvironment = "SENTRY_DSN"
	flushTimeout   = 2 * time.Second
)

// buildDSN is set at build time for packaged applications, which have no
// environment to read the DSN from. SENTRY_DSN overrides it.
var buildDSN string

// dsn is the DSN the client was initialised with; empty while reporting is off.
var dsn atomic.Pointer[string]

// disabled is the user's choice to stop sending reports; see SetEnabled.
var disabled atomic.Bool

// Init starts error reporting for release when a DSN is configured. It does
// nothing without one, and leaves reporting off with an error when the DSN is
// invalid.
func Init(release string) error {
	value := os.Getenv(dsnEnvironment)
	if value == "" {
		value = buildDSN
	}
	if value == "" {
		return nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:     value,
		Release: release,
		// The SDK defaults to sending the machine's host name, which identifies the user's computer.
		ServerName: "desktop",
		BeforeSend: redactHomeDirectory,
	})
	if err != nil {
		return fmt.Errorf("start error reporting: %w", err)
	}
	dsn.Store(&value)
	return nil
}

// redactHomeDirectory keeps the user's name out of reports: file errors quote
// paths, and the data and log directories live under the home directory.
func redactHomeDirectory(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return event
	}
	event.Message = strings.ReplaceAll(event.Message, home, "~")
	for i := range event.Exception {
		event.Exception[i].Value = strings.ReplaceAll(event.Exception[i].Value, home, "~")
	}
	return event
}

// SetEnabled lets the user turn reporting off and on. Reports are on by
// default when a DSN is configured; a disabled client sends nothing, and DSN
// returns "" so the frontend stops too.
func SetEnabled(enabled bool) { disabled.Store(!enabled) }

// Configured reports whether a DSN is set, whatever the user's choice, so an
// application shows the switch only when it has something to switch.
func Configured() bool { return dsn.Load() != nil }

// DSN returns the DSN errors are reported to, or "" when reporting is off. The
// frontend reads it through a bound service so its errors reach the same project.
func DSN() string {
	if value := dsn.Load(); value != nil && !disabled.Load() {
		return *value
	}
	return ""
}

// Capture reports err, tagged with the operation and component that failed.
func Capture(err error, operation, component string) {
	if DSN() == "" || err == nil {
		return
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("operation", operation)
		scope.SetTag("component", component)
		sentry.CaptureException(err)
	})
}

// CapturePanic reports a value returned by recover.
func CapturePanic(recovered any, operation, component string) {
	if DSN() == "" || recovered == nil {
		return
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("operation", operation)
		scope.SetTag("component", component)
		sentry.CurrentHub().Recover(recovered)
	})
}

// Flush delivers pending events. Call it before the process exits.
func Flush() {
	if DSN() != "" {
		sentry.Flush(flushTimeout)
	}
}

// RecoverAndRepanic is deferred at the top of the program: it reports a panic
// and panics again so the process still crashes with its stack trace. Defer
// Flush before it so the report is delivered while the panic unwinds.
func RecoverAndRepanic() {
	if recovered := recover(); recovered != nil {
		CapturePanic(recovered, "main", "runtime")
		panic(recovered)
	}
}
