package desktop

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"mygo-framework/app/bootstrap"
	"mygo-framework/app/models"
	"mygo-framework/app/requests"
	"mygo-framework/app/services"
	"mygo-framework/database/migrations"
	"mygo-framework/internal/database"
	"mygo-framework/problem"
	appRuntime "mygo-framework/runtime"
	"mygo-framework/validation"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNotesWaitsForReadinessBeforeResolvingServices(t *testing.T) {
	t.Parallel()

	readiness := appRuntime.NewReadiness()
	var resolves atomic.Int32
	adapter := NewNotes(readiness, func() (*services.Notes, *slog.Logger, error) {
		resolves.Add(1)

		return nil, nil, errors.New("resolver should not run")
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	response := adapter.Create(ctx, requests.CreateNoteInput{Body: "waiting"})
	if response.Data != nil || response.Problem == nil || response.Problem.Code != problem.CodeCanceled {
		t.Fatalf("create response = %#v, want canceled problem", response)
	}
	if got := resolves.Load(); got != 0 {
		t.Fatalf("dependency resolutions = %d, want 0", got)
	}
}

func TestNotesMapsStartupFailureToUnavailable(t *testing.T) {
	t.Parallel()

	readiness := appRuntime.NewReadiness()
	if err := readiness.Fail(errors.New("startup failed")); err != nil {
		t.Fatalf("publish startup failure: %v", err)
	}
	adapter := NewNotes(readiness, func() (*services.Notes, *slog.Logger, error) {
		t.Fatal("resolver ran after startup failure")

		return nil, nil, nil
	})

	response := adapter.List(t.Context(), requests.ListNotesInput{})
	if response.Data != nil || response.Problem == nil || response.Problem.Code != problem.CodeUnavailable {
		t.Fatalf("list response = %#v, want unavailable problem", response)
	}
}

func TestNotesMapsConcreteCreateAndListResponses(t *testing.T) {
	t.Parallel()

	applicationRuntime := bootstrap.New()
	if err := applicationRuntime.Start(t.Context(), bootstrap.Options{
		Development:  true,
		DatabasePath: filepath.Join(t.TempDir(), "notes.db"),
	}); err != nil {
		t.Fatalf("start application runtime: %v", err)
	}
	t.Cleanup(func() {
		if err := applicationRuntime.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown application runtime: %v", err)
		}
	})
	adapter := NewNotes(applicationRuntime.Readiness(), applicationRuntime.ResolveNotes)

	invalid := adapter.Create(t.Context(), requests.CreateNoteInput{Body: "  "})
	if invalid.Data != nil || invalid.Problem == nil || invalid.Problem.Code != problem.CodeValidation {
		t.Fatalf("invalid create response = %#v", invalid)
	}
	created := adapter.Create(t.Context(), requests.CreateNoteInput{Body: "  Buy milk  "})
	if created.Data == nil || created.Problem != nil || created.Data.Note.Body != "Buy milk" {
		t.Fatalf("create response = %#v", created)
	}
	listed := adapter.List(t.Context(), requests.ListNotesInput{})
	if listed.Data == nil || listed.Problem != nil || len(listed.Data.Notes) != 1 {
		t.Fatalf("list response = %#v", listed)
	}
	if listed.Data.Notes[0].ID != created.Data.Note.ID {
		t.Fatalf("listed note = %#v, want created note %#v", listed.Data.Notes[0], created.Data.Note)
	}
}

func TestNotesLogsUnexpectedFailureOnce(t *testing.T) {
	t.Parallel()

	openedDatabase, err := database.Open(
		t.Context(),
		filepath.Join(t.TempDir(), "notes.db"),
		migrations.FS(),
	)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	service := services.NewNotes(openedDatabase.GORM, validation.New(), func(context.Context, models.Note) {})
	if err := openedDatabase.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	readiness := appRuntime.NewReadiness()
	if err := readiness.Succeed(); err != nil {
		t.Fatalf("publish readiness: %v", err)
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	adapter := NewNotes(readiness, func() (*services.Notes, *slog.Logger, error) {
		return service, logger, nil
	})

	response := adapter.List(t.Context(), requests.ListNotesInput{})
	if response.Data != nil || response.Problem == nil || response.Problem.Code != problem.CodeInternal {
		t.Fatalf("list response = %#v, want internal problem", response)
	}
	got := output.String()
	if count := strings.Count(got, "msg=\"operation failed\""); count != 1 {
		t.Fatalf("unexpected failure records = %d, want 1: %q", count, got)
	}
	for _, want := range []string{"operation=notes.list", "component=desktop", "duration="} {
		if !strings.Contains(got, want) {
			t.Errorf("failure log %q does not contain %q", got, want)
		}
	}
}

func TestNotesExportsOnlyFeatureMethods(t *testing.T) {
	t.Parallel()

	typeOfAdapter := reflect.TypeFor[*Notes]()
	if typeOfAdapter.NumMethod() != 2 {
		t.Fatalf("exported adapter methods = %d, want 2", typeOfAdapter.NumMethod())
	}
	for index, want := range []string{"Create", "List"} {
		if got := typeOfAdapter.Method(index).Name; got != want {
			t.Fatalf("exported method %d = %q, want %q", index, got, want)
		}
	}
}
