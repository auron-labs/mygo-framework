package appctl

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"mygo-framework/app/profile"
	appdb "mygo-framework/internal/database"
	"mygo-framework/logging"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newDatabaseCommand(dataDirectory *string, migrations fs.FS) *cobra.Command {
	databaseCommand := &cobra.Command{
		Use:   "db",
		Short: "Inspect or apply database migrations offline",
		Args:  cobra.NoArgs,
	}
	databaseCommand.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Show applied and pending migrations without changing the database",
			Args:  cobra.NoArgs,
			RunE: func(command *cobra.Command, _ []string) error {
				return withOffline(*dataDirectory, func(paths profile.Paths, logger *slog.Logger) error {
					statuses, exists, err := appdb.InspectMigrations(command.Context(), paths.DatabasePath, migrations)
					if err != nil {
						return fmt.Errorf("inspect database migrations: %w", err)
					}
					logger.DebugContext(command.Context(), "inspected database migrations",
						slog.String("operation", "db.status"),
						slog.Int("migration_count", len(statuses)),
					)

					state := "exists"
					if !exists {
						state = "not created"
					}
					if _, err := fmt.Fprintf(command.OutOrStdout(), "database: %s (%s)\n", paths.DatabasePath, state); err != nil {
						return fmt.Errorf("write database status: %w", err)
					}
					for _, status := range statuses {
						state := "pending"
						if status.Applied {
							state = "applied"
						}
						if _, err := fmt.Fprintf(command.OutOrStdout(), "%05d  %-7s  %s\n",
							status.Version, state, filepath.Base(status.Path),
						); err != nil {
							return fmt.Errorf("write migration status: %w", err)
						}
					}
					return nil
				})
			},
		},
		&cobra.Command{
			Use:   "up",
			Short: "Apply all pending migrations",
			Args:  cobra.NoArgs,
			RunE: func(command *cobra.Command, _ []string) error {
				return withOffline(*dataDirectory, func(paths profile.Paths, logger *slog.Logger) error {
					if err := os.MkdirAll(paths.DataDirectory, 0o700); err != nil {
						return fmt.Errorf("create data directory: %w", err)
					}
					applied, err := appdb.Migrate(command.Context(), paths.DatabasePath, migrations)
					if err != nil {
						return fmt.Errorf("apply database migrations: %w", err)
					}
					logger.DebugContext(command.Context(), "applied database migrations",
						slog.String("operation", "db.up"),
						slog.Int("applied_count", applied),
					)
					if _, err := fmt.Fprintf(command.OutOrStdout(), "database: %s\napplied: %d\n", paths.DatabasePath, applied); err != nil {
						return fmt.Errorf("write migration result: %w", err)
					}
					return nil
				})
			},
		},
	)
	return databaseCommand
}

func withOffline(dataDirectory string, run func(profile.Paths, *slog.Logger) error) (err error) {
	paths, err := profile.Resolve(dataDirectory)
	if err != nil {
		return fmt.Errorf("resolve application profile: %w", err)
	}
	logger, closer, err := logging.New(logging.DefaultOptions(false, paths.LogDirectory))
	if err != nil {
		return fmt.Errorf("open application logs: %w", err)
	}
	defer func() {
		if closeErr := closer.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close application logs: %w", closeErr))
		}
	}()

	return run(paths, logger)
}
