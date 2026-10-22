package generator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateProjectWritesPinnedSupportedTemplate(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()

	result, err := CreateProject(context.Background(), ProjectConfig{Dir: parent, Name: "AcmeNotes"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Path != filepath.Join(parent, "acme-notes") || result.Files < 35 {
		t.Fatalf("result = %+v", result)
	}
	checks := map[string][]string{
		"go.mod":                      {"module acme-notes", "github.com/egoist/mygo v0.5.5", "gorm.io/gorm v1.31.2"},
		"package.json":                {`"name": "acme-notes"`, `"packageManager": "bun@1.4.2"`, `"mygo-cli": "0.5.5"`},
		"mygo.config.ts":              {`name: "Acme Notes"`, `identifier: "com.example.acmenotes"`},
		"app/profile/profile.go":      {`Name`, `"Acme Notes"`, `databaseFilename`},
		"app/bootstrap/bootstrap.go":  {`"acme-notes/app/services"`, "registerGeneratedServices"},
		"app/mcp/runtime.go":          {`net.JoinHostPort("127.0.0.1"`, "Stateless:", "NewCrossOriginProtection", "requireBearerToken", "Shutdown(ctx context.Context)"},
		"main.go":                     {"MYGO_MCP_ENABLED", "MYGO_MCP_PORT", "MYGO_MCP_DISCLOSE"},
		"cmd/appctl/main.go":          {`"acme-notes/internal/appctl"`, "migrations.FS()"},
		"internal/appctl/database.go": {"InspectMigrations", "Migrate"},
		"internal/appctl/root.go":     {"data-directory"},
		"database/migrations/00001_create_notes.sql": {"-- +goose Up", "CREATE TABLE notes"},
		"src/framework/form.ts":                      {"export const createForm"},
		"AGENTS.md":                                  {"framework make feature NAME", "Never hand-edit"},
		"framework_bindings_generated.go":            {generatedHeader, `"acme-notes/app/bootstrap"`},
		".git/HEAD":                                  {"ref: refs/heads/"},
	}
	for path, expected := range checks {
		assertFileContainsAll(t, result.Path, path, expected...)
	}

	err = filepath.WalkDir(result.Path, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		// #nosec G304,G122 -- path comes from walking the newly created temporary project, which contains no symlinks.
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read created project file: %w", readErr)
		}
		if strings.Contains(string(content), "mygo-framework") {
			t.Errorf("%s retains the template module name", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateProjectRefusesExistingTarget(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	target := filepath.Join(parent, "existing-app")
	if err := os.Mkdir(target, 0o750); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := CreateProject(context.Background(), ProjectConfig{Dir: parent, Name: "ExistingApp"})
	if err == nil || !strings.Contains(err.Error(), "refuse to overwrite") {
		t.Fatalf("error = %v", err)
	}
	// #nosec G304 -- sentinel is inside the temporary project.
	content, readErr := os.ReadFile(sentinel)
	if readErr != nil || string(content) != "keep\n" {
		t.Fatalf("existing project changed: %q, %v", content, readErr)
	}
}

func TestCreateProjectRejectsInvalidNameAndCancellation(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	if _, err := CreateProject(context.Background(), ProjectConfig{Dir: parent, Name: "../escape"}); err == nil {
		t.Fatal("unsafe project name succeeded")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CreateProject(ctx, ProjectConfig{Dir: parent, Name: "CanceledApp"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(parent, "canceled-app")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("canceled project exists: %v", statErr)
	}
}

func assertFileContainsAll(t *testing.T, directory, path string, expected ...string) {
	t.Helper()
	// #nosec G304 -- path is fixed by the test under the temporary project.
	content, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range expected {
		if !strings.Contains(string(content), value) {
			t.Errorf("%s does not contain %q", path, value)
		}
	}
}
