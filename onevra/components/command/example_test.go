package command_test

import (
	"runtime"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/components/command"
	"github.com/egoist/mygo/ui"
)

func ExampleCommandDialog() {
	var state command.State
	var open bool
	var destination string
	inputSource := ""
	if runtime.GOOS == "darwin" {
		inputSource = command.InputSourceABC
	}
	items := []command.Item{{ID: "settings", Title: "Settings", Keywords: []string{"preferences"}}}
	view := func(c *ui.Context) {
		if c.Shortcut(ui.Cmd, ui.KeyF) {
			open = true
		}
		command.CommandDialog(c, &open, &state, items, command.Options{
			// Omit this to show results immediately, even with an empty query.
			InputSource:    inputSource,
			ExpandOnSearch: true,
			OnActivate:     func(id string) { destination = id; open = false },
		}, components.DialogOptions{})
	}
	// Also call state.ReleaseInputSource on the UI thread before destroying
	// the host window. Normal dialog dismissal restores the source automatically.
	_ = view // Supply to a MyGo window or Onevra Shell content builder.
	_ = destination
}

func ExampleCommandView() {
	var state command.State
	items := []command.Item{{ID: "settings", Title: "Settings"}}
	view := func(c *ui.Context) {
		// A bounded parent, such as a panel window's content, sizes the view.
		ui.Column(c).Fill().Children(func() {
			command.CommandView(c, &state, items, command.Options{
				OnActivate: func(id string) { /* Run id; hide the window if needed. */ },
			})
		})
	}
	// After showing a hidden host window, call state.Focus so typing reaches
	// search. Embedded hosts call state.ReleaseInputSource when removing the view.
	_ = view
}
