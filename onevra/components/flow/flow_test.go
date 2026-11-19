package flow

import (
	"math"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/components"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func sample() Data {
	card := func(title string) func(*ui.Context) {
		return func(c *ui.Context) { Card(c, CardOptions{Title: title, Description: title + " details"}) }
	}
	return Data{
		Nodes: []Node{
			{ID: "plan", Label: "Plan", Content: card("Plan")},
			{ID: "build", Label: "Build", Content: card("Build")},
			{ID: "test", Label: "Test", Content: card("Test")},
			{ID: "ship", Label: "Ship", Muted: true, Content: card("Ship")},
			{ID: "fixed", Label: "Fixed", Width: 90, Height: 40},
		},
		Edges: []Edge{
			{Source: "plan", Target: "build"},
			{Source: "plan", Target: "test", Dashed: true, Label: "depends on"},
			{Source: "build", Target: "ship"},
			{Source: "test", Target: "ship"},
			{Source: "ship", Target: "ship"},
			{Source: "ship", Target: "missing"},
		},
	}
}

type harness struct {
	tt               *ui.Tester
	state            State
	data             Data
	opts             Options
	selected, opened []string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{data: sample()}
	h.opts = Options{
		Label:  "Pipeline",
		Select: func(id string) { h.selected = append(h.selected, id); h.opts.Selected = id },
		Open:   func(id string) { h.opened = append(h.opened, id) },
	}
	h.tt = ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		View(c, &h.state, h.data, h.opts)
	}, 900, 700)
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	h.settle()
	if !h.state.Ready() {
		t.Fatal("diagram did not become ready")
	}
	return h
}

// settle builds a frame for changed data and lets measurement and motion
// finish.
func (h *harness) settle() {
	h.tt.Frame()
	for range 4 {
		h.tt.Advance(20 * time.Millisecond)
	}
}

func TestViewMeasuresAndLaysOut(t *testing.T) {
	h := newHarness(t)
	v := &h.state.view
	if len(v.nodes) != 5 || len(v.edges) != 4 {
		t.Fatalf("view kept %d nodes and %d edges", len(v.nodes), len(v.edges))
	}
	for _, n := range v.nodes {
		if !n.measured {
			t.Fatalf("node %s is unmeasured", n.id)
		}
	}
	if n := v.nodes[v.index["plan"]]; n.width != 240 || n.height <= 0 {
		t.Fatalf("card measured %vx%v", n.width, n.height)
	}
	if n := v.nodes[v.index["fixed"]]; n.width != 90 || n.height != 40 {
		t.Fatalf("fixed node measured %vx%v", n.width, n.height)
	}
	_, planY, _ := h.state.Position("plan")
	_, buildY, _ := h.state.Position("build")
	_, shipY, _ := h.state.Position("ship")
	if !(planY < buildY && buildY < shipY) {
		t.Fatalf("ranks at %v, %v, %v", planY, buildY, shipY)
	}
	if _, _, ok := h.state.Position("missing"); ok {
		t.Fatal("unknown node is laid out")
	}
	// The fitted diagram shows whole, at no more than its natural size.
	if z := h.state.Zoom(); z <= 0 || z > 1 {
		t.Fatalf("fitted zoom %v", z)
	}
	r, ok := h.tt.Find("Plan")
	if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 900 || r.Y+r.H > 700 {
		t.Fatalf("plan shows at %v", r)
	}
	if _, ok := h.tt.Find("Pipeline"); !ok {
		t.Fatal("diagram has no accessible label")
	}
}

func TestCardSlots(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		Card(c, CardOptions{
			Title:    "Replaced title",
			Header:   func(c *ui.Context) { components.Text(c, "Status") },
			Leading:  func(c *ui.Context) { components.Text(c, "Icon") },
			Body:     func(c *ui.Context) { components.Text(c, "Custom body") },
			Trailing: func(c *ui.Context) { components.Text(c, "Badge") },
			Footer:   func(c *ui.Context) { components.Text(c, "Counts") },
		})
	}, 400, 300)
	tt.Frame()
	if _, ok := tt.Find("Replaced title"); ok {
		t.Fatal("title shows beside a body slot")
	}
	var boxes []ui.Rect
	for _, s := range []string{"Status", "Icon", "Custom body", "Badge", "Counts"} {
		r, ok := tt.Find(s)
		if !ok {
			t.Fatalf("slot %q does not show", s)
		}
		boxes = append(boxes, r)
	}
	if !(boxes[0].Y < boxes[2].Y && boxes[2].Y < boxes[4].Y) || !(boxes[1].X < boxes[2].X && boxes[2].X < boxes[3].X) {
		t.Fatalf("slots out of place: %v", boxes)
	}
}

func TestViewSelectsAndOpens(t *testing.T) {
	h := newHarness(t)
	if err := h.tt.Click("Build"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if len(h.selected) != 1 || h.selected[0] != "build" {
		t.Fatalf("selected %v", h.selected)
	}
	if !h.state.emphasized(h.state.edges[0]) || h.state.emphasized(h.state.edges[1]) {
		t.Fatal("selection does not emphasize its edges alone")
	}
	r, _ := h.tt.Find("Test")
	h.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	h.tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	h.settle()
	if len(h.opened) != 1 || h.opened[0] != "test" {
		t.Fatalf("opened %v", h.opened)
	}
	// Clicking empty space clears the selection without opening anything.
	h.tt.ClickAt(4, 690)
	h.settle()
	if last := h.selected[len(h.selected)-1]; last != "" || len(h.opened) != 1 {
		t.Fatalf("empty click selected %q and opened %v", last, h.opened)
	}
}

func TestViewPansAndZooms(t *testing.T) {
	h := newHarness(t)
	v := &h.state.view
	before := v.shown
	h.tt.Press(4, 690)
	h.tt.Move(40, 650)
	h.tt.Move(64, 640)
	h.tt.Release(64, 640)
	h.settle()
	if v.shown.x-before.x != 60 || v.shown.y-before.y != -50 {
		t.Fatalf("pan moved the camera from %v to %v", before, v.shown)
	}
	if len(h.selected) != 0 {
		t.Fatal("panning selected")
	}
	zoom := h.state.Zoom()
	h.state.ZoomIn()
	h.settle()
	if h.state.Zoom() <= zoom {
		t.Fatal("zoom in did not zoom")
	}
	h.state.Fit()
	h.settle()
	if h.state.Zoom() != zoom || v.userMoved {
		t.Fatalf("fit zoom %v, want %v", h.state.Zoom(), zoom)
	}
}

func TestViewRelaysOutWhenDataChanges(t *testing.T) {
	h := newHarness(t)
	h.data.Nodes = append(h.data.Nodes, Node{ID: "docs", Label: "Docs", Content: func(c *ui.Context) { Card(c, CardOptions{Title: "Docs"}) }})
	h.data.Edges = append(h.data.Edges, Edge{Source: "ship", Target: "docs"})
	h.settle()
	_, shipY, _ := h.state.Position("ship")
	_, docsY, ok := h.state.Position("docs")
	if !ok || docsY <= shipY {
		t.Fatalf("new node at %v below %v", docsY, shipY)
	}
	if !h.state.view.nodes[h.state.index["docs"]].measured {
		t.Fatal("new node is unmeasured")
	}
	// Content that grows is measured and laid out again.
	h.data.Nodes[0].Content = func(c *ui.Context) { Card(c, CardOptions{Width: 320, Title: "Plan"}) }
	h.settle()
	if n := h.state.view.nodes[0]; n.width != 320 {
		t.Fatalf("resized card measured %v wide", n.width)
	}
	if x, _, _ := h.state.Position("plan"); h.state.view.result.width < float64(x)+320 {
		t.Fatal("layout did not follow the resized card")
	}
	h.data = Data{}
	h.opts.EmptyText = "Nothing planned"
	h.settle()
	if !h.tt.HasText("Nothing planned") {
		t.Fatal("empty state is missing")
	}
}

func TestViewShownAgainKeepsLayout(t *testing.T) {
	var state State
	data := sample()
	shown, collapsed := true, false
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		if !shown {
			return
		}
		View(c, &state, data, Options{Label: "Pipeline"})
		for _, n := range state.view.nodes {
			collapsed = collapsed || (state.view.ready && n.width == 0)
		}
	}, 900, 700)
	camera := state.view.camera
	shown = false
	tt.Frame()
	shown = true
	tt.Frame()
	if collapsed || state.view.camera != camera || state.view.shown != camera {
		t.Fatalf("diagram shown again moved: collapsed %v, camera %+v from %+v", collapsed, state.view.camera, camera)
	}
}

func TestViewPlacesSettlingContentAtOnce(t *testing.T) {
	var state State
	data := sample()
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		View(c, &state, data, Options{Label: "Pipeline"})
	}, 900, 700)
	v := &state.view
	moved := func() bool {
		for _, n := range v.nodes {
			x, y, _ := state.Position(n.id)
			if math.Abs(n.x-float64(x)) > .01 || math.Abs(n.y-float64(y)) > .01 {
				return true
			}
		}
		return v.shown != v.camera
	}
	// Content that settles to a new size places the diagram at once.
	data.Nodes[0].Content = func(c *ui.Context) { Card(c, CardOptions{Width: 320, Title: "Plan"}) }
	tt.Frame()
	if moved() {
		t.Fatal("resized content animated the layout")
	}
	// A new topology still moves the nodes.
	data.Nodes = append(data.Nodes, Node{ID: "docs", Label: "Docs", Width: 120, Height: 40})
	data.Edges = append(data.Edges, Edge{Source: "plan", Target: "docs"})
	tt.Frame()
	if !moved() {
		t.Fatal("new topology did not animate")
	}
}
