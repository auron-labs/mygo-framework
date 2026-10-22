package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	appDesktop "mygo-framework/app/desktop"
	appMCP "mygo-framework/app/mcp"
	"mygo-framework/app/models"
	"mygo-framework/app/requests"
	"mygo-framework/app/services"
	"mygo-framework/internal/database"
	"mygo-framework/problem"
	"mygo-framework/validation"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/do/v2"
)

func TestMCPAcceptance(t *testing.T) {
	application := New()
	changed := make(chan models.Note, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseChange := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseChange)
	do.Override(application.injector, func(injector do.Injector) (*services.Notes, error) {
		openedDatabase, err := do.Invoke[*database.Database](injector)
		if err != nil {
			return nil, err
		}
		validator, err := do.Invoke[*validation.Validator](injector)
		if err != nil {
			return nil, err
		}
		return services.NewNotes(openedDatabase.GORM, validator, func(_ context.Context, note models.Note) {
			changed <- note
			<-release
		}), nil
	})
	if err := application.Start(t.Context(), Options{
		Development:  true,
		DatabasePath: filepath.Join(t.TempDir(), "notes.db"),
		MCP:          appMCP.Options{Enabled: true, Port: 0},
	}); err != nil {
		t.Fatalf("start MCP acceptance runtime: %v", err)
	}
	t.Cleanup(func() {
		releaseChange()
		if err := application.Shutdown(context.Background()); err != nil {
			t.Errorf("cleanup MCP acceptance runtime: %v", err)
		}
	})

	service, err := do.Invoke[*services.Notes](application.injector)
	if err != nil {
		t.Fatalf("resolve application notes service: %v", err)
	}
	resolvers, err := application.mcpResolvers(application.injector)
	if err != nil {
		t.Fatalf("resolve MCP services: %v", err)
	}
	mcpService, err := resolvers.Notes()
	if err != nil {
		t.Fatalf("resolve MCP notes service: %v", err)
	}
	if mcpService != service {
		t.Fatal("MCP resolver constructed a second notes service")
	}
	openedDatabase, err := do.Invoke[*database.Database](application.injector)
	if err != nil {
		t.Fatalf("resolve application database: %v", err)
	}
	details, err := application.MCPConnectionDetails(t.Context())
	if err != nil {
		t.Fatalf("read MCP connection details: %v", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	t.Cleanup(transport.CloseIdleConnections)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: bearerTransport{
			token: details.BearerToken,
			base:  transport,
		},
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "acceptance", Version: "test"}, nil)
	session, err := client.Connect(t.Context(), &sdk.StreamableClientTransport{
		Endpoint:             details.Endpoint,
		HTTPClient:           httpClient,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect real MCP HTTP client: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	listedTools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("list MCP tools: %v", err)
	}
	if len(listedTools.Tools) != 2 || listedTools.Tools[0].Name != "notes_create" || listedTools.Tools[1].Name != "notes_list" {
		t.Fatalf("listed MCP tools = %#v, want only annotated note tools", listedTools.Tools)
	}

	invalid, err := session.CallTool(t.Context(), &sdk.CallToolParams{
		Name:      "notes_create",
		Arguments: map[string]any{"body": ""},
	})
	if err != nil {
		t.Fatalf("call invalid MCP tool: %v", err)
	}
	invalidProblem := decodeStructured[problem.Problem](t, invalid.StructuredContent)
	if !invalid.IsError || invalidProblem.Code != problem.CodeValidation ||
		len(invalidProblem.FieldErrors) != 1 || invalidProblem.FieldErrors[0].Field != "body" {
		t.Fatalf("invalid MCP result = %#v, problem %#v", invalid, invalidProblem)
	}

	type callOutcome struct {
		result *sdk.CallToolResult
		err    error
	}
	called := make(chan callOutcome, 1)
	go func() {
		result, err := session.CallTool(t.Context(), &sdk.CallToolParams{
			Name:      "notes_create",
			Arguments: map[string]any{"body": "Created through MCP"},
		})
		called <- callOutcome{result: result, err: err}
	}()

	var published models.Note
	select {
	case published = <-changed:
	case <-time.After(2 * time.Second):
		t.Fatal("committed MCP mutation did not reach the UI callback")
	}
	desktop := appDesktop.NewNotes(application.Readiness(), application.ResolveNotes)
	visible := desktop.List(t.Context(), requests.ListNotesInput{})
	if visible.Problem != nil || visible.Data == nil || len(visible.Data.Notes) != 1 || !sameNote(visible.Data.Notes[0], published) {
		t.Fatalf("desktop-visible notes = %#v, problem %#v, want %#v", visible.Data, visible.Problem, published)
	}

	shutdown := make(chan error, 1)
	go func() { shutdown <- application.Shutdown(t.Context()) }()
	waitForListenerClosed(t, details.Endpoint)
	if err := openedDatabase.SQL.PingContext(t.Context()); err != nil {
		t.Fatalf("database closed before MCP handler drained: %v", err)
	}
	select {
	case err := <-shutdown:
		t.Fatalf("shutdown completed before active MCP handler: %v", err)
	default:
	}

	releaseChange()
	select {
	case outcome := <-called:
		if outcome.err != nil {
			t.Fatalf("call valid MCP tool: %v", outcome.err)
		}
		created := decodeStructured[services.CreateNoteOutput](t, outcome.result.StructuredContent)
		if outcome.result.IsError || !sameNote(created.Note, published) {
			t.Fatalf("created MCP result = %#v, output %#v, want %#v", outcome.result, created, published)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("active MCP call did not finish during graceful shutdown")
	}
	_ = session.Close()
	transport.CloseIdleConnections()
	select {
	case err := <-shutdown:
		if err != nil {
			t.Fatalf("shutdown MCP acceptance runtime: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runtime shutdown did not finish after MCP handler drained")
	}
	if err := openedDatabase.SQL.PingContext(t.Context()); err == nil {
		t.Fatal("shared database remained open after runtime shutdown")
	}
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header = request.Header.Clone()
	request.Header.Set("Authorization", "Bearer "+t.token)
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, fmt.Errorf("send authenticated MCP request: %w", err)
	}
	return response, nil
}

func decodeStructured[Value any](t *testing.T, value any) Value {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal structured MCP value: %v", err)
	}
	var decoded Value
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode structured MCP value: %v", err)
	}
	return decoded
}

func sameNote(left, right models.Note) bool {
	return left.ID == right.ID && left.Body == right.Body && left.CreatedAt.Equal(right.CreatedAt)
}

func waitForListenerClosed(t *testing.T, endpoint string) {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("parse MCP endpoint: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		connection, err := (&net.Dialer{Timeout: 20 * time.Millisecond}).DialContext(t.Context(), "tcp", parsed.Host)
		if err != nil {
			return
		}
		_ = connection.Close()
		if time.Now().After(deadline) {
			t.Fatal("MCP listener remained open during shutdown")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
