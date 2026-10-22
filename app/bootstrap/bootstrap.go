// Package bootstrap owns application provider registration and composition.
package bootstrap

import (
	"context"
	"fmt"
	"mygo-framework/runtime"
	"mygo-framework/validation"

	"github.com/samber/do/v2"
)

// Bootstrap owns the application's dependency graph.
type Bootstrap struct {
	injector *do.RootScope
}

// New registers the application's explicit providers without resolving them.
func New() *Bootstrap {
	injector := do.New()
	do.Provide(injector, provideReadiness)
	do.Provide(injector, provideValidator)

	return &Bootstrap{injector: injector}
}

// ResolveRequired constructs every dependency required for application startup.
func (b *Bootstrap) ResolveRequired() error {
	if _, err := do.Invoke[*runtime.Readiness](b.injector); err != nil {
		return fmt.Errorf("resolve runtime readiness: %w", err)
	}
	if _, err := do.Invoke[*validation.Validator](b.injector); err != nil {
		return fmt.Errorf("resolve application validator: %w", err)
	}

	return nil
}

// Shutdown disposes the resolved graph in dependency order.
func (b *Bootstrap) Shutdown(ctx context.Context) error {
	report := b.injector.ShutdownWithContext(ctx)
	if !report.Succeed {
		return fmt.Errorf("shutdown dependency graph: %w", *report)
	}

	return nil
}

func provideReadiness(_ do.Injector) (*runtime.Readiness, error) {
	return runtime.NewReadiness(), nil
}

func provideValidator(_ do.Injector) (*validation.Validator, error) {
	return validation.New(), nil
}
