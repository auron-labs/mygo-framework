// Package mcp tests the generated MCP registrations with the official SDK.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mygo-framework/app/models"
	"mygo-framework/app/requests"
	"mygo-framework/app/services"
	"mygo-framework/database/migrations"
	"mygo-framework/internal/database"
	"mygo-framework/problem"
	"mygo-framework/validation"
	"path/filepath"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGeneratedToolsPublishTypedSchemasAndEffects(t *testing.T) {
	t.Parallel()

	server := sdk.NewServer(&sdk.Implementation{Name: "generated-tools-test", Version: "test"}, nil)
	RegisterGeneratedTools(server, Resolvers{Notes: func() (*services.Notes, error) {
		return &services.Notes{}, nil
	}}, slog.New(slog.DiscardHandler))
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP server: %v", err)
	}
	t.Cleanup(func() {
		if err := serverSession.Close(); err != nil {
			t.Errorf("close MCP server session: %v", err)
		}
	})
	client := sdk.NewClient(&sdk.Implementation{Name: "generated-tools-test", Version: "test"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() {
		if err := clientSession.Close(); err != nil {
			t.Errorf("close MCP client session: %v", err)
		}
	})

	result, err := clientSession.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("list generated tools: %v", err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("generated tools = %d, want 2 explicitly annotated methods", len(result.Tools))
	}
	tools := map[string]*sdk.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	create := tools["notes_create"]
	if create == nil || create.Description == "" || create.Annotations == nil || create.Annotations.ReadOnlyHint ||
		create.Annotations.DestructiveHint == nil || *create.Annotations.DestructiveHint {
		t.Fatalf("notes_create metadata = %#v", create)
	}
	body := schemaProperty(t, create.InputSchema, "body")
	if body["description"] != "The note body to store." {
		t.Fatalf("body schema = %#v", body)
	}
	if schemaProperty(t, create.OutputSchema, "note")["description"] != "The created note." {
		t.Fatalf("note output schema = %#v", create.OutputSchema)
	}
	list := tools["notes_list"]
	if list == nil || list.Description == "" || list.Annotations == nil || !list.Annotations.ReadOnlyHint ||
		list.Annotations.DestructiveHint == nil || *list.Annotations.DestructiveHint {
		t.Fatalf("notes_list metadata = %#v", list)
	}
}

func TestGeneratedHandlersCallOrdinaryServiceMethods(t *testing.T) {
	t.Parallel()

	openedDatabase, err := database.Open(
		t.Context(),
		filepath.Join(t.TempDir(), "notes.db"),
		migrations.FS(),
	)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := openedDatabase.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	service := services.NewNotes(openedDatabase.GORM, validation.New(), func(context.Context, models.Note) {})
	resolve := func() (*services.Notes, error) { return service, nil }

	callResult, created, err := toolNotesCreate(resolve)(
		t.Context(),
		nil,
		requests.CreateNoteInput{Body: "  Generated tool  "},
	)
	if err != nil {
		t.Fatalf("call generated create handler: %v", err)
	}
	if callResult != nil || created.Note.ID == 0 || created.Note.Body != "Generated tool" {
		t.Fatalf("create result = %#v, %#v", callResult, created)
	}
	_, listed, err := toolNotesList(resolve)(t.Context(), nil, requests.ListNotesInput{})
	if err != nil {
		t.Fatalf("call generated list handler: %v", err)
	}
	if len(listed.Notes) != 1 || listed.Notes[0].ID != created.Note.ID {
		t.Fatalf("listed notes = %#v", listed.Notes)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _, err = toolNotesList(resolve)(ctx, nil, requests.ListNotesInput{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled list error = %v, want context cancellation", err)
	}
}

func TestGeneratedToolsMapDomainFailures(t *testing.T) {
	t.Parallel()

	openedDatabase, err := database.Open(
		t.Context(),
		filepath.Join(t.TempDir(), "notes.db"),
		migrations.FS(),
	)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := openedDatabase.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	service := services.NewNotes(openedDatabase.GORM, validation.New(), func(context.Context, models.Note) {})
	server := sdk.NewServer(&sdk.Implementation{Name: "problem-test", Version: "test"}, nil)
	RegisterGeneratedTools(server, Resolvers{Notes: func() (*services.Notes, error) { return service, nil }}, slog.New(slog.DiscardHandler))
	serverTransport, clientTransport := sdk.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := sdk.NewClient(&sdk.Implementation{Name: "problem-test", Version: "test"}, nil)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("connect MCP client: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	result, err := clientSession.CallTool(t.Context(), &sdk.CallToolParams{
		Name:      "notes_create",
		Arguments: map[string]any{"body": ""},
	})
	if err != nil {
		t.Fatalf("call invalid note tool: %v", err)
	}
	if !result.IsError || len(result.Content) != 1 {
		t.Fatalf("tool result = %#v, want one error content item", result)
	}
	text, ok := result.Content[0].(*sdk.TextContent)
	if !ok || text.Text != "Some fields are invalid." {
		t.Fatalf("tool error text = %#v", result.Content[0])
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured problem: %v", err)
	}
	var mapped problem.Problem
	if err := json.Unmarshal(encoded, &mapped); err != nil {
		t.Fatalf("decode structured problem: %v", err)
	}
	if mapped.Code != problem.CodeValidation || len(mapped.FieldErrors) != 1 || mapped.FieldErrors[0].Field != "body" {
		t.Fatalf("structured problem = %#v", mapped)
	}
}

func schemaProperty(t *testing.T, schema any, name string) map[string]any {
	t.Helper()
	root, ok := schema.(map[string]any)
	if !ok {
		t.Fatalf("schema type = %T, want map[string]any", schema)
	}
	properties, ok := root["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %#v", root["properties"])
	}
	property, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("schema property %q = %#v", name, properties[name])
	}
	return property
}
