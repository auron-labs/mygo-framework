package cli

import (
	"context"
	"fmt"
	"io"
	"mygo-framework/internal/generator"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type (
	frameworkGenerator func(context.Context, generator.GenerateConfig) (generator.GenerateResult, error)
	mygoRunner         func(context.Context, string, io.Reader, io.Writer, io.Writer, ...string) error
)

// NewDevCommand generates framework files once and starts MyGo development.
func NewDevCommand() *cobra.Command {
	return newMyGoCommand("dev", false, generator.Generate, runMyGo)
}

// NewBuildCommand generates framework files once and delegates the production build to MyGo.
func NewBuildCommand() *cobra.Command {
	return newMyGoCommand("build", false, generator.Generate, runMyGo)
}

// NewDoctorCommand checks framework output once before running MyGo's environment doctor.
func NewDoctorCommand() *cobra.Command {
	return newMyGoCommand("doctor", true, generator.Generate, runMyGo)
}

func newMyGoCommand(name string, check bool, generate frameworkGenerator, run mygoRunner) *cobra.Command {
	return &cobra.Command{
		Use:                name + " [mygo flags]",
		Short:              myGoCommandDescription(name),
		DisableFlagParsing: true,
		RunE: func(command *cobra.Command, args []string) error {
			directory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve project directory: %w", err)
			}
			if _, err := generate(command.Context(), generator.GenerateConfig{Dir: directory, Check: check}); err != nil {
				return fmt.Errorf("prepare framework files for %s: %w", name, err)
			}
			if err := run(command.Context(), directory, command.InOrStdin(), command.OutOrStdout(), command.ErrOrStderr(), append([]string{name}, args...)...); err != nil {
				return fmt.Errorf("run MyGo %s: %w", name, err)
			}
			return nil
		},
	}
}

func myGoCommandDescription(name string) string {
	switch name {
	case "dev":
		return "Generate framework files and start MyGo development"
	case "build":
		return "Generate framework files and build with MyGo"
	case "doctor":
		return "Check framework files and run the MyGo doctor"
	default:
		return "Run MyGo"
	}
}

func runMyGo(ctx context.Context, directory string, stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	arguments := append([]string{"run", "mygo"}, args...)
	// #nosec G204 -- the executable is fixed and remaining arguments are intentionally delegated to MyGo.
	command := exec.CommandContext(ctx, "bun", arguments...)
	command.Dir = directory
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("execute bun %s: %w", strings.Join(arguments, " "), err)
	}
	return nil
}
