package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mygo-framework/internal/generator"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type (
	frameworkGenerator func(context.Context, generator.GenerateConfig) (generator.GenerateResult, error)
	mygoRunner         func(context.Context, string, io.Reader, io.Writer, io.Writer, ...string) error
)

// NewDevCommand prepares web adapters when needed and starts MyGo development.
func NewDevCommand() *cobra.Command {
	return newMyGoCommand("dev", false, generator.Generate, runMyGo)
}

// NewBuildCommand prepares web adapters when needed and delegates packaging to MyGo.
func NewBuildCommand() *cobra.Command {
	return newMyGoCommand("build", false, generator.Generate, runMyGo)
}

// NewDoctorCommand checks web adapters when needed and runs MyGo's environment doctor.
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
			_, nativeErr := os.Stat(filepath.Join(directory, "mygo.json"))
			if nativeErr != nil && !errors.Is(nativeErr, os.ErrNotExist) {
				return fmt.Errorf("inspect MyGo configuration: %w", nativeErr)
			}
			if nativeErr != nil {
				if _, err := generate(command.Context(), generator.GenerateConfig{Dir: directory, Check: check}); err != nil {
					return fmt.Errorf("prepare framework files for %s: %w", name, err)
				}
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
		return "Start MyGo development (generate adapters for web projects)"
	case "build":
		return "Build with MyGo (generate adapters for web projects)"
	case "doctor":
		return "Run the MyGo doctor (check adapters for web projects)"
	default:
		return "Run MyGo"
	}
}

func runMyGo(ctx context.Context, directory string, stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	executable := "bun"
	arguments := append([]string{"run", "mygo"}, args...)
	if _, err := os.Stat(filepath.Join(directory, "mygo.json")); err == nil {
		executable = "go"
		arguments = append([]string{"tool", "mygo"}, args...)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect MyGo configuration: %w", err)
	}
	// #nosec G204 -- the executable is fixed and remaining arguments are intentionally delegated to MyGo.
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = directory
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("execute %s %s: %w", executable, strings.Join(arguments, " "), err)
	}
	return nil
}
