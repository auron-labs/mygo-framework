package treemap_test

import (
	"fmt"

	"github.com/auron-labs/mygo-framework/v2/onevra/components/treemap"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func ExampleView() {
	state := treemap.State{}
	usage := []treemap.Node{
		{ID: "photos", Label: "Photos", Children: []treemap.Node{
			{ID: "photos/2025", Label: "2025", Value: 962},
			{ID: "photos/2026", Label: "2026", Value: 82},
		}},
		{ID: "music", Label: "Music", Value: 640},
	}
	view := func(c *ui.Context) {
		theme.Apply(c)
		treemap.View(c, &state, usage, treemap.Options{
			Label:       "Disk usage",
			FormatValue: func(v float64) string { return fmt.Sprintf("%g MB", v) },
		})
	}
	// Supply view to a Onevra Shell, or run it without a window for interaction.
	tt := ui.NewTester(view, 480, 320)
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyEnter)
	fmt.Println(state.Root, state.Active)
	// Output: photos photos/2025
}
