package components_test

import (
	"fmt"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

func ExampleSidebarItemWithLeading() {
	// The application owns what the mark represents; the slot can compose
	// text, images and status decorations without a new component variant.
	mark := func(c *ui.Context, size float32) {
		ui.Box(c).Size(size, size).Shrink(0).Radius(size * .2).Center().
			Background(c.Theme().Accent).Children(func() {
			components.Text(c, "A").FontSize(size * .6).TextColor(c.Theme().AccentText)
		})
	}
	state := components.SettingsState{}
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			components.SidebarItemWithLeading(c, "Workspace", true, mark)
			components.Settings(c, &state, []components.SettingsCategory{{
				ID: "workspace", Title: "Workspace settings", Leading: mark,
			}}, components.SettingsOptions{})
		})
	}, 800, 500)
	fmt.Println(tester.HasText("A"))
	// Output: true
}
