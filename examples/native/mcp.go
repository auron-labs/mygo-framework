package main

import (
	"context"
	"mygo-framework/jobs"

	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func nativeFixture() mygomcp.Fixture {
	return mygomcp.Fixture{
		Name: appName, Width: 640, Height: 640,
		Factory: func() (func(*ui.Context), func(), error) {
			manager := jobs.New(context.Background())
			s := &screen{prefs: preferences{Units: unitsMetric, Theme: themeSystem}, jobs: manager, work: sampleImport}
			cleanup := func() { _ = manager.Shutdown(context.Background()) }
			return func(c *ui.Context) { theme.Apply(c); s.view(c) }, cleanup, nil
		},
	}
}
