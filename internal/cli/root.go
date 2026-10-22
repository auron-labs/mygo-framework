// Package cli defines the framework command-line interface.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const developmentVersion = "dev"

// New creates the side-effect-free framework command root.
func New(version string, commands ...*cobra.Command) *cobra.Command {
	if strings.TrimSpace(version) == "" {
		version = developmentVersion
	}

	root := &cobra.Command{
		Use:           "framework",
		Short:         "Build and maintain MyGo applications",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("framework {{.Version}}\n")
	root.AddCommand(newCompletionCommand())
	root.AddCommand(commands...)
	return root
}

func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:                   "completion [bash|fish|powershell|zsh]",
		Short:                 "Generate a shell completion script",
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs:             []string{"bash", "fish", "powershell", "zsh"},
		ValidArgsFunction:     cobra.NoFileCompletions,
		DisableFlagsInUseLine: true,
		RunE: func(command *cobra.Command, args []string) error {
			output := command.OutOrStdout()
			switch args[0] {
			case "bash":
				return command.Root().GenBashCompletionV2(output, true)
			case "fish":
				return command.Root().GenFishCompletion(output, true)
			case "powershell":
				return command.Root().GenPowerShellCompletionWithDesc(output)
			case "zsh":
				return command.Root().GenZshCompletion(output)
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
}
