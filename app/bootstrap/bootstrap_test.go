package bootstrap

import (
	"context"
	"errors"
	"mygo-framework/app/models"
	"mygo-framework/internal/database"
	"mygo-framework/problem"
	appRuntime "mygo-framework/runtime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/samber/do/v2"
)

func TestStartEagerlyConstructsRequiredGraph(t *testing.T) {
	t.Parallel()

	bootstrap := New()
	if got := len(bootstrap.injector.ListInvokedServices()); got != 0 {
		t.Fatalf("invoked services before start = %d, want 0", got)
	}
	if err := bootstrap.Start(t.Context(), Options{
		Development:  true,
		DatabasePath: filepath.Join(t.TempDir(), "notes.db"),
	}); err != nil {
		t.Fatalf("start required graph: %v", err)
	}
	t.Cleanup(func() {
		if err := bootstrap.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown required graph: %v", err)
		}
	})

	if got := len(bootstrap.injector.ListInvokedServices()); got != 7 {
		t.Fatalf("invoked services after start = %d, want 7", got)
	}
	if got := bootstrap.Readiness().State(); got != appRuntime.StateReady {
		t.Fatalf("readiness state = %d, want ready", got)
	}
}

func TestResolveRequiredReportsMissingRegistration(t *testing.T) {
	t.Parallel()

	bootstrap := &Bootstrap{injector: do.New()}
	err := bootstrap.resolveRequired()
	if !errors.Is(err, do.ErrServiceNotFound) {
		t.Fatalf("resolve error = %v, want ErrServiceNotFound", err)
	}
}

func TestRuntimeMigrationPersistsAndDatabaseDisposesOnce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "notes.db")
	first := New()
	if err := first.Start(t.Context(), Options{Development: true, DatabasePath: path}); err != nil {
		t.Fatalf("start first runtime: %v", err)
	}
	firstDatabase, err := do.Invoke[*database.Database](first.injector)
	if err != nil {
		t.Fatalf("resolve first database: %v", err)
	}
	want := models.Note{Body: "persists through runtime restart"}
	if err := firstDatabase.GORM.WithContext(t.Context()).Create(&want).Error; err != nil {
		t.Fatalf("create note: %v", err)
	}
	if err := first.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown first runtime: %v", err)
	}
	if err := first.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown first runtime again: %v", err)
	}
	if err := firstDatabase.SQL.PingContext(t.Context()); err == nil {
		t.Fatal("runtime database remains open after shutdown")
	}

	second := New()
	if err := second.Start(t.Context(), Options{Development: true, DatabasePath: path}); err != nil {
		t.Fatalf("start second runtime: %v", err)
	}
	t.Cleanup(func() {
		if err := second.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown second runtime: %v", err)
		}
	})
	secondDatabase, err := do.Invoke[*database.Database](second.injector)
	if err != nil {
		t.Fatalf("resolve second database: %v", err)
	}
	var got models.Note
	if err := secondDatabase.GORM.WithContext(t.Context()).First(&got, want.ID).Error; err != nil {
		t.Fatalf("reload note: %v", err)
	}
	if got.Body != want.Body {
		t.Fatalf("reloaded body = %q, want %q", got.Body, want.Body)
	}
}

func TestMigrationFailureIsReportedAndReleasesDatabase(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	databasePath := filepath.Join(directory, "notes.db")
	logDirectory := filepath.Join(directory, "logs")
	bootstrap := New()
	bootstrap.migrations = fstest.MapFS{
		"00001_broken.sql": {Data: []byte("-- +goose Up\nCREATE TABLE notes (id INTEGER);\nINVALID MIGRATION;\n")},
	}

	startErr := bootstrap.Start(t.Context(), Options{
		DatabasePath: databasePath,
		LogDirectory: logDirectory,
	})
	if startErr == nil {
		t.Fatal("start error = nil, want migration failure")
	}
	if !strings.Contains(startErr.Error(), "00001_broken.sql") {
		t.Fatalf("start error does not identify migration: %v", startErr)
	}
	if got := bootstrap.Readiness().State(); got != appRuntime.StateFailed {
		t.Fatalf("readiness state = %d, want failed", got)
	}
	waitErr := bootstrap.Readiness().Wait(t.Context())
	var unavailable *problem.Problem
	if !errors.As(waitErr, &unavailable) || unavailable.Code != problem.CodeUnavailable {
		t.Fatalf("readiness error = %v, want unavailable problem", waitErr)
	}
	if !errors.Is(waitErr, startErr) {
		t.Fatalf("readiness error does not preserve startup failure: %v", waitErr)
	}

	//nolint:gosec // The test reads only its own temporary log directory.
	logOutput, err := os.ReadFile(filepath.Join(logDirectory, "app.log"))
	if err != nil {
		t.Fatalf("read startup failure log: %v", err)
	}
	for _, want := range []string{"operation=runtime.start", "component=runtime", "00001_broken.sql"} {
		if !strings.Contains(string(logOutput), want) {
			t.Errorf("startup log does not contain %q: %s", want, logOutput)
		}
	}
	if strings.Contains(string(logOutput), databasePath) {
		t.Errorf("startup log contains database path: %s", logOutput)
	}
	if err := os.Remove(databasePath); err != nil {
		t.Fatalf("remove database after failed startup: %v", err)
	}
}

func TestFailedStartRollsBackResolvedResourcesOnce(t *testing.T) {
	t.Parallel()

	bootstrap := New()
	probe := &rollbackProbe{}
	do.Provide(bootstrap.injector, func(do.Injector) (*rollbackProbe, error) {
		return probe, nil
	})
	forcedErr := errors.New("forced database startup failure")
	do.Override(bootstrap.injector, func(injector do.Injector) (*databaseResource, error) {
		if _, err := do.Invoke[*rollbackProbe](injector); err != nil {
			return nil, err
		}

		return nil, forcedErr
	})

	err := bootstrap.Start(t.Context(), Options{Development: true, DatabasePath: "unused"})
	if !errors.Is(err, forcedErr) {
		t.Fatalf("start error = %v, want forced failure", err)
	}
	if got := probe.shutdowns.Load(); got != 1 {
		t.Fatalf("rollback shutdowns = %d, want 1", got)
	}
	if err := bootstrap.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown failed runtime: %v", err)
	}
	if got := probe.shutdowns.Load(); got != 1 {
		t.Fatalf("shutdowns after explicit shutdown = %d, want 1", got)
	}
}

func TestShutdownUsesDependencyOrder(t *testing.T) {
	t.Parallel()

	bootstrap := &Bootstrap{
		injector:  do.New(),
		readiness: appRuntime.NewReadiness(),
	}
	order := &shutdownOrder{}
	do.ProvideValue(bootstrap.injector, order)
	do.Provide(bootstrap.injector, func(injector do.Injector) (*testDependency, error) {
		log, err := do.Invoke[*shutdownOrder](injector)
		if err != nil {
			return nil, err
		}

		return &testDependency{order: log}, nil
	})
	do.Provide(bootstrap.injector, func(injector do.Injector) (*testDependent, error) {
		dependency, err := do.Invoke[*testDependency](injector)
		if err != nil {
			return nil, err
		}

		return &testDependent{dependency: dependency}, nil
	})
	if _, err := do.Invoke[*testDependent](bootstrap.injector); err != nil {
		t.Fatalf("resolve dependent: %v", err)
	}

	if err := bootstrap.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown graph: %v", err)
	}
	if got := order.values(); len(got) != 2 || got[0] != "dependent" || got[1] != "dependency" {
		t.Fatalf("shutdown order = %#v", got)
	}
}

type rollbackProbe struct {
	shutdowns atomic.Int32
}

func (p *rollbackProbe) Shutdown() error {
	p.shutdowns.Add(1)

	return nil
}

type shutdownOrder struct {
	mu    sync.Mutex
	order []string
}

func (o *shutdownOrder) append(name string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.order = append(o.order, name)
}

func (o *shutdownOrder) values() []string {
	o.mu.Lock()
	defer o.mu.Unlock()

	return append([]string(nil), o.order...)
}

type testDependency struct {
	order *shutdownOrder
}

func (d *testDependency) Shutdown(context.Context) error {
	d.order.append("dependency")

	return nil
}

type testDependent struct {
	dependency *testDependency
}

func (d *testDependent) Shutdown(context.Context) error {
	d.dependency.order.append("dependent")

	return nil
}
