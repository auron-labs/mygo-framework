package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootHelpAndVersion(t *testing.T) {
	t.Parallel()

	help, err := execute(context.Background(), New("1.2.3"), "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Build and maintain MyGo applications",
		"framework [command]",
		"completion  Generate a shell completion script",
		"-h, --help",
		"-v, --version",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("help does not contain %q:\n%s", want, help)
		}
	}

	version, err := execute(context.Background(), New("1.2.3"), "--version")
	if err != nil {
		t.Fatal(err)
	}
	if version != "framework 1.2.3\n" {
		t.Fatalf("version = %q", version)
	}
}

func TestCompletionAndExtensionsAreSideEffectFree(t *testing.T) {
	t.Chdir(t.TempDir())

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	runs := 0
	generate := &cobra.Command{
		Use:  "generate",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if command.Context().Value(contextKey{}) != "request" {
				t.Fatal("command context was not propagated")
			}
			runs++
			return nil
		},
	}

	root := New("test", generate)
	completion, err := execute(ctx, root, "completion", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(completion, "__start_framework") {
		t.Fatal("bash completion script was not generated")
	}
	if runs != 0 {
		t.Fatalf("extension ran during completion: %d", runs)
	}

	if _, err = execute(ctx, New("test", generate), "generate"); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("extension runs = %d", runs)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("CLI created files: %v", entries)
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	t.Parallel()

	_, err := execute(context.Background(), New("test"), "completion", "tcsh")
	if err == nil || !strings.Contains(err.Error(), "invalid argument") {
		t.Fatalf("error = %v", err)
	}
}

func execute(ctx context.Context, command *cobra.Command, args ...string) (string, error) {
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&bytes.Buffer{})
	command.SetArgs(args)
	err := command.ExecuteContext(ctx)
	return stdout.String(), err
}
