package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/auron-labs/mygo-framework/internal/generator"

	"github.com/spf13/cobra"
)

// frameworkResolver adds the framework module to a new project's go.mod and
// completes its dependency graph. A non-empty checkout replaces the published
// module with that local directory.
type frameworkResolver func(ctx context.Context, directory, checkout string, stdout, stderr io.Writer) error

// NewProjectCommand creates a MyGo web or native project.
func NewProjectCommand() *cobra.Command {
	return newProjectCommand(resolveFramework)
}

func newProjectCommand(resolve frameworkResolver) *cobra.Command {
	var native bool
	var checkout string
	command := &cobra.Command{
		Use:               "new NAME",
		Short:             "Create a MyGo framework application",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, args []string) error {
			directory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve parent directory: %w", err)
			}
			result, err := generator.CreateProject(command.Context(), generator.ProjectConfig{Dir: directory, Name: args[0], Native: native})
			if err != nil {
				return fmt.Errorf("create project: %w", err)
			}
			if _, err := fmt.Fprintf(command.OutOrStdout(), "created %s (%d files)\n", filepath.Base(result.Path), result.Files); err != nil {
				return fmt.Errorf("write project result: %w", err)
			}
			if err := resolve(command.Context(), result.Path, checkout, command.OutOrStdout(), command.ErrOrStderr()); err != nil {
				return fmt.Errorf("resolve dependencies of %s (run go mod tidy there once the network is available): %w", filepath.Base(result.Path), err)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&native, "native", false, "Create a native Onevra app without a web frontend")
	command.Flags().StringVar(&checkout, "framework", "", "Use the framework checkout in `DIR` instead of the published module")
	return command
}

// frameworkVersion is the framework release the CLI was installed from, or
// main for a build from a checkout.
func frameworkVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && strings.HasPrefix(info.Main.Version, "v") && !strings.Contains(info.Main.Version, "+") {
		return info.Main.Version
	}
	return "main"
}

func resolveFramework(ctx context.Context, directory, checkout string, stdout, stderr io.Writer) error {
	steps := [][]string{{"get", generator.FrameworkModule + "@" + frameworkVersion()}, {"mod", "tidy"}}
	if checkout != "" {
		absolute, err := filepath.Abs(checkout)
		if err != nil {
			return fmt.Errorf("resolve framework checkout: %w", err)
		}
		steps = [][]string{
			{"mod", "edit", "-require=" + generator.FrameworkModule + "@v0.0.0", "-replace=" + generator.FrameworkModule + "=" + absolute},
			{"mod", "tidy"},
		}
	}
	for _, arguments := range steps {
		command := exec.CommandContext(ctx, "go", arguments...) // #nosec G204 -- fixed go subcommands with the framework module path.
		command.Dir = directory
		command.Stdout = stdout
		command.Stderr = stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("go %s: %w", strings.Join(arguments, " "), err)
		}
	}
	return nil
}
