package mcp

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const endpointPath = "/mcp"

// Options controls the optional local MCP transport.
type Options struct {
	Enabled bool
	Port    int
}

// ConnectionDetails contains the credentials for one enabled launch.
type ConnectionDetails struct {
	Endpoint    string
	BearerToken string
}

// Runtime owns the optional MCP listener and HTTP server.
type Runtime struct {
	listener *readyListener
	server   *http.Server
	logger   *slog.Logger
	endpoint string
	token    string
	done     chan struct{}

	mu       sync.Mutex
	started  bool
	stopped  bool
	serveErr error

	shutdownOnce sync.Once
	shutdownErr  error
}

type readyListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

func (l *readyListener) Accept() (net.Conn, error) {
	l.once.Do(func() { close(l.ready) })
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accept MCP connection: %w", err)
	}
	return connection, nil
}

// NewRuntime binds the loopback listener without accepting requests.
func NewRuntime(
	ctx context.Context,
	name string,
	options Options,
	logger *slog.Logger,
	resolvers Resolvers,
) (*Runtime, error) {
	runtime := &Runtime{}
	if !options.Enabled {
		return runtime, nil
	}
	if ctx == nil {
		return nil, errors.New("create MCP runtime: context is required")
	}
	if name == "" {
		return nil, errors.New("create MCP runtime: application name is required")
	}
	if logger == nil {
		return nil, errors.New("create MCP runtime: logger is required")
	}
	if options.Port < 0 || options.Port > 65535 {
		return nil, fmt.Errorf("create MCP runtime: port %d is outside 0..65535", options.Port)
	}
	if err := resolvers.validate(); err != nil {
		return nil, fmt.Errorf("create MCP runtime: %w", err)
	}

	listener, err := (&net.ListenConfig{}).Listen(
		ctx,
		"tcp",
		net.JoinHostPort("127.0.0.1", strconv.Itoa(options.Port)),
	)
	if err != nil {
		return nil, fmt.Errorf("bind MCP loopback listener: %w", err)
	}
	sdkServer := sdk.NewServer(&sdk.Implementation{Name: name, Version: "1"}, &sdk.ServerOptions{Logger: logger})
	RegisterGeneratedTools(sdkServer, resolvers, logger)
	protection := http.NewCrossOriginProtection()
	handler := sdk.NewStreamableHTTPHandler(
		func(*http.Request) *sdk.Server { return sdkServer },
		&sdk.StreamableHTTPOptions{
			Stateless:                    true,
			Logger:                       logger,
			PropagateRequestCancellation: true,
		},
	)
	token := rand.Text()
	mux := http.NewServeMux()
	mux.Handle(endpointPath, requireBearerToken(token, protection.Handler(handler)))
	runtime.listener = &readyListener{Listener: listener, ready: make(chan struct{})}
	runtime.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	runtime.logger = logger
	runtime.endpoint = "http://" + listener.Addr().String() + endpointPath
	runtime.token = token
	runtime.done = make(chan struct{})
	return runtime, nil
}

// Start begins accepting requests when the application runtime is ready.
func (r *Runtime) Start() {
	r.mu.Lock()
	if r.listener == nil || r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	server := r.server
	listener := r.listener
	go func() {
		err := server.Serve(listener)
		r.mu.Lock()
		if errors.Is(err, http.ErrServerClosed) || r.stopped && errors.Is(err, net.ErrClosed) {
			err = nil
		}
		r.serveErr = err
		close(r.done)
		r.mu.Unlock()
		if err != nil {
			r.logger.Error("MCP server stopped unexpectedly",
				slog.String("operation", "mcp.serve"),
				slog.String("component", "mcp"),
				slog.Any("error", err),
			)
		}
	}()
	<-listener.ready
	r.mu.Unlock()
	r.logger.Info("MCP server ready",
		slog.String("operation", "mcp.start"),
		slog.String("component", "mcp"),
	)
}

// ConnectionDetails returns the per-launch local endpoint and bearer token.
func (r *Runtime) ConnectionDetails() (ConnectionDetails, error) {
	if r.listener == nil {
		return ConnectionDetails{}, errors.New("MCP is disabled")
	}
	return ConnectionDetails{Endpoint: r.endpoint, BearerToken: r.token}, nil
}

func requireBearerToken(token string, next http.Handler) http.Handler {
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		provided := []byte(request.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(provided, expected) != 1 {
			response.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(response, request)
	})
}

// Shutdown stops accepting MCP requests before shared services are disposed.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("shutdown MCP runtime: context is required")
	}
	r.shutdownOnce.Do(func() {
		r.mu.Lock()
		if r.listener == nil {
			r.stopped = true
			r.mu.Unlock()
			return
		}
		r.stopped = true
		started := r.started
		r.mu.Unlock()

		if !started {
			if err := r.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				r.shutdownErr = fmt.Errorf("close unstarted MCP listener: %w", err)
			}
			close(r.done)
			return
		}
		if err := r.server.Shutdown(ctx); err != nil {
			r.shutdownErr = fmt.Errorf("shutdown MCP HTTP server: %w", err)
			if closeErr := r.server.Close(); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				r.shutdownErr = errors.Join(r.shutdownErr, fmt.Errorf("force close MCP HTTP server: %w", closeErr))
			}
		}
		select {
		case <-r.done:
			r.mu.Lock()
			r.shutdownErr = errors.Join(r.shutdownErr, r.serveErr)
			r.mu.Unlock()
		case <-ctx.Done():
			r.shutdownErr = errors.Join(r.shutdownErr, fmt.Errorf("wait for MCP HTTP server: %w", ctx.Err()))
		}
	})
	return r.shutdownErr
}
