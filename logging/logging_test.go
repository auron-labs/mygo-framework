package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDevelopmentLoggerWritesSafeFieldsAndRedactsSensitiveValues(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, closer, err := newLogger(DefaultOptions(true, ""), &output)
	if err != nil {
		t.Fatalf("create development logger: %v", err)
	}
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Errorf("close development logger: %v", err)
		}
	})

	logger.Info("note created",
		slog.String("operation", "notes.create"),
		slog.Int64("note_id", 42),
		slog.String("token", "do-not-log"),
		slog.Group("settings", slog.String("path", "private")),
	)
	got := output.String()
	for _, want := range []string{"operation=notes.create", "note_id=42", "token=[REDACTED]", "path=[REDACTED]"} {
		if !strings.Contains(got, want) {
			t.Errorf("log output %q does not contain %q", got, want)
		}
	}
	for _, forbidden := range []string{"do-not-log", "private"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("log output contains sensitive value %q: %q", forbidden, got)
		}
	}
}

func TestPackagedLoggerWritesStderrAndRotatingFile(t *testing.T) {
	t.Parallel()

	directory := filepath.Join(t.TempDir(), "logs")
	var stderr bytes.Buffer
	logger, closer, err := newLogger(DefaultOptions(false, directory), &stderr)
	if err != nil {
		t.Fatalf("create packaged logger: %v", err)
	}
	logger.Info("runtime ready", slog.String("component", "runtime"))
	if err := closer.Close(); err != nil {
		t.Fatalf("close packaged logger: %v", err)
	}

	//nolint:gosec // The test reads only its own temporary directory.
	fileOutput, err := os.ReadFile(filepath.Join(directory, logFilename))
	if err != nil {
		t.Fatalf("read packaged log: %v", err)
	}
	for destination, output := range map[string]string{
		"stderr": stderr.String(),
		"file":   string(fileOutput),
	} {
		if !strings.Contains(output, "msg=\"runtime ready\"") || !strings.Contains(output, "component=runtime") {
			t.Errorf("%s output = %q, want runtime fields", destination, output)
		}
	}
}

func TestReportUnexpectedLogsOnceAndIgnoresCancellation(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger, closer, err := newLogger(DefaultOptions(true, ""), &output)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	t.Cleanup(func() {
		if err := closer.Close(); err != nil {
			t.Errorf("close logger: %v", err)
		}
	})

	ReportUnexpected(t.Context(), logger, "notes.create", "desktop", nil)
	ReportUnexpected(t.Context(), logger, "notes.create", "desktop", context.Canceled)
	ReportUnexpected(
		t.Context(),
		logger,
		"notes.create",
		"desktop",
		errors.New("database unavailable"),
		slog.Duration("duration", 12*time.Millisecond),
		slog.Int64("note_id", 42),
	)

	got := output.String()
	if count := strings.Count(got, "msg=\"operation failed\""); count != 1 {
		t.Fatalf("unexpected-failure records = %d, want 1: %q", count, got)
	}
	for _, want := range []string{
		"operation=notes.create",
		"component=desktop",
		"duration=12ms",
		"note_id=42",
		"error=\"database unavailable\"",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("log output %q does not contain %q", got, want)
		}
	}
}

func TestPackagedLoggerRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	tests := map[string]Options{
		"missing directory": DefaultOptions(false, ""),
		"invalid max size": {
			Directory: t.TempDir(),
			MaxSizeMB: -1,
		},
	}
	for name, options := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := newLogger(options, &bytes.Buffer{}); err == nil {
				t.Fatal("create packaged logger error = nil, want validation error")
			}
		})
	}
}
