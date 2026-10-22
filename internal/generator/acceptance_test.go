package generator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const generatorAcceptanceEnvironment = "MYGO_GENERATOR_ACCEPTANCE"

func TestGeneratedProjectAcceptance(t *testing.T) {
	if os.Getenv(generatorAcceptanceEnvironment) != "1" {
		t.Skip("set " + generatorAcceptanceEnvironment + "=1 to run the pinned Go/Bun/MyGo acceptance flow")
	}

	repository := repositoryRoot(t)
	directory := t.TempDir()
	framework := filepath.Join(directory, "framework")
	if runtime.GOOS == "windows" {
		framework += ".exe"
	}
	runAcceptanceCommand(t, repository, "go", "build", "-o", framework, "./cmd/framework")
	runAcceptanceCommand(t, directory, framework, "new", "AcceptanceApp")

	project := filepath.Join(directory, "acceptance-app")
	runAcceptanceCommand(t, project, framework, "make", "feature", "Bookmark")

	servicePath := filepath.Join(project, "app", "services", "bookmark.go")
	replaceAcceptanceFile(t, servicePath,
		"// framework:command bookmark.create\n",
		"// framework:command bookmark.create\n// framework:mcp bookmarks_create readonly=false destructive=false\n",
	)
	runAcceptanceCommand(t, project, framework, "generate")
	appendAcceptanceFile(t, servicePath, "\nconst AcceptanceImplementation = true\n")
	serviceDigest := acceptanceDigest(t, servicePath)

	generatedPaths := []string{
		"app/bootstrap/framework_generated.go",
		"app/desktop/framework_generated.go",
		"app/mcp/framework_generated.go",
		"framework_bindings_generated.go",
	}
	generatedBefore := acceptanceDigests(t, project, generatedPaths)
	output := runAcceptanceCommand(t, project, framework, "generate")
	if !strings.Contains(output, "0 changed") {
		t.Fatalf("repeat framework generation output = %q, want no changes", output)
	}
	assertAcceptanceDigests(t, project, generatedBefore)
	assertAcceptanceDigest(t, servicePath, serviceDigest)

	runAcceptanceCommand(t, project, "bun", "install", "--frozen-lockfile")
	runAcceptanceCommand(t, project, "bun", "run", "generate")
	clientPath := filepath.Join(project, "src", "mygo.ts")
	clientDigest := acceptanceDigest(t, clientPath)
	runAcceptanceCommand(t, project, "bun", "run", "generate")
	assertAcceptanceDigest(t, clientPath, clientDigest)

	clientTestPath := filepath.Join(project, "tests", "web", "generated_client_acceptance.test.ts")
	// #nosec G306 -- the disposable fixture contains source code, not credentials.
	if err := os.WriteFile(clientTestPath, []byte(generatedClientAcceptanceTest), 0o600); err != nil {
		t.Fatalf("write generated-client acceptance test: %v", err)
	}
	capabilitiesTestPath := filepath.Join(project, "framework_capabilities_acceptance_test.go")
	// #nosec G306 -- the disposable fixture contains source code, not credentials.
	if err := os.WriteFile(capabilitiesTestPath, []byte(frameworkCapabilitiesAcceptanceTest), 0o600); err != nil {
		t.Fatalf("write framework-capabilities acceptance test: %v", err)
	}
	runAcceptanceCommand(t, project, "bun", "run", "test:web")
	runAcceptanceCommand(t, project, "go", "test", "./...")
	runAcceptanceCommand(t, project, "go", "build", "./...")
	runAcceptanceCommand(t, project, "go", "mod", "tidy", "-diff")
	runAcceptanceCommand(t, project, "bun", "run", "build:web")

	stalePath := filepath.Join(project, "app", "desktop", "framework_generated.go")
	appendAcceptanceFile(t, stalePath, "// deliberate stale output\n")
	staleDigest := acceptanceDigest(t, stalePath)
	output, err := acceptanceCommand(t.Context(), project, framework, "generate", "--check")
	if err == nil || !strings.Contains(output, "app/desktop/framework_generated.go") {
		t.Fatalf("stale check output = %q, error = %v", output, err)
	}
	assertAcceptanceDigest(t, stalePath, staleDigest)

	runAcceptanceCommand(t, project, framework, "generate")
	runAcceptanceCommand(t, project, framework, "generate", "--check")
	assertAcceptanceDigest(t, servicePath, serviceDigest)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve acceptance test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func runAcceptanceCommand(t *testing.T, directory, name string, arguments ...string) string {
	t.Helper()
	output, err := acceptanceCommand(t.Context(), directory, name, arguments...)
	if err != nil {
		t.Fatalf("run %s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
	return output
}

func acceptanceCommand(ctx context.Context, directory, name string, arguments ...string) (string, error) {
	// #nosec G204 -- this opt-in test executes fixed tools with test-owned paths and arguments.
	command := exec.CommandContext(ctx, name, arguments...)
	command.Dir = directory
	command.Env = os.Environ()
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	return output.String(), err
}

func appendAcceptanceFile(t *testing.T, path, content string) {
	t.Helper()
	// #nosec G304 -- every path is fixed beneath the disposable acceptance project.
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open acceptance file %s: %v", filepath.Base(path), err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		t.Fatalf("append acceptance file %s: %v", filepath.Base(path), err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close acceptance file %s: %v", filepath.Base(path), err)
	}
}

func replaceAcceptanceFile(t *testing.T, path, old, replacement string) {
	t.Helper()
	// #nosec G304 -- every path is fixed beneath the disposable acceptance project.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read acceptance file %s: %v", filepath.Base(path), err)
	}
	updated := strings.Replace(string(content), old, replacement, 1)
	if updated == string(content) {
		t.Fatalf("acceptance file %s does not contain %q", filepath.Base(path), old)
	}
	// #nosec G306,G703 -- path is fixed beneath the disposable project; content is source code.
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Fatalf("write acceptance file %s: %v", filepath.Base(path), err)
	}
}

func acceptanceDigest(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()
	// #nosec G304 -- every path is fixed beneath the disposable acceptance project.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read acceptance file %s: %v", filepath.Base(path), err)
	}
	return sha256.Sum256(content)
}

func acceptanceDigests(t *testing.T, directory string, paths []string) map[string][sha256.Size]byte {
	t.Helper()
	digests := make(map[string][sha256.Size]byte, len(paths))
	for _, path := range paths {
		digests[path] = acceptanceDigest(t, filepath.Join(directory, filepath.FromSlash(path)))
	}
	return digests
}

func assertAcceptanceDigests(t *testing.T, directory string, expected map[string][sha256.Size]byte) {
	t.Helper()
	for path, digest := range expected {
		assertAcceptanceDigest(t, filepath.Join(directory, filepath.FromSlash(path)), digest)
	}
}

func assertAcceptanceDigest(t *testing.T, path string, expected [sha256.Size]byte) {
	t.Helper()
	if actual := acceptanceDigest(t, path); actual != expected {
		t.Fatalf("acceptance file %s changed: got %x, want %x", filepath.Base(path), actual, expected)
	}
}

const generatedClientAcceptanceTest = `import { expect, test } from "bun:test";
import { Bookmark } from "../../src/mygo";

test("invokes the scaffolded feature through the MyGo-generated client", async () => {
  const calls: Array<{ method: string; args: unknown[] }> = [];
  const response = {
    data: { bookmark: { id: 7, name: "Accepted", createdAt: "2026-10-22T00:00:00Z" } },
    problem: null,
  };
  Object.assign(globalThis, {
    mygo: {
      call: async (method: string, ...args: unknown[]) => {
        calls.push({ method, args });
        return response;
      },
    },
  });

  try {
    await expect(Bookmark.create({ name: "Accepted" })).resolves.toEqual(response);
    expect(calls).toEqual([
      { method: "Bookmark.Create", args: [{ name: "Accepted" }] },
    ]);
  } finally {
    Reflect.deleteProperty(globalThis, "mygo");
  }
});
`

const frameworkCapabilitiesAcceptanceTest = `package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"acceptance-app/database/migrations"
	"acceptance-app/httpclient"
	appdb "acceptance-app/internal/database"
	secretmemory "acceptance-app/secrets/memory"
	"acceptance-app/settings"
	settingsbolt "acceptance-app/settings/bbolt"
	settingssqlite "acceptance-app/settings/sqlite"
)

func TestGeneratedFrameworkCapabilities(t *testing.T) {
	ctx := context.Background()
	database, err := appdb.Open(ctx, filepath.Join(t.TempDir(), "app.db"), migrations.FS())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Error(err)
		}
	})

	enabled := settings.NewKey("feature.enabled", false, nil)
	sqliteStore, err := settingssqlite.New(database.SQL)
	if err != nil {
		t.Fatal(err)
	}
	values, err := settings.New(sqliteStore, enabled)
	if err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(ctx, values, enabled, true); err != nil {
		t.Fatal(err)
	}
	if value, err := settings.Get(ctx, values, enabled); err != nil || !value {
		t.Fatalf("SQLite setting = %v, %v", value, err)
	}

	boltStore, err := settingsbolt.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := boltStore.Close(); err != nil {
			t.Error(err)
		}
	})
	boltValues, err := settings.New(boltStore, enabled)
	if err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(ctx, boltValues, enabled, true); err != nil {
		t.Fatal(err)
	}
	if value, err := settings.Get(ctx, boltValues, enabled); err != nil || !value {
		t.Fatalf("bbolt setting = %v, %v", value, err)
	}

	secretStore := secretmemory.New()
	if err := secretStore.Set(ctx, "token", []byte("accepted")); err != nil {
		t.Fatal(err)
	}
	if value, err := secretStore.Get(ctx, "token"); err != nil || string(value) != "accepted" {
		t.Fatalf("secret = %q, %v", value, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"name": "accepted"})
	}))
	defer server.Close()
	client, err := httpclient.New(httpclient.Options{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Name string ` + "`json:\"name\"`" + `
	}
	if _, err := client.Request(ctx, "acceptance.http").SetResult(&result).Get("/"); err != nil {
		t.Fatal(err)
	}
	if result.Name != "accepted" {
		t.Fatalf("HTTP result = %+v", result)
	}
}
`
