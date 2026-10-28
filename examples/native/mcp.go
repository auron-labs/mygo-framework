package main

import (
	mygomcp "github.com/auron-labs/mygo-mcp"
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

func nativeFixture() mygomcp.Fixture {
	return mygomcp.Fixture{
		Name: appName, Width: 640, Height: 360,
		Factory: func() (func(*ui.Context), func(), error) {
			s := &screen{}
			return func(c *ui.Context) { theme.Apply(c); s.view(c) }, nil, nil
		},
	}
}
