package cli

import (
	"fmt"
	"mygo-framework/internal/generator"
	"os"

	"github.com/spf13/cobra"
)

// NewGenerateCommand creates the side-effect-free source generator command.
func NewGenerateCommand() *cobra.Command {
	var check bool
	command := &cobra.Command{
		Use:   "generate",
		Short: "Generate framework-owned adapters and registrations",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			directory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve project directory: %w", err)
			}
			result, err := generator.Generate(command.Context(), generator.GenerateConfig{Dir: directory, Check: check})
			if err != nil {
				return fmt.Errorf("generate framework files: %w", err)
			}
			if check {
				_, err = fmt.Fprintln(command.OutOrStdout(), "generated files are current")
			} else {
				_, err = fmt.Fprintf(command.OutOrStdout(), "generated %d files (%d changed)\n", len(result.Files), len(result.Changed))
			}
			if err != nil {
				return fmt.Errorf("write generation result: %w", err)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&check, "check", false, "fail if generated files would change")
	return command
}
