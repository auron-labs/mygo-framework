// Command framework manages MyGo framework projects.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/auron-labs/mygo-framework/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := cli.New(
		version,
		cli.NewBuildCommand(),
		cli.NewDevCommand(),
		cli.NewDoctorCommand(),
		cli.NewGenerateCommand(),
		cli.NewMakeCommand(),
		cli.NewProjectCommand(),
	)
	if err := command.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(command.ErrOrStderr(), "framework: %v\n", err)
		return 1
	}
	return 0
}
