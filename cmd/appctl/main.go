// Command appctl maintains the application while its desktop process is closed.
package main

import (
	"context"
	"fmt"
	"mygo-framework/database/migrations"
	"mygo-framework/internal/appctl"
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

	command := appctl.New(version, migrations.FS())
	if err := command.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(command.ErrOrStderr(), "appctl: %v\n", err)
		return 1
	}
	return 0
}
