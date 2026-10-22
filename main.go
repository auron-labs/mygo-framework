// Package main runs the MyGo framework reference application.
package main

import (
	"context"
	"fmt"
	"log"
	"mygo-framework/app/bootstrap"
	"mygo-framework/app/desktop"
	"path/filepath"

	"github.com/egoist/mygo"
)

func main() {
	applicationRuntime := bootstrap.New()
	mygo.Bind(desktop.NewNotes(applicationRuntime.Readiness(), applicationRuntime.ResolveNotes))

	mygo.App.WhenReady(func() {
		go startRuntime(context.Background(), applicationRuntime)

		mygo.NewWindow(mygo.WindowOptions{
			Title:           "MyGo Framework Notes",
			Width:           960,
			Height:          640,
			MinWidth:        480,
			MinHeight:       360,
			BackgroundColor: "light-dark(#f7f7f5, #171816)",
			StateKey:        "main",
			URL:             "/",
		})
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func startRuntime(ctx context.Context, applicationRuntime *bootstrap.Bootstrap) {
	userData, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		publishPathFailure(applicationRuntime, fmt.Errorf("resolve user data directory: %w", err))
		return
	}
	logDirectory, err := mygo.App.Path(mygo.PathLogs)
	if err != nil {
		publishPathFailure(applicationRuntime, fmt.Errorf("resolve log directory: %w", err))
		return
	}

	_ = applicationRuntime.Start(ctx, bootstrap.Options{
		Development:  mygo.IsDev(),
		DatabasePath: filepath.Join(userData, "notes.db"),
		LogDirectory: logDirectory,
	})
}

func publishPathFailure(applicationRuntime *bootstrap.Bootstrap, cause error) {
	if err := applicationRuntime.Readiness().Fail(cause); err != nil {
		log.Print("runtime startup failure could not be published")
	}
}
