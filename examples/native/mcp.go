package main

import (
	"context"
	"mygo-framework/jobs"
	"mygo-framework/state"

	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func nativeFixture() mygomcp.Fixture {
	return mygomcp.Fixture{
		Name: appName, Width: 640, Height: 640,
		Factory: func() (func(*ui.Context), func(), error) {
			manager := jobs.New(context.Background())
			// The harness never persists preferences.
			cell := state.NewCell(preferences{Units: unitsMetric, Theme: themeSystem})
			saver := state.Autosave(cell, saveDelay, func(context.Context, preferences) error { return nil })
			s := &screen{prefs: cell, saver: saver, jobs: manager, work: sampleImport}
			cleanup := func() {
				_ = saver.Close(context.Background())
				_ = manager.Shutdown(context.Background())
			}
			return func(c *ui.Context) { theme.Apply(c); s.view(c) }, cleanup, nil
		},
	}
}
