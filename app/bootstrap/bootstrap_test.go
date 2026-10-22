package bootstrap

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/samber/do/v2"
)

func TestResolveRequiredEagerlyConstructsGraph(t *testing.T) {
	t.Parallel()

	bootstrap := New()
	if got := len(bootstrap.injector.ListInvokedServices()); got != 0 {
		t.Fatalf("invoked services before resolve = %d, want 0", got)
	}
	if err := bootstrap.ResolveRequired(); err != nil {
		t.Fatalf("resolve required graph: %v", err)
	}
	if got := len(bootstrap.injector.ListInvokedServices()); got != 2 {
		t.Fatalf("invoked services after resolve = %d, want 2", got)
	}
}

func TestResolveRequiredReportsMissingRegistration(t *testing.T) {
	t.Parallel()

	bootstrap := &Bootstrap{injector: do.New()}
	err := bootstrap.ResolveRequired()
	if !errors.Is(err, do.ErrServiceNotFound) {
		t.Fatalf("resolve error = %v, want ErrServiceNotFound", err)
	}
}

func TestShutdownUsesDependencyOrder(t *testing.T) {
	t.Parallel()

	bootstrap := &Bootstrap{injector: do.New()}
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
