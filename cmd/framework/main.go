// Command framework manages MyGo framework projects.
package main

import (
	"context"
	"fmt"
	"mygo-framework/internal/cli"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := cli.New(version, cli.NewGenerateCommand(), cli.NewMakeCommand(), cli.NewProjectCommand())
	if err := command.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(command.ErrOrStderr(), "framework: %v\n", err)
		return 1
	}
	return 0
}
