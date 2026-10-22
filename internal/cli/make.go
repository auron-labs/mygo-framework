package cli

import (
	"fmt"
	"mygo-framework/internal/generator"
	"os"

	"github.com/spf13/cobra"
)

var scaffoldKinds = []string{"service", "request", "model", "migration", "command"}

// NewMakeCommand creates the focused application scaffold commands.
func NewMakeCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "make",
		Short: "Create an application scaffold",
		Args:  cobra.NoArgs,
	}
	for _, kind := range scaffoldKinds {
		command.AddCommand(newMakeTargetCommand(kind))
	}
	command.AddCommand(newMakeFeatureCommand())
	return command
}

func newMakeFeatureCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "feature NAME",
		Short:             "Create a complete feature and regenerate adapters",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, args []string) error {
			directory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve project directory: %w", err)
			}
			result, err := generator.ScaffoldFeature(command.Context(), generator.FeatureConfig{Dir: directory, Name: args[0]})
			if err != nil {
				return fmt.Errorf("create feature scaffold: %w", err)
			}
			_, err = fmt.Fprintf(command.OutOrStdout(), "created %d feature files; generated %d files (%d changed)\n", len(result.Files), len(result.Generated.Files), len(result.Generated.Changed))
			if err != nil {
				return fmt.Errorf("write feature result: %w", err)
			}
			return nil
		},
	}
}

func newMakeTargetCommand(kind string) *cobra.Command {
	return &cobra.Command{
		Use:               kind + " NAME",
		Short:             "Create a " + kind + " scaffold",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(command *cobra.Command, args []string) error {
			directory, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolve project directory: %w", err)
			}
			result, err := generator.Scaffold(generator.ScaffoldConfig{Dir: directory, Kind: kind, Name: args[0]})
			if err != nil {
				return fmt.Errorf("create %s scaffold: %w", kind, err)
			}
			if _, err := fmt.Fprintln(command.OutOrStdout(), "created "+result.Path); err != nil {
				return fmt.Errorf("write scaffold result: %w", err)
			}
			return nil
		},
	}
}
