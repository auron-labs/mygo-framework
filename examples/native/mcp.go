package main

import (
	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func nativeFixture() mygomcp.Fixture {
	return mygomcp.Fixture{
		Name: appName, Width: 640, Height: 420,
		Factory: func() (func(*ui.Context), func(), error) {
			s := &screen{prefs: preferences{Units: unitsMetric, Theme: themeSystem}}
			return func(c *ui.Context) { theme.Apply(c); s.view(c) }, nil, nil
		},
	}
}
