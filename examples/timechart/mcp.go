package main

import (
	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func fixture() mygomcp.Fixture {
	return mygomcp.Fixture{
		Name: appName, Width: 1000, Height: 760,
		Factory: func() (func(*ui.Context), func(), error) {
			s := newScreen(newRecording())
			return func(c *ui.Context) { theme.Apply(c); s.view(c) }, s.close, nil
		},
	}
}
