package components_test

import (
	"fmt"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

func ExampleSettings() {
	// Retain state and values in the application, not inside the frame builder.
	state := components.SettingsState{}
	enabled := false
	categories := []components.SettingsCategory{{
		ID: "notifications", Title: "Notifications",
		Groups: []components.SettingsGroup{{ID: "delivery", Title: "Delivery", Actions: func(c *ui.Context) {
			// Group actions sit at the end of the group's header row.
			components.Button(c, "Reset", components.ButtonGhost).OnClick(func() { enabled = false })
		}, Items: []components.SettingsItem{{
			ID: "desktop", Title: "Desktop notifications", Keywords: []string{"alerts"},
			Build: func(c *ui.Context) { components.Switch(c, &enabled, "Enable desktop notifications") },
		}, {
			// Content builds the whole row; its title still matches searches.
			ID: "preview", Title: "Preview", Keywords: []string{"sample"},
			Content: func(c *ui.Context) { components.Text(c, "A notification preview").Wrap() },
		}}}},
	}}
	view := func(c *ui.Context) {
		// The host decides narrow layout from the width allocated to this surface.
		components.Settings(c, &state, categories, components.SettingsOptions{Narrow: true})
	}
	// Use view as window content or inside an application-owned Dialog.
	tester := ui.NewTester(view, 400, 500)
	tester.Frame()
	fmt.Println(tester.HasText("Desktop notifications"))
	// Output: true
}

func ExampleSearchSettings() {
	categories := []components.SettingsCategory{{ID: "appearance", Title: "Appearance", Groups: []components.SettingsGroup{{ID: "theme", Items: []components.SettingsItem{{ID: "mode", Title: "Color mode", Keywords: []string{"dark"}}}}}}}
	for _, result := range components.SearchSettings(categories, "dark appearance") {
		fmt.Println(categories[result.Category].Groups[result.Group].Items[result.Item].Title)
	}
	// Output: Color mode
}
