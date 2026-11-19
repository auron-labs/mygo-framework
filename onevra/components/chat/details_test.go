package chat

import (
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/components"
	"github.com/egoist/mygo/ui"
)

func TestIndependentLazyActivityDetails(t *testing.T) {
	var a, b ActivityState
	builds := [2]int{}
	var turn TurnState
	g := &GroupState{}
	opts := TurnOptions{ID: "turn", Status: Running, Steps: []Step{{ID: "g", Title: "Ran 2 tools", Group: g, Activities: []Activity{
		{ID: "a", Action: "Read", Detail: "a.go", State: &a, Content: func(c *ui.Context) { builds[0]++; components.Text(c, "Read output") }},
		{ID: "b", Action: "Run", Detail: "ls", State: &b, MaxHeight: 100, Content: func(c *ui.Context) { builds[1]++; ui.Box(c).Height(900).Shrink(0) }},
	}}}}
	tt := ui.NewTester(func(c *ui.Context) { Turn(c, &turn, opts) }, 400, 400)
	if builds != [2]int{} {
		t.Fatal("closed activity content built")
	}
	if err := tt.Click("Run ls"); err != nil {
		t.Fatal(err)
	}
	if a.Expanded || !b.Expanded || builds[0] != 0 || b.Scroll.MaxY <= 0 {
		t.Fatal("independent bounded activity view failed")
	}
	// A settled group closes automatically but keeps activity state.
	opts.Status = Failed
	builds = [2]int{}
	tt.Frame()
	if builds != [2]int{} || tt.HasText("a.go") {
		t.Fatal("settled group stayed open")
	}
	if err := tt.Click("Ran 2 tools"); err != nil {
		t.Fatal(err)
	}
	if !b.Expanded || builds[1] == 0 {
		t.Fatal("group fold discarded activity state")
	}
}

func TestResultSummaryLimitsAndActions(t *testing.T) {
	var state SummaryState
	items := make([]SummaryItem, 1000)
	for i := range items {
		items[i] = SummaryItem{ID: fmt.Sprint(i), Label: fmt.Sprintf("Artifact %d", i)}
	}
	reviews := 0
	selected := ""
	tt := ui.NewTester(func(c *ui.Context) {
		p := ResultSummary(c, &state, SummaryOptions{Title: "Results", ActionLabel: "Review", MoreLabel: "Show first 100", LessLabel: "Less", Items: items})
		if p.Action {
			reviews++
		}
		if p.SelectedID != "" {
			selected = p.SelectedID
		}
	}, 400, 5000)
	if tt.HasText("Artifact 3") {
		t.Fatal("preview exceeds three items")
	}
	_ = tt.Click("Artifact 1")
	_ = tt.Click("Review")
	if selected != "1" || reviews != 1 {
		t.Fatal("result actions missing")
	}
	_ = tt.Click("Show first 100")
	if !tt.HasText("Artifact 99") || tt.HasText("Artifact 100") {
		t.Fatal("expanded result cap failed")
	}
}

func TestTurnFooterSurvivesFoldWithoutAnswer(t *testing.T) {
	var state TurnState
	state.SetExpanded(false)
	tt := ui.NewTester(func(c *ui.Context) {
		Turn(c, &state, TurnOptions{
			ID: "turn", Summary: "Stopped", Status: Cancelled,
			Steps:   []Step{{ID: "tool", Message: func(*ui.Context) { t.Fatal("hidden tool built") }}},
			Footer:  func(c *ui.Context) { components.Text(c, "Artifacts") },
			Actions: func(c *ui.Context) { components.Button(c, "Retry", components.ButtonGhost) },
		})
	}, 400, 300)
	if !tt.HasText("Artifacts") || !tt.HasText("Retry") {
		t.Fatal("fold hid footer")
	}
}
