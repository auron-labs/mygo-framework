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
	return command
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
