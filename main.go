// Package main runs the MyGo framework reference application.
package main

import (
	"context"
	"fmt"
	"log"
	"mygo-framework/app/bootstrap"
	"mygo-framework/app/desktop"
	"mygo-framework/app/profile"
	"os"
	"time"

	"github.com/egoist/mygo"
)

func main() {
	mygo.App.SetName(profile.Name)
	applicationRuntime := bootstrap.New()
	applicationContext, cancelApplication := context.WithCancel(context.Background())
	bindGenerated(applicationRuntime)
	quitStage := desktop.NewQuitStage(
		cancelApplication,
		applicationRuntime.Shutdown,
		mygo.App.Quit,
		func(err error) { log.Printf("runtime shutdown failed: %v", err) },
		5*time.Second,
	)
	mygo.App.OnBeforeQuit(quitStage.BeforeQuit)

	mygo.App.WhenReady(func() {
		go startRuntime(applicationContext, applicationRuntime)

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
	paths, err := profile.Resolve("")
	if err != nil {
		publishPathFailure(applicationRuntime, fmt.Errorf("resolve application profile: %w", err))
		return
	}
	if err := os.MkdirAll(paths.DataDirectory, 0o700); err != nil {
		publishPathFailure(applicationRuntime, fmt.Errorf("create application data directory: %w", err))
		return
	}

	_ = applicationRuntime.Start(ctx, bootstrap.Options{
		Development:  mygo.IsDev(),
		DatabasePath: paths.DatabasePath,
		LogDirectory: paths.LogDirectory,
	})
}

func publishPathFailure(applicationRuntime *bootstrap.Bootstrap, cause error) {
	if err := applicationRuntime.Readiness().Fail(cause); err != nil {
		log.Print("runtime startup failure could not be published")
	}
}
