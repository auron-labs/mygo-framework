package generator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateInitialRepeatAndCheck(t *testing.T) {
	t.Parallel()
	directory := generatorFixture(t)

	checked, err := Generate(context.Background(), GenerateConfig{Dir: directory, Check: true})
	var stale *StaleError
	if !errors.As(err, &stale) || len(checked.Changed) != 4 || len(stale.Files) != 4 {
		t.Fatalf("initial check = %+v, %v", checked, err)
	}
	for _, name := range checked.Files {
		if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("check wrote %s: %v", name, err)
		}
	}

	generated, err := Generate(context.Background(), GenerateConfig{Dir: directory})
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Files) != 4 || len(generated.Changed) != 4 {
		t.Fatalf("initial generation = %+v", generated)
	}
	contents := map[string][]byte{}
	for _, name := range generated.Files {
		// #nosec G304 -- names are the generator's fixed owned-file set in a temporary fixture.
		content, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), generatedHeader) {
			t.Fatalf("%s lacks generated header", name)
		}
		contents[name] = content
	}

	desktop := string(contents["app/desktop/framework_generated.go"])
	if !strings.Contains(desktop, "type CreateResponse struct") || !strings.Contains(desktop, "func NewNotes") ||
		!strings.Contains(desktop, "func (a *Notes) Create") || strings.Contains(desktop, "func (a *Notes) Inspect") ||
		strings.Contains(desktop, "func NewSearch") {
		t.Fatalf("desktop output:\n%s", desktop)
	}
	bindings := string(contents["framework_bindings_generated.go"])
	if !strings.Contains(bindings, "mygo.Bind(desktop.NewNotes") {
		t.Fatalf("bindings output:\n%s", bindings)
	}
	providers := string(contents["app/bootstrap/framework_generated.go"])
	for _, want := range []string{
		"registerGeneratedServices", "provideNotes", "provideSearch", "resolveRequiredServices", "ResolveNotes",
		"func (b *Bootstrap) mcpResolvers(injector do.Injector)", "resolvers.Notes", "resolvers.Search",
	} {
		if !strings.Contains(providers, want) {
			t.Fatalf("bootstrap output lacks %q:\n%s", want, providers)
		}
	}
	tools := string(contents["app/mcp/framework_generated.go"])
	for _, want := range []string{
		`registerTool(server`,
		`Name:        "fixture_create"`,
		`Description: "Create creates a fixture."`,
		`Name:        "fixture_inspect"`,
		`Name:        "fixture_lookup"`,
		"ReadOnlyHint:    true",
		"DestructiveHint: boolPointer(false)",
		"sdk.ToolHandlerFor[services.Input, services.CreateOutput]",
		"logger *slog.Logger",
		"output, err := service.Create(ctx, input)",
		"MCP resolver Notes is required",
		"MCP resolver Search is required",
	} {
		if !strings.Contains(tools, want) {
			t.Fatalf("MCP output lacks %q:\n%s", want, tools)
		}
	}
	for _, unwanted := range []string{"ToolRegistrationStub", "PackagePath:"} {
		if strings.Contains(tools, unwanted) {
			t.Fatalf("MCP output contains metadata stub %q:\n%s", unwanted, tools)
		}
	}
	if strings.Contains(tools, "fixture_hidden") || strings.Contains(tools, "toolNotesHidden") {
		t.Fatalf("unannotated method was exposed:\n%s", tools)
	}

	repeated, err := Generate(context.Background(), GenerateConfig{Dir: directory})
	if err != nil || len(repeated.Changed) != 0 {
		t.Fatalf("repeated generation = %+v, %v", repeated, err)
	}
	for name, before := range contents {
		// #nosec G304 -- names are the generator's fixed owned-file set in a temporary fixture.
		after, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(after) != string(before) {
			t.Fatalf("%s changed on repeated generation: %v", name, err)
		}
	}

	stalePath := filepath.Join(directory, "app/desktop/framework_generated.go")
	staleContent := append([]byte(nil), contents["app/desktop/framework_generated.go"]...)
	staleContent = append(staleContent, []byte("// stale\n")...)
	if err := os.WriteFile(stalePath, staleContent, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := Generate(context.Background(), GenerateConfig{Dir: directory, Check: true})
	if !errors.As(err, &stale) || len(result.Changed) != 1 || result.Changed[0] != "app/desktop/framework_generated.go" {
		t.Fatalf("stale check = %+v, %v", result, err)
	}
	// #nosec G304 -- stalePath is inside the temporary fixture.
	afterCheck, err := os.ReadFile(stalePath)
	if err != nil || string(afterCheck) != string(staleContent) {
		t.Fatalf("check changed stale file: %v", err)
	}
}

func TestGenerateCompilesWithoutAnnotatedMethods(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	serviceDirectory := filepath.Join(directory, "app", "services")
	if err := os.MkdirAll(serviceDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.test/empty\n\ngo 1.27.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const source = `package services

import "context"

type Input struct{}
type Output struct{}
type Notes struct{}

func (s *Notes) Hidden(ctx context.Context, input Input) (Output, error) {
	return Output{}, nil
}
`
	if err := os.WriteFile(filepath.Join(serviceDirectory, "service.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(context.Background(), GenerateConfig{Dir: directory})
	if err != nil {
		t.Fatalf("generate without annotated methods: %v", err)
	}
	if len(result.Files) != 4 || len(result.Changed) != 4 {
		t.Fatalf("empty generation = %+v", result)
	}
	for _, name := range result.Files {
		// #nosec G304 -- names are the generator's fixed owned-file set in a temporary fixture.
		content, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), generatedHeader) {
			t.Fatalf("%s lacks generated header", name)
		}
	}
	// #nosec G304 -- path is inside the temporary fixture.
	desktop, err := os.ReadFile(filepath.Join(directory, "app", "desktop", "framework_generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(desktop) != generatedHeader+"package desktop\n" {
		t.Fatalf("empty desktop output = %q", desktop)
	}
	// #nosec G304 -- path is inside the temporary fixture.
	tools, err := os.ReadFile(filepath.Join(directory, "app", "mcp", "framework_generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tools), `"log/slog"`) || !strings.Contains(string(tools), "logger *slog.Logger") {
		t.Fatalf("empty MCP output dropped its logger import:\n%s", tools)
	}
}

func TestGenerateRefusesUnownedFile(t *testing.T) {
	t.Parallel()
	directory := generatorFixture(t)
	target := filepath.Join(directory, "app", "desktop", "framework_generated.go")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("package desktop\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Generate(context.Background(), GenerateConfig{Dir: directory})
	if err == nil || !strings.Contains(err.Error(), "refuse to replace non-generated file") {
		t.Fatalf("error = %v", err)
	}
	// #nosec G304 -- target is inside the temporary fixture.
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != "package desktop\n" {
		t.Fatalf("unowned file changed: %q, %v", content, readErr)
	}
}

func generatorFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	serviceDirectory := filepath.Join(directory, "app", "services")
	if err := os.MkdirAll(serviceDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.test/project\n\ngo 1.27.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const source = `package services

import "context"

type Input struct{}
type CreateOutput struct{}
type InspectOutput struct{}
type SearchOutput struct{}
type Notes struct{}
type Search struct{}

// Create creates a fixture.
// framework:command fixture.create
// framework:mcp fixture_create readonly=false destructive=false
func (s *Notes) Create(ctx context.Context, input Input) (CreateOutput, error) {
	return CreateOutput{}, nil
}

// Inspect inspects a fixture without a desktop command.
// framework:mcp fixture_inspect readonly=true destructive=false
func (s *Notes) Inspect(ctx context.Context, input Input) (InspectOutput, error) {
	return InspectOutput{}, nil
}

// Hidden is an ordinary exported method without MCP exposure.
func (s *Notes) Hidden(ctx context.Context, input Input) (InspectOutput, error) {
	return InspectOutput{}, nil
}

// Lookup searches the fixture without a desktop command.
// framework:mcp fixture_lookup readonly=true destructive=false
func (s *Search) Lookup(ctx context.Context, input Input) (SearchOutput, error) {
	return SearchOutput{}, nil
}
`
	if err := os.WriteFile(filepath.Join(serviceDirectory, "service.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}
