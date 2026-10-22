package mcp

import (
	"context"
	"log/slog"
	"mygo-framework/app/services"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRuntimeIsDisabledWithoutOpeningAListener(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(context.TODO(), "", Options{}, nil, Resolvers{})
	if err != nil {
		t.Fatalf("create disabled runtime: %v", err)
	}
	runtime.Start()
	if runtime.Endpoint() != "" {
		t.Fatalf("disabled endpoint = %q", runtime.Endpoint())
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown disabled runtime: %v", err)
	}
}

func TestRuntimeServesStatelessHTTPOnLoopbackAndShutsDown(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	runtime, err := NewRuntime(
		t.Context(),
		"runtime-test",
		Options{Enabled: true, Port: 0},
		logger,
		Resolvers{Notes: func() (*services.Notes, error) { return &services.Notes{}, nil }},
	)
	if err != nil {
		t.Fatalf("create MCP runtime: %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup MCP runtime: %v", err)
		}
	})
	endpoint, err := url.Parse(runtime.Endpoint())
	if err != nil {
		t.Fatalf("parse MCP endpoint: %v", err)
	}
	host, _, err := net.SplitHostPort(endpoint.Host)
	if err != nil || host != "127.0.0.1" || endpoint.Path != endpointPath {
		t.Fatalf("MCP endpoint = %q, split error = %v", runtime.Endpoint(), err)
	}

	runtime.Start()
	client := &http.Client{Timeout: 2 * time.Second}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, runtime.Endpoint(), nil)
	if err != nil {
		t.Fatalf("create stateless MCP request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("GET stateless MCP endpoint: %v", err)
	}
	if closeErr := response.Body.Close(); closeErr != nil {
		t.Fatalf("close MCP response: %v", closeErr)
	}
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", response.StatusCode, http.StatusMethodNotAllowed)
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown MCP runtime: %v", err)
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown MCP runtime again: %v", err)
	}
	connection, err := (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(t.Context(), "tcp", endpoint.Host)
	if err == nil {
		_ = connection.Close()
		t.Fatal("MCP listener still accepts connections after shutdown")
	}
}

func TestRuntimeValidatesEnabledConfiguration(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	validResolvers := Resolvers{Notes: func() (*services.Notes, error) { return &services.Notes{}, nil }}
	tests := []struct {
		name      string
		appName   string
		options   Options
		logger    *slog.Logger
		resolvers Resolvers
		want      string
	}{
		{name: "name", options: Options{Enabled: true}, logger: logger, resolvers: validResolvers, want: "application name is required"},
		{name: "logger", appName: "test", options: Options{Enabled: true}, resolvers: validResolvers, want: "logger is required"},
		{name: "port", appName: "test", options: Options{Enabled: true, Port: 65536}, logger: logger, resolvers: validResolvers, want: "outside 0..65535"},
		{name: "resolver", appName: "test", options: Options{Enabled: true}, logger: logger, want: "resolver Notes is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewRuntime(t.Context(), test.appName, test.options, test.logger, test.resolvers)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}
