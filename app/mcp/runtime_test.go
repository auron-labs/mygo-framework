package mcp

import (
	"bytes"
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
	if _, err := runtime.ConnectionDetails(); err == nil {
		t.Fatal("disabled runtime returned connection details")
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
	details, err := runtime.ConnectionDetails()
	if err != nil {
		t.Fatalf("read MCP connection details: %v", err)
	}
	endpoint, err := url.Parse(details.Endpoint)
	if err != nil {
		t.Fatalf("parse MCP endpoint: %v", err)
	}
	host, _, err := net.SplitHostPort(endpoint.Host)
	if err != nil || host != "127.0.0.1" || endpoint.Path != endpointPath {
		t.Fatalf("MCP endpoint = %q, split error = %v", details.Endpoint, err)
	}

	runtime.Start()
	client := &http.Client{Timeout: 2 * time.Second}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, details.Endpoint, nil)
	if err != nil {
		t.Fatalf("create stateless MCP request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+details.BearerToken)
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

func TestRuntimeStartReturnsWhenServerAlreadyClosed(t *testing.T) {
	t.Parallel()

	runtime, err := NewRuntime(
		t.Context(),
		"closed-server-test",
		Options{Enabled: true, Port: 0},
		slog.New(slog.DiscardHandler),
		Resolvers{Notes: func() (*services.Notes, error) { return &services.Notes{}, nil }},
	)
	if err != nil {
		t.Fatalf("create MCP runtime: %v", err)
	}
	// Serve returns ErrServerClosed before the first Accept when the server is
	// already closed; Start must still unblock and report a clean stop.
	if err := runtime.server.Close(); err != nil {
		t.Fatalf("pre-close MCP server: %v", err)
	}
	runtime.Start()
	select {
	case <-runtime.done:
	case <-time.After(2 * time.Second):
		t.Fatal("serve goroutine did not finish after starting a closed server")
	}
	runtime.mu.Lock()
	serveErr := runtime.serveErr
	runtime.mu.Unlock()
	if serveErr != nil {
		t.Fatalf("closed-server serve error = %v, want nil", serveErr)
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown closed-server runtime: %v", err)
	}
}

func TestRuntimeProtectsConnectionDetailsAndHTTPBoundary(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	resolvers := Resolvers{Notes: func() (*services.Notes, error) { return &services.Notes{}, nil }}
	runtime, err := NewRuntime(t.Context(), "security-test", Options{Enabled: true}, logger, resolvers)
	if err != nil {
		t.Fatalf("create protected MCP runtime: %v", err)
	}
	t.Cleanup(func() {
		if err := runtime.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown protected MCP runtime: %v", err)
		}
	})
	details, err := runtime.ConnectionDetails()
	if err != nil {
		t.Fatalf("read protected connection details: %v", err)
	}
	second, err := NewRuntime(t.Context(), "security-test", Options{Enabled: true}, logger, resolvers)
	if err != nil {
		t.Fatalf("create second protected MCP runtime: %v", err)
	}
	t.Cleanup(func() {
		if err := second.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown second protected MCP runtime: %v", err)
		}
	})
	secondDetails, err := second.ConnectionDetails()
	if err != nil {
		t.Fatalf("read second connection details: %v", err)
	}
	if details.BearerToken == "" || details.BearerToken == secondDetails.BearerToken {
		t.Fatal("MCP bearer token is empty or reused across launches")
	}

	runtime.Start()
	client := &http.Client{Timeout: 2 * time.Second}
	tests := []struct {
		name   string
		token  string
		host   string
		origin string
		want   int
	}{
		{name: "missing token", want: http.StatusUnauthorized},
		{name: "wrong token", token: "wrong", want: http.StatusUnauthorized},
		{name: "invalid host", token: details.BearerToken, host: "evil.example", want: http.StatusForbidden},
		{name: "cross origin", token: details.BearerToken, origin: "https://evil.example", want: http.StatusForbidden},
		{name: "authorized", token: details.BearerToken, want: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := initializeStatus(t, client, details.Endpoint, test.token, test.host, test.origin)
			if status != test.want {
				t.Fatalf("initialize status = %d, want %d", status, test.want)
			}
		})
	}
	if err := runtime.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown protected MCP runtime: %v", err)
	}
	if strings.Contains(logs.String(), details.BearerToken) {
		t.Fatal("normal MCP logs contain the bearer token")
	}
}

func initializeStatus(t *testing.T, client *http.Client, endpoint, token, host, origin string) int {
	t.Helper()
	const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint, strings.NewReader(initialize))
	if err != nil {
		t.Fatalf("create MCP initialize request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	request.Host = host
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("send MCP initialize request: %v", err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close MCP initialize response: %v", err)
	}
	return response.StatusCode
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
