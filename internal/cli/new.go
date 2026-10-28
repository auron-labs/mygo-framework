package cli

import (
	"fmt"
	"mygo-framework/internal/generator"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// NewProjectCommand creates a MyGo web or native project.
func NewProjectCommand() *cobra.Command {
	var native bool
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
			return nil
		},
	}
	command.Flags().BoolVar(&native, "native", false, "Create a native Onevra app without a web frontend")
	return command
}
