// Package main runs the MyGo framework reference application.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/auron-labs/mygo-framework/app/bootstrap"
	"github.com/auron-labs/mygo-framework/app/desktop"
	appMCP "github.com/auron-labs/mygo-framework/app/mcp"
	"github.com/auron-labs/mygo-framework/app/profile"
	"github.com/auron-labs/mygo-framework/errorreport"

	"github.com/egoist/mygo"
)

const (
	mcpEnabledEnvironment  = "MYGO_MCP_ENABLED"
	mcpPortEnvironment     = "MYGO_MCP_PORT"
	mcpDiscloseEnvironment = "MYGO_MCP_DISCLOSE"
)

func main() {
	if err := errorreport.Init(mygo.App.Version()); err != nil {
		log.Print(err)
	}
	if err := runApp(); err != nil {
		log.Fatal(err)
	}
}

func runApp() error {
	defer errorreport.Flush()
	defer errorreport.RecoverAndRepanic()
	// A second launch would open the same database; the running instance takes over.
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	mygo.App.SetName(profile.Name)
	applicationRuntime := bootstrap.New()
	applicationContext, cancelApplication := context.WithCancel(context.Background())
	bindGenerated(applicationRuntime)
	mygo.Bind(desktop.NewErrorReporting())
	quitStage := desktop.NewQuitStage(
		cancelApplication,
		applicationRuntime.Shutdown,
		mygo.App.Quit,
		func(err error) { log.Printf("runtime shutdown failed: %v", err) },
		5*time.Second,
	)
	mygo.App.OnBeforeQuit(quitStage.BeforeQuit)

	var window *mygo.Window
	mygo.App.OnSecondInstance(func([]string, string) {
		if window != nil {
			window.Restore()
			window.Focus()
		}
	})
	mygo.App.WhenReady(func() {
		go startRuntime(applicationContext, applicationRuntime)

		window = mygo.NewWindow(mygo.WindowOptions{
			Title:           "MyGo Framework Notes",
			Width:           960,
			Height:          640,
			MinWidth:        480,
			MinHeight:       360,
			BackgroundColor: "light-dark(#f7f7f5, #171816)",
			StateKey:        "main",
			URL:             "/",
			Page:            mygo.PageOptions{PinchZoom: true},
		})
	})

	if err := mygo.App.Run(); err != nil {
		errorreport.Capture(err, "app.run", "runtime")
		return fmt.Errorf("run application: %w", err)
	}
	return nil
}

func startRuntime(ctx context.Context, applicationRuntime *bootstrap.Bootstrap) {
	mcpOptions, err := mcpOptionsFromEnvironment(os.LookupEnv)
	if err != nil {
		publishStartupFailure(applicationRuntime, err)
		return
	}
	discloseMCP, err := mcpDisclosureFromEnvironment(os.LookupEnv, mcpOptions.Enabled)
	if err != nil {
		publishStartupFailure(applicationRuntime, err)
		return
	}
	paths, err := profile.Resolve("")
	if err != nil {
		publishStartupFailure(applicationRuntime, fmt.Errorf("resolve application profile: %w", err))
		return
	}
	if err := os.MkdirAll(paths.DataDirectory, 0o700); err != nil {
		publishStartupFailure(applicationRuntime, fmt.Errorf("create application data directory: %w", err))
		return
	}

	if err := applicationRuntime.Start(ctx, bootstrap.Options{
		Development:  mygo.IsDev(),
		DatabasePath: paths.DatabasePath,
		LogDirectory: paths.LogDirectory,
		MCP:          mcpOptions,
	}); err != nil || !discloseMCP {
		return
	}
	details, err := applicationRuntime.MCPConnectionDetails(ctx)
	if err != nil {
		log.Printf("MCP connection disclosure failed: %v", err)
		return
	}
	if err := writeMCPConnectionDetails(os.Stdout, details); err != nil {
		log.Print("MCP connection disclosure output failed")
	}
}

func mcpOptionsFromEnvironment(lookup func(string) (string, bool)) (appMCP.Options, error) {
	if lookup == nil {
		return appMCP.Options{}, errors.New("resolve MCP environment: lookup is required")
	}
	enabledValue, exists := lookup(mcpEnabledEnvironment)
	if !exists || strings.TrimSpace(enabledValue) == "" {
		return appMCP.Options{}, nil
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(enabledValue))
	if err != nil {
		return appMCP.Options{}, fmt.Errorf("parse %s: %w", mcpEnabledEnvironment, err)
	}
	if !enabled {
		return appMCP.Options{}, nil
	}
	portValue, exists := lookup(mcpPortEnvironment)
	if !exists || strings.TrimSpace(portValue) == "" {
		return appMCP.Options{}, fmt.Errorf("%s is required when %s is enabled", mcpPortEnvironment, mcpEnabledEnvironment)
	}
	port, err := strconv.ParseUint(strings.TrimSpace(portValue), 10, 16)
	if err != nil || port == 0 {
		return appMCP.Options{}, fmt.Errorf("parse %s: expected a port from 1 to 65535", mcpPortEnvironment)
	}
	return appMCP.Options{Enabled: true, Port: int(port)}, nil
}

func mcpDisclosureFromEnvironment(lookup func(string) (string, bool), enabled bool) (bool, error) {
	if lookup == nil {
		return false, errors.New("resolve MCP disclosure environment: lookup is required")
	}
	value, exists := lookup(mcpDiscloseEnvironment)
	if !exists || strings.TrimSpace(value) == "" {
		return false, nil
	}
	disclose, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", mcpDiscloseEnvironment, err)
	}
	if disclose && !enabled {
		return false, fmt.Errorf("%s requires %s", mcpDiscloseEnvironment, mcpEnabledEnvironment)
	}
	return disclose, nil
}

func writeMCPConnectionDetails(writer io.Writer, details appMCP.ConnectionDetails) error {
	if _, err := fmt.Fprintf(
		writer,
		"MCP endpoint: %s\nMCP authorization: Bearer %s\n",
		details.Endpoint,
		details.BearerToken,
	); err != nil {
		return fmt.Errorf("write MCP connection details: %w", err)
	}
	return nil
}

func publishStartupFailure(applicationRuntime *bootstrap.Bootstrap, cause error) {
	errorreport.Capture(cause, "runtime.start", "runtime")
	if err := applicationRuntime.Readiness().Fail(cause); err != nil {
		log.Print("runtime startup failure could not be published")
	}
}
