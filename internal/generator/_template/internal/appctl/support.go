package appctl

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"mygo-framework/app/profile"
	"os"

	"github.com/auron-labs/mygo-framework/diagnostics"

	"github.com/spf13/cobra"
)

func newSupportCommand(dataDirectory *string, version string, migrations fs.FS) *cobra.Command {
	return &cobra.Command{
		Use:   "support-bundle FILE",
		Short: "Write a zip of versions, migration status and logs to attach to a bug report",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withOffline(*dataDirectory, func(paths profile.Paths, _ *slog.Logger) (err error) {
				file, err := os.OpenFile(args[0], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- the operator names the output file.
				if err != nil {
					return fmt.Errorf("create support bundle: %w", err)
				}
				defer func() { err = errors.Join(err, file.Close()) }()
				if err := diagnostics.Write(command.Context(), file, diagnostics.Options{
					Version:      version,
					LogDirectory: paths.LogDirectory,
					DatabasePath: paths.DatabasePath,
					Migrations:   migrations,
				}); err != nil {
					return fmt.Errorf("write support bundle: %w", err)
				}
				if _, err := fmt.Fprintf(command.OutOrStdout(), "support bundle: %s\n", args[0]); err != nil {
					return fmt.Errorf("write support bundle result: %w", err)
				}
				return nil
			})
		},
	}
}
