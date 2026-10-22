// Package appctl defines offline commands for the reference application.
package appctl

import (
	"io/fs"
	"strings"

	"github.com/spf13/cobra"
)

const developmentVersion = "dev"

// New creates the side-effect-free application command root.
func New(version string, migrations fs.FS) *cobra.Command {
	if strings.TrimSpace(version) == "" {
		version = developmentVersion
	}

	var dataDirectory string
	root := &cobra.Command{
		Use:           "appctl",
		Short:         "Maintain the application while the desktop process is closed",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetVersionTemplate("appctl {{.Version}}\n")
	root.PersistentFlags().StringVar(&dataDirectory, "data-directory", "", "override the application data directory")
	root.AddCommand(newDatabaseCommand(&dataDirectory, migrations))
	return root
}
