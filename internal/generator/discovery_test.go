package generator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverReferenceServices(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	methods, err := Discover(context.Background(), DiscoveryConfig{
		Dir:      root,
		Patterns: []string{"./app/services"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var notes []Method
	for _, method := range methods {
		if method.Service == "Notes" {
			notes = append(notes, method)
		}
	}
	if len(notes) != 2 {
		t.Fatalf("note methods = %+v; all methods = %+v", notes, methods)
	}

	create := notes[0]
	if create.PackagePath != "mygo-framework/app/services" || create.Service != "Notes" || create.Name != "Create" ||
		create.Description != "Create validates and stores one note in a service-owned transaction." ||
		create.Command != "notes.create" || create.Input != (NamedType{PackagePath: "mygo-framework/app/requests", Name: "CreateNoteInput"}) ||
		create.Output != (NamedType{PackagePath: "mygo-framework/app/services", Name: "CreateNoteOutput"}) ||
		create.MCP == nil || create.MCP.Name != "notes_create" || create.MCP.ReadOnly || create.MCP.Destructive {
		t.Fatalf("create = %+v", create)
	}
	if filepath.Base(create.Position.Filename) != "notes.go" || create.Position.Line == 0 {
		t.Fatalf("create position = %s", create.Position)
	}

	list := notes[1]
	if list.Name != "List" || list.Command != "notes.list" || list.MCP == nil ||
		list.MCP.Name != "notes_list" || !list.MCP.ReadOnly || list.MCP.Destructive {
		t.Fatalf("list = %+v", list)
	}
}

func TestDiscoverDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "unsupported signature",
			source: `package fixture

import "context"

type Output struct{}
type Service struct{}

// Bad has no concrete input.
// framework:command fixture.bad
func (s *Service) Bad(ctx context.Context) (Output, error) { return Output{}, nil }
`,
			want: "service.go:10:1: marked method Bad must have signature",
		},
		{
			name: "missing effect metadata",
			source: `package fixture

import "context"

type Input struct{}
type Output struct{}
type Service struct{}

// Read reads a fixture.
// framework:mcp fixture_read readonly=true
func (s *Service) Read(ctx context.Context, input Input) (Output, error) { return Output{}, nil }
`,
			want: "framework:mcp requires a tool name and explicit readonly/destructive booleans",
		},
		{
			name: "contradictory effect metadata",
			source: `package fixture

import "context"

type Input struct{}
type Output struct{}
type Service struct{}

// Read reads a fixture.
// framework:mcp fixture_read readonly=true destructive=true
func (s *Service) Read(ctx context.Context, input Input) (Output, error) { return Output{}, nil }
`,
			want: "framework:mcp cannot be both readonly and destructive",
		},
		{
			name: "duplicate declaration",
			source: `package fixture

import "context"

type Input struct{}
type Output struct{}
type First struct{}
type Second struct{}

// Run runs the first fixture.
// framework:command fixture.run
func (s *First) Run(ctx context.Context, input Input) (Output, error) { return Output{}, nil }

// Run runs the second fixture.
// framework:command fixture.run
func (s *Second) Run(ctx context.Context, input Input) (Output, error) { return Output{}, nil }
`,
			want: `duplicate command "fixture.run"; also declared at`,
		},
		{
			name: "missing description",
			source: `package fixture

import "context"

type Input struct{}
type Output struct{}
type Service struct{}

// framework:command fixture.run
func (s *Service) Run(ctx context.Context, input Input) (Output, error) { return Output{}, nil }
`,
			want: "marked method requires a description",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			directory := fixtureModule(t, test.source)
			_, err := Discover(context.Background(), DiscoveryConfig{Dir: directory, Patterns: []string{"./..."}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
			if !strings.Contains(err.Error(), "service.go:") {
				t.Fatalf("error lacks file/line diagnostic: %v", err)
			}
		})
	}
}

func TestDiscoverIndependentMarkersAndStableOrder(t *testing.T) {
	t.Parallel()
	directory := fixtureModule(t, `package fixture

import "context"

type Input struct{}
type Output struct{}
type Service struct{}

// Zebra is exposed only to MCP.
// framework:mcp fixture_zebra readonly=true destructive=false
func (s *Service) Zebra(ctx context.Context, input Input) (Output, error) { return Output{}, nil }

// Alpha is exposed only as a desktop command.
// framework:command fixture.alpha
func (s *Service) Alpha(ctx context.Context, input Input) (Output, error) { return Output{}, nil }

// Ordinary is not exposed.
func (s *Service) Ordinary(ctx context.Context, input Input) (Output, error) { return Output{}, nil }
`)
	methods, err := Discover(context.Background(), DiscoveryConfig{Dir: directory, Patterns: []string{"./..."}})
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 2 || methods[0].Name != "Alpha" || methods[0].Command != "fixture.alpha" || methods[0].MCP != nil ||
		methods[1].Name != "Zebra" || methods[1].Command != "" || methods[1].MCP == nil || methods[1].MCP.Name != "fixture_zebra" {
		t.Fatalf("methods = %+v", methods)
	}
}

func fixtureModule(t *testing.T, source string) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module fixture\n\ngo 1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "service.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}
