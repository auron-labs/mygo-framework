// Package bootstrap owns application provider registration and composition.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mygo-framework/app/services"
	"mygo-framework/database/migrations"
	"mygo-framework/internal/database"
	"mygo-framework/logging"
	appRuntime "mygo-framework/runtime"
	"mygo-framework/validation"
	"strings"
	"sync"
	"time"

	"github.com/samber/do/v2"
)

var errStartupSkipped = errors.New("application runtime was stopped before startup")

// Options contains immutable values resolved at the desktop or CLI edge.
type Options struct {
	Development  bool
	DatabasePath string
	LogDirectory string
}

// Bootstrap owns the application's dependency graph and runtime resources.
type Bootstrap struct {
	injector   *do.RootScope
	readiness  *appRuntime.Readiness
	migrations fs.FS

	startOnce sync.Once
	startErr  error
	options   Options

	resourceShutdownOnce sync.Once
	resourceShutdownErr  error
	shutdownOnce         sync.Once
	shutdownErr          error
}

// New registers the application's explicit providers without resolving them.
func New() *Bootstrap {
	bootstrap := &Bootstrap{
		injector:   do.New(),
		readiness:  appRuntime.NewReadiness(),
		migrations: migrations.FS(),
	}
	do.Provide(bootstrap.injector, bootstrap.provideReadiness)
	do.Provide(bootstrap.injector, provideValidator)
	do.Provide(bootstrap.injector, bootstrap.provideLogOutput)
	do.Provide(bootstrap.injector, provideLogger)
	do.Provide(bootstrap.injector, bootstrap.provideDatabaseResource)
	do.Provide(bootstrap.injector, provideDatabase)
	do.Provide(bootstrap.injector, provideNotes)
	do.Provide(bootstrap.injector, provideLifetimeWorker)

	return bootstrap
}

// Readiness returns the gate shared by every application entry point.
func (b *Bootstrap) Readiness() *appRuntime.Readiness {
	return b.readiness
}

// ResolveNotes returns the dependencies used by the desktop notes adapter.
// Callers wait for Readiness before invoking it.
func (b *Bootstrap) ResolveNotes() (*services.Notes, *slog.Logger, error) {
	logger, err := do.Invoke[*slog.Logger](b.injector)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve notes adapter logger: %w", err)
	}
	service, err := do.Invoke[*services.Notes](b.injector)
	if err != nil {
		return nil, logger, fmt.Errorf("resolve notes adapter service: %w", err)
	}

	return service, logger, nil
}

// Start opens runtime resources, applies migrations, and publishes readiness.
func (b *Bootstrap) Start(ctx context.Context, options Options) error {
	b.startOnce.Do(func() {
		if ctx == nil {
			b.startErr = errors.New("start application runtime: context is required")
			_ = b.readiness.Fail(b.startErr)
			return
		}

		b.startErr = b.start(ctx, options)
	})

	return b.startErr
}

func (b *Bootstrap) start(ctx context.Context, options Options) error {
	b.options = options
	do.ProvideValue[context.Context](b.injector, ctx)
	startedAt := time.Now()
	if err := b.resolveRequired(); err != nil {
		return b.failStartup(ctx, startedAt, err)
	}
	if err := ctx.Err(); err != nil {
		return b.failStartup(ctx, startedAt, fmt.Errorf("finish application startup: %w", err))
	}
	if err := b.readiness.Succeed(); err != nil {
		return b.failStartup(ctx, startedAt, fmt.Errorf("publish application readiness: %w", err))
	}

	logger, err := do.Invoke[*slog.Logger](b.injector)
	if err != nil {
		return fmt.Errorf("resolve logger after startup: %w", err)
	}
	logger.InfoContext(ctx, "runtime ready",
		slog.String("operation", "runtime.start"),
		slog.String("component", "runtime"),
		slog.Duration("duration", time.Since(startedAt)),
	)

	return nil
}

func (b *Bootstrap) failStartup(ctx context.Context, startedAt time.Time, cause error) error {
	startupErr := fmt.Errorf("start application runtime: %w", cause)
	if logger, err := do.Invoke[*slog.Logger](b.injector); err == nil {
		loggedErr := startupErr
		if b.options.DatabasePath != "" {
			loggedErr = errors.New(strings.ReplaceAll(
				startupErr.Error(),
				b.options.DatabasePath,
				"[database path]",
			))
		}
		logging.ReportUnexpected(ctx, logger, "runtime.start", "runtime", loggedErr,
			slog.Duration("duration", time.Since(startedAt)),
		)
	}
	if err := b.readiness.Fail(startupErr); err != nil {
		startupErr = errors.Join(startupErr, fmt.Errorf("publish startup failure: %w", err))
	}
	if err := b.shutdownResources(context.WithoutCancel(ctx)); err != nil {
		startupErr = errors.Join(startupErr, fmt.Errorf("rollback failed startup: %w", err))
	}

	return startupErr
}

func (b *Bootstrap) resolveRequired() error {
	if _, err := do.Invoke[*appRuntime.Readiness](b.injector); err != nil {
		return fmt.Errorf("resolve runtime readiness: %w", err)
	}
	if _, err := do.Invoke[*validation.Validator](b.injector); err != nil {
		return fmt.Errorf("resolve application validator: %w", err)
	}
	if _, err := do.Invoke[*slog.Logger](b.injector); err != nil {
		return fmt.Errorf("resolve application logger: %w", err)
	}
	if _, err := do.Invoke[*database.Database](b.injector); err != nil {
		return fmt.Errorf("resolve application database: %w", err)
	}
	if _, err := do.Invoke[*services.Notes](b.injector); err != nil {
		return fmt.Errorf("resolve notes service: %w", err)
	}
	if _, err := do.Invoke[*appRuntime.LifetimeWorker](b.injector); err != nil {
		return fmt.Errorf("resolve app-lifetime worker: %w", err)
	}

	return nil
}

// Shutdown rejects new work and disposes the resolved graph once.
func (b *Bootstrap) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("shutdown application runtime: context is required")
	}

	b.startOnce.Do(func() {
		b.startErr = errStartupSkipped
	})
	b.shutdownOnce.Do(func() {
		b.readiness.BeginShutdown()
		b.shutdownErr = b.shutdownResources(ctx)
		if err := b.readiness.FinishShutdown(); err != nil {
			b.shutdownErr = errors.Join(b.shutdownErr, err)
		}
	})

	return b.shutdownErr
}

func (b *Bootstrap) shutdownResources(ctx context.Context) error {
	b.resourceShutdownOnce.Do(func() {
		report := b.injector.ShutdownWithContext(ctx)
		if !report.Succeed {
			b.resourceShutdownErr = fmt.Errorf("shutdown dependency graph: %w", *report)
		}
	})

	return b.resourceShutdownErr
}

func (b *Bootstrap) provideReadiness(_ do.Injector) (*appRuntime.Readiness, error) {
	return b.readiness, nil
}

func provideValidator(_ do.Injector) (*validation.Validator, error) {
	return validation.New(), nil
}

type logOutput struct {
	logger *slog.Logger
	closer io.Closer
}

func (b *Bootstrap) provideLogOutput(_ do.Injector) (*logOutput, error) {
	logger, closer, err := logging.New(logging.DefaultOptions(
		b.options.Development,
		b.options.LogDirectory,
	))
	if err != nil {
		return nil, fmt.Errorf("open application logs: %w", err)
	}

	return &logOutput{logger: logger, closer: closer}, nil
}

func (o *logOutput) Shutdown() error {
	if err := o.closer.Close(); err != nil {
		return fmt.Errorf("close log output: %w", err)
	}

	return nil
}

func provideLogger(injector do.Injector) (*slog.Logger, error) {
	output, err := do.Invoke[*logOutput](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve log output: %w", err)
	}

	return output.logger, nil
}

type databaseResource struct {
	database *database.Database
	logger   *slog.Logger
}

func (b *Bootstrap) provideDatabaseResource(injector do.Injector) (*databaseResource, error) {
	logger, err := do.Invoke[*slog.Logger](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve logger for database: %w", err)
	}
	ctx, err := do.Invoke[context.Context](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve startup context for database: %w", err)
	}

	startedAt := time.Now()
	database, err := database.Open(ctx, b.options.DatabasePath, b.migrations)
	if err != nil {
		return nil, fmt.Errorf("open runtime database: %w", err)
	}
	database.GORM.Logger = logging.NewGORM(logger)
	logger.InfoContext(ctx, "database ready",
		slog.String("operation", "database.open"),
		slog.String("component", "database"),
		slog.Duration("duration", time.Since(startedAt)),
	)

	return &databaseResource{database: database, logger: logger}, nil
}

func (r *databaseResource) Shutdown() error {
	if err := r.database.Close(); err != nil {
		r.logger.Error("database shutdown failed",
			slog.String("operation", "database.close"),
			slog.String("component", "database"),
			slog.Any("error", err),
		)

		return fmt.Errorf("close runtime database: %w", err)
	}

	return nil
}

func provideDatabase(injector do.Injector) (*database.Database, error) {
	resource, err := do.Invoke[*databaseResource](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve database resource: %w", err)
	}

	return resource.database, nil
}

func provideNotes(injector do.Injector) (*services.Notes, error) {
	database, err := do.Invoke[*database.Database](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve database for notes service: %w", err)
	}
	validator, err := do.Invoke[*validation.Validator](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve validator for notes service: %w", err)
	}

	return services.NewNotes(database.GORM, validator), nil
}

func provideLifetimeWorker(injector do.Injector) (*appRuntime.LifetimeWorker, error) {
	// Resolve the database to ensure the worker is disposed before its resources.
	if _, err := do.Invoke[*database.Database](injector); err != nil {
		return nil, fmt.Errorf("resolve database for app-lifetime worker: %w", err)
	}
	ctx, err := do.Invoke[context.Context](injector)
	if err != nil {
		return nil, fmt.Errorf("resolve context for app-lifetime worker: %w", err)
	}

	return appRuntime.NewLifetimeWorker(ctx), nil
}
