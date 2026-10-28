package generator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldFeatureCreatesAndGeneratesWithoutReplacingImplementation(t *testing.T) {
	t.Parallel()
	directory := featureFixture(t)

	result, err := ScaffoldFeature(context.Background(), FeatureConfig{Dir: directory, Name: "Widget"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 5 || len(result.Generated.Files) != 4 || len(result.Generated.Changed) != 4 {
		t.Fatalf("feature result = %+v", result)
	}
	for _, expected := range []string{
		"app/models/widget.go",
		"app/requests/widget.go",
		"app/services/widget.go",
		"app/bootstrap/widget.go",
		"database/migrations/00001_create_widget_records.sql",
	} {
		if !containsString(result.Files, expected) {
			t.Errorf("feature files lack %q: %v", expected, result.Files)
		}
	}

	assertFileContains(t, directory, "app/desktop/framework_generated.go", "type CreateWidgetResponse struct")
	assertFileContains(t, directory, "framework_bindings_generated.go", "mygo.Bind(desktop.NewWidget")
	assertFileContains(t, directory, "app/bootstrap/framework_generated.go", "do.Provide(bootstrap.injector, provideWidget)")
	assertFileContains(t, directory, "app/services/widget.go", "framework:command widget.create")

	servicePath := filepath.Join(directory, "app", "services", "widget.go")
	// #nosec G304 -- servicePath is inside the temporary project.
	service, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatal(err)
	}
	service = append(service, []byte("\nconst HandwrittenImplementation = true\n")...)
	// #nosec G703 -- servicePath is inside the temporary project.
	if err := os.WriteFile(servicePath, service, 0o600); err != nil {
		t.Fatal(err)
	}
	repeated, err := Generate(context.Background(), GenerateConfig{Dir: directory})
	if err != nil || len(repeated.Changed) != 0 {
		t.Fatalf("repeat generation = %+v, %v", repeated, err)
	}
	// #nosec G304 -- servicePath is inside the temporary project.
	after, err := os.ReadFile(servicePath)
	if err != nil || !strings.Contains(string(after), "HandwrittenImplementation") {
		t.Fatalf("feature implementation changed: %v\n%s", err, after)
	}

	_, err = ScaffoldFeature(context.Background(), FeatureConfig{Dir: directory, Name: "Widget"})
	if err == nil || !strings.Contains(err.Error(), "refuse to overwrite") {
		t.Fatalf("duplicate feature error = %v", err)
	}
}

func TestScaffoldFeaturePreflightsEveryHandwrittenFile(t *testing.T) {
	t.Parallel()
	directory := featureFixture(t)
	target := filepath.Join(directory, "app", "requests", "widget.go")
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	const handwritten = "package requests\n\nconst Keep = true\n"
	if err := os.WriteFile(target, []byte(handwritten), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ScaffoldFeature(context.Background(), FeatureConfig{Dir: directory, Name: "Widget"})
	if err == nil || !strings.Contains(err.Error(), "refuse to overwrite existing file app/requests/widget.go") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "app", "models", "widget.go")); !os.IsNotExist(statErr) {
		t.Fatalf("feature wrote before preflight completed: %v", statErr)
	}
	// #nosec G304 -- target is inside the temporary project.
	content, readErr := os.ReadFile(target)
	if readErr != nil || string(content) != handwritten {
		t.Fatalf("hand-written request changed: %q, %v", content, readErr)
	}
}

func TestScaffoldFeatureRollsBackWhenGenerationFails(t *testing.T) {
	t.Parallel()
	directory := featureFixture(t)
	broken := filepath.Join(directory, "app", "services", "broken.go")
	const source = `package services

// Broken has an invalid marker.
// framework:command
func (s *Notes) Broken() {}
`
	if err := os.WriteFile(broken, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ScaffoldFeature(context.Background(), FeatureConfig{Dir: directory, Name: "Widget"})
	if err == nil || !strings.Contains(err.Error(), "generate feature adapters") {
		t.Fatalf("error = %v", err)
	}
	for _, path := range []string{
		"app/models/widget.go",
		"app/requests/widget.go",
		"app/services/widget.go",
		"app/bootstrap/widget.go",
		"database/migrations/00001_create_widget_records.sql",
	} {
		if _, statErr := os.Stat(filepath.Join(directory, path)); !os.IsNotExist(statErr) {
			t.Errorf("failed feature left %s: %v", path, statErr)
		}
	}
}

func featureFixture(t *testing.T) string {
	t.Helper()
	directory := generatorFixture(t)
	const module = `module example.test/project

go 1.27.3

require gorm.io/gorm v0.0.0

replace gorm.io/gorm => ./stubs/gorm
`
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatal(err)
	}
	validationDirectory := filepath.Join(directory, "validation")
	if err := os.MkdirAll(validationDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	const validation = `package validation

type Validator struct{}

func (v *Validator) Struct(any) error { return nil }
`
	if err := os.WriteFile(filepath.Join(validationDirectory, "validation.go"), []byte(validation), 0o600); err != nil {
		t.Fatal(err)
	}
	stubDirectory := filepath.Join(directory, "stubs", "gorm")
	if err := os.MkdirAll(stubDirectory, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stubDirectory, "go.mod"), []byte("module gorm.io/gorm\n\ngo 1.27.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const gorm = `package gorm

import "context"

type DB struct{ Error error }

func (d *DB) WithContext(context.Context) *DB { return d }
func (d *DB) Create(any) *DB { return d }
func (d *DB) Transaction(run func(*DB) error) error { return run(d) }
`
	if err := os.WriteFile(filepath.Join(stubDirectory, "gorm.go"), []byte(gorm), 0o600); err != nil {
		t.Fatal(err)
	}
	return directory
}

func assertFileContains(t *testing.T, directory, path, expected string) {
	t.Helper()
	// #nosec G304 -- path is fixed by the test under the temporary project.
	content, err := os.ReadFile(filepath.Join(directory, path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), expected) {
		t.Fatalf("%s does not contain %q:\n%s", path, expected, content)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
