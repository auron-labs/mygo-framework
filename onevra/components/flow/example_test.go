package flow_test

import (
	"fmt"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/components/flow"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func ExampleView() {
	var state flow.State
	selected := ""
	card := func(title, detail string) func(*ui.Context) {
		return func(c *ui.Context) { flow.Card(c, flow.CardOptions{Title: title, Description: detail}) }
	}
	data := flow.Data{
		Nodes: []flow.Node{
			{ID: "survey", Label: "Survey", Content: card("Survey", "Collect requirements")},
			{ID: "design", Label: "Design", Content: card("Design", "Sketch the approach")},
			{ID: "prototype", Label: "Prototype", Content: card("Prototype", "Try the riskiest part")},
			{ID: "review", Label: "Review", Content: card("Review", "Decide what ships")},
		},
		Edges: []flow.Edge{
			{Source: "survey", Target: "design"},
			{Source: "survey", Target: "prototype", Dashed: true, Label: "informs"},
			{Source: "design", Target: "review"},
			{Source: "prototype", Target: "review"},
		},
	}
	view := func(c *ui.Context) {
		theme.Apply(c)
		flow.View(c, &state, data, flow.Options{
			Label: "Project plan", Height: 480, Selected: selected,
			Select: func(id string) { selected = id },
		})
	}
	// Supply view to a Onevra Shell, or run it without a window.
	tt := ui.NewTester(view, 720, 480)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	// Nodes are measured on the frame after they are first built.
	tt.Advance(50 * time.Millisecond)
	_ = tt.Click("Design")
	_, survey, _ := state.Position("survey")
	_, review, _ := state.Position("review")
	fmt.Println(state.Ready(), selected, survey < review)
	// Output: true design true
}
