package chart_test

import (
	"fmt"

	"github.com/auron-labs/mygo-framework/v2/onevra/components/chart"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func ExampleLine() {
	state := chart.State{Index: -1}
	data := chart.Data{
		Labels: []string{"Mon", "Tue", "Wed"},
		Series: []chart.Series{{ID: "hours", Label: "Hours", Values: []float64{4, 6, 5}}},
	}
	view := func(c *ui.Context) {
		theme.Apply(c)
		chart.Line(c, &state, data, chart.Options{
			Label: "Daily hours", Description: "Use arrow keys to explore", EmptyText: "No data",
			FormatValue: func(v float64) string { return fmt.Sprintf("%g h", v) },
		})
	}
	// Supply view to a Onevra Shell, or run it without a window for interaction.
	tt := ui.NewTester(view, 420, 360)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyRight)
	fmt.Println(state.Index)
	// Output: 1
}
