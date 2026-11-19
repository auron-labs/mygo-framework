package graph

import (
	"math"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/components/gesture"
	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

var sample = Data{
	Nodes: []Node{
		{ID: "home", Label: "Home", Openable: true},
		{ID: "a", Label: "Alpha", Group: "projects", Openable: true},
		{ID: "b", Label: "Beta", Group: "projects", Openable: true},
		{ID: "c", Label: "Gamma", Group: "areas", Openable: true},
		{ID: "file", Label: "diagram.png", Group: "projects", Size: 8, Color: ui.Hex("#e0b35a")},
		{ID: "lonely", Label: "Lonely", Openable: true},
	},
	Edges: []Edge{
		{Source: "home", Target: "a", Style: EdgeStyle{Distance: 160}},
		{Source: "home", Target: "c", Style: EdgeStyle{Distance: 160}},
		{Source: "a", Target: "b", Style: EdgeStyle{Distance: 200, HoverOpacity: .35}},
		{Source: "a", Target: "file", Style: EdgeStyle{Distance: 64, Opacity: .13}},
		{Source: "a", Target: "missing"},
		{Source: "b", Target: "b"},
	},
}

type harness struct {
	t      *testing.T
	tt     *ui.Tester
	state  *State
	data   Data
	opts   Options
	opened []string
}

func newHarness(t *testing.T) *harness {
	h := &harness{t: t, state: &State{}, data: sample}
	h.opts = Options{Label: "Graph", Height: 400, Open: func(id string) { h.opened = append(h.opened, id) }}
	h.tt = ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		View(c, h.state, h.data, h.opts)
	}, 600, 400)
	// Reduced Motion settles the layout and camera synchronously.
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	h.tt.Frame()
	// The layout settles on the next painted frame, which the tester's
	// clock must reach.
	for i := 0; i < 100 && !(h.state.ready && h.state.Settled()) && h.tt.FramePending(); i++ {
		h.tt.Advance(20 * time.Millisecond)
	}
	if !h.state.ready || !h.state.Settled() {
		t.Fatal("graph did not settle under Reduced Motion")
	}
	return h
}

func (h *harness) canvas() ui.Rect {
	r, ok := h.tt.Find("Graph")
	if !ok {
		h.t.Fatal("graph canvas is not labelled")
	}
	return r
}

func (h *harness) point(id string) (float32, float32) {
	i := h.state.indexOf(id)
	if i < 0 {
		h.t.Fatalf("node %q is missing", id)
	}
	r := h.canvas()
	x, y := h.state.screen(i)
	return r.X + float32(x), r.Y + float32(y)
}

// doubleClick sends the second press of a double-click as platforms report
// it; Tester clicks always count one.
func (h *harness) doubleClick(x, y float32, mods ui.Modifiers) {
	r := h.canvas()
	ex, ey := x-r.X, y-r.Y
	h.state.input(ui.InputEvent{Kind: ui.InputPointerDown, X: ex, Y: ey, Clicks: 2, Mods: mods}, h.opts.Open)
	h.state.input(ui.InputEvent{Kind: ui.InputPointerUp, X: ex, Y: ey, Clicks: 2, Mods: mods}, h.opts.Open)
	h.tt.Frame()
}

func TestViewFitsAndFiltersData(t *testing.T) {
	h := newHarness(t)
	if len(h.state.nodes) != 6 || len(h.state.edges) != 4 {
		t.Fatalf("kept %d nodes, %d edges; want 6, 4", len(h.state.nodes), len(h.state.edges))
	}
	r := h.canvas()
	for _, id := range h.state.ids {
		x, y := h.point(id)
		if x < r.X || x > r.X+r.W || y < r.Y || y > r.Y+r.H {
			t.Fatalf("%s at (%g,%g) lies outside the fitted view %+v", id, x, y, r)
		}
	}
	file := h.state.nodes[h.state.indexOf("file")]
	if file.diameter != 8 || file.fill != ui.Hex("#e0b35a") {
		t.Fatalf("node overrides ignored: %+v", file)
	}
	lonely := h.state.nodes[h.state.indexOf("lonely")]
	if lonely.fill != DefaultPalette(false).Isolated {
		t.Fatal("isolated node does not use the isolated color")
	}
	a, c := h.state.nodes[h.state.indexOf("a")], h.state.nodes[h.state.indexOf("c")]
	if a.fill == c.fill || a.fill == DefaultPalette(false).Node {
		t.Fatal("groups do not receive distinct colors")
	}
	if e := h.state.edges[3]; e.opacity != .13 || e.color != a.fill || e.hover != hoverEdgePeakOpacity {
		t.Fatalf("edge style = %+v", e)
	}
}

func TestHoverOpenAndCursor(t *testing.T) {
	h := newHarness(t)
	x, y := h.point("a")
	h.tt.Move(x, y)
	if h.state.Hovered() != "a" {
		t.Fatalf("hovered = %q", h.state.Hovered())
	}
	if h.tt.Cursor() != ui.CursorPointer {
		t.Fatalf("cursor over openable node = %v", h.tt.Cursor())
	}
	depths := h.state.hoverDepths
	if len(depths) != 4 || depths[h.state.indexOf("b")] != 1 {
		t.Fatalf("hover neighbourhood = %v", depths)
	}
	h.tt.ClickAt(x, y)
	if len(h.opened) != 0 {
		t.Fatal("a single click opened the node")
	}
	h.doubleClick(x, y, 0)
	if len(h.opened) != 1 || h.opened[0] != "a" {
		t.Fatalf("opened = %v", h.opened)
	}
	fx, fy := h.point("file")
	h.doubleClick(fx, fy, 0)
	if len(h.opened) != 1 {
		t.Fatal("a node that is not openable opened")
	}
	r := h.canvas()
	h.tt.Move(r.X+2, r.Y+2)
	if h.state.Hovered() != "" || h.tt.Cursor() != ui.CursorGrab {
		t.Fatalf("empty space: hovered %q, cursor %v", h.state.Hovered(), h.tt.Cursor())
	}
}

func TestZoomAndPan(t *testing.T) {
	h := newHarness(t)
	r := h.canvas()
	px, py := r.X+r.W/2, r.Y+r.H/2
	gx := (float64(px-r.X) - h.state.camera.x) / h.state.camera.zoom
	before := h.state.Zoom()
	h.tt.Wheel(px, py, 0, -120)
	if got := h.state.Zoom(); math.Abs(float64(got/before)-1.5) > 1e-3 {
		t.Fatalf("wheel zoom = %g, want ×1.5 of %g", got, before)
	}
	if after := (float64(px-r.X) - h.state.camera.x) / h.state.camera.zoom; math.Abs(after-gx) > 1e-6 {
		t.Fatal("zoom did not stay anchored at the pointer")
	}
	// Tester scrolls carry no modifiers; platforms report them.
	scroll := func(dx, dy float32, mods ui.Modifiers, precise bool) {
		e, _ := gesture.Classify(ui.InputEvent{Kind: ui.InputScroll, X: px - r.X, Y: py - r.Y, DX: dx, DY: dy, Mods: mods, Precise: precise})
		h.state.navigate(e)
	}
	zoomed := h.state.Zoom()
	scroll(0, 48, ui.Ctrl, true)
	if got := h.state.Zoom(); math.Abs(float64(zoomed/got)-1.5) > 1e-3 {
		t.Fatalf("touchpad Control-scroll zoom = %g from %g", got, zoomed)
	}
	zoomed = h.state.Zoom()
	h.tt.Pinch(px, py, 2)
	if got := h.state.Zoom(); math.Abs(float64(got/zoomed)-2) > 1e-3 {
		t.Fatalf("pinch zoom = %g from %g", got, zoomed)
	}

	// Two fingers on a touchpad pan without zooming, and so does a
	// sideways wheel.
	zoomed, cam := h.state.Zoom(), h.state.camera
	scroll(12, -30, 0, true)
	if dx, dy := h.state.camera.x-cam.x, h.state.camera.y-cam.y; math.Abs(dx+12) > 1e-6 || math.Abs(dy-30) > 1e-6 || h.state.Zoom() != zoomed {
		t.Fatalf("touchpad scroll moved the camera by (%g,%g), zoom %g", dx, dy, h.state.Zoom())
	}
	cam = h.state.camera
	scroll(40, 0, ui.Shift, false)
	if dx, dy := h.state.camera.x-cam.x, h.state.camera.y-cam.y; math.Abs(dx+40) > 1e-6 || dy != 0 || h.state.Zoom() != zoomed {
		t.Fatalf("sideways wheel moved the camera by (%g,%g), zoom %g", dx, dy, h.state.Zoom())
	}

	// Pan from a corner, which holds no node.
	cam = h.state.camera
	h.tt.Press(r.X+3, r.Y+3)
	h.tt.Move(r.X+43, r.Y+23)
	h.tt.Release(r.X+43, r.Y+23)
	if dx, dy := h.state.camera.x-cam.x, h.state.camera.y-cam.y; dx != 40 || dy != 20 {
		t.Fatalf("pan moved the camera by (%g,%g)", dx, dy)
	}
	zoomed = h.state.Zoom()
	h.doubleClick(r.X+3, r.Y+3, 0)
	if got := h.state.Zoom(); math.Abs(float64(got/zoomed)-2) > 1e-3 {
		t.Fatalf("double-click zoom = %g from %g", got, zoomed)
	}
	h.doubleClick(r.X+3, r.Y+3, ui.Shift)
	if got := h.state.Zoom(); math.Abs(float64(got/zoomed)-1) > 1e-3 {
		t.Fatalf("Shift double-click zoom = %g, want %g", got, zoomed)
	}
}

func TestDragPinsNode(t *testing.T) {
	h := newHarness(t)
	x, y := h.point("b")
	i := h.state.indexOf("b")
	h.tt.Press(x, y)
	h.tt.Move(x+30, y+10)
	h.tt.Move(x+60, y+20)
	b := h.state.sim.bodies[i]
	if !b.pinned || !h.state.sim.running {
		t.Fatal("dragging did not pin the node")
	}
	if nx, ny := h.point("b"); math.Abs(float64(nx-x-60)) > .5 || math.Abs(float64(ny-y-20)) > .5 {
		t.Fatalf("node at (%g,%g), want pointer offset (%g,%g)", nx, ny, x+60, y+20)
	}
	h.tt.Release(x+60, y+20)
	if h.state.sim.bodies[i].pinned {
		t.Fatal("release did not unpin the node")
	}
	if len(h.opened) != 0 {
		t.Fatal("dragging opened the node")
	}
}

func TestDataUpdatesKeepLayout(t *testing.T) {
	h := newHarness(t)
	ax, ay := h.state.sim.bodies[h.state.indexOf("a")].x, h.state.sim.bodies[h.state.indexOf("a")].y
	key := h.state.layoutKey

	// Theme and color changes repaint without moving anything.
	h.tt.SetDark(true)
	if h.state.layoutKey != key || h.state.sim.bodies[h.state.indexOf("a")].x != ax {
		t.Fatal("theme change restarted the layout")
	}
	if h.state.nodes[h.state.indexOf("lonely")].fill != DefaultPalette(true).Isolated {
		t.Fatal("theme change did not recolor")
	}

	// A new node starts beside its neighbour; others keep their places.
	x, y := h.point("a")
	h.tt.Move(x, y)
	h.data = Data{
		Nodes: append(append([]Node(nil), sample.Nodes...), Node{ID: "d", Label: "Delta"}),
		Edges: append(append([]Edge(nil), sample.Edges...), Edge{Source: "a", Target: "d"}),
	}
	h.state.reduceMotion = false
	h.tt.SetPreferences(ui.Preferences{})
	h.tt.Frame()
	d := h.state.sim.bodies[h.state.indexOf("d")]
	if math.Hypot(d.x-ax, d.y-ay) > 400 {
		t.Fatalf("new node placed far from its neighbour: (%g,%g)", d.x, d.y)
	}
	if h.state.Hovered() != "a" {
		t.Fatal("hover was lost across the update")
	}

	// Removing the hovered node clears hover.
	h.data = Data{Nodes: sample.Nodes[2:], Edges: sample.Edges}
	h.tt.Frame()
	if h.state.Hovered() != "" || len(h.state.nodes) != 4 {
		t.Fatalf("after removal: hovered %q, %d nodes", h.state.Hovered(), len(h.state.nodes))
	}
}

func TestSlotsAndCustomColors(t *testing.T) {
	palette := DefaultPalette(false)
	palette.Background = ui.Hex("#102030")
	opts := Options{
		Label: "Graph", Height: 300, EmptyText: "Nothing yet", Palette: &palette,
		GroupColor: func(group string, index, count int, dark bool) (ui.Color, ui.Color) {
			return ui.RGB(uint8((index*80)&0xff), 0, 0), ui.RGB(0, 0, 0)
		},
		Overlay: func(c *ui.Context) { ui.Text(c, "Settings") },
	}
	var data Data
	state := &State{}
	tt := ui.NewTester(func(c *ui.Context) { View(c, state, data, opts) }, 400, 300)
	if !tt.HasText("Nothing yet") || tt.HasText("Settings") {
		t.Fatal("empty graph should show only its empty text")
	}
	opts.Empty = func(c *ui.Context) { ui.Text(c, "Custom empty") }
	tt.Frame()
	if !tt.HasText("Custom empty") {
		t.Fatal("empty slot missing")
	}
	data = sample
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if !tt.HasText("Settings") {
		t.Fatal("overlay slot missing")
	}
	if got := state.nodes[state.indexOf("c")].fill; got != ui.RGB(0, 0, 0) {
		t.Fatalf("group color for areas (index 0) = %v", got)
	}
	if got := state.nodes[state.indexOf("a")].fill; got != ui.RGB(80, 0, 0) {
		t.Fatalf("group color for projects (index 1) = %v", got)
	}
	img := tt.Image()
	if px := img.RGBAAt(2, 2); px.R != 0x10 || px.G != 0x20 || px.B != 0x30 {
		t.Fatalf("background pixel = %v", px)
	}
}

func TestDefaultGroupColorsSpreadHues(t *testing.T) {
	a, _ := DefaultGroupColor("areas", 0, 2, false)
	b, _ := DefaultGroupColor("projects", 1, 2, false)
	if a == b {
		t.Fatal("two groups share a color")
	}
	if got := hsl(208, 100, 50); got != ui.RGB(0, 136, 255) {
		t.Fatalf("hsl(208,100%%,50%%) = %v", got)
	}
}

func TestFitLabel(t *testing.T) {
	measure := func(s string) float32 { return float32(len([]rune(s))) * 10 }
	if got, w := fitLabel("short", 132, measure); got != "short" || w != 50 {
		t.Fatalf("fit = %q %g", got, w)
	}
	if got, _ := fitLabel("a much longer label here", 132, measure); got != "a much longe…" {
		t.Fatalf("truncated = %q", got)
	}
	if got, _ := fitLabel("x", 5, measure); got != "" {
		t.Fatalf("too narrow = %q", got)
	}
}

func TestRevealCount(t *testing.T) {
	if revealCount(10, 0) != 1 || revealCount(10, revealDuration) != 10 || revealCount(0, 0) != 0 {
		t.Fatal("reveal bounds")
	}
	if mid := revealCount(100, revealDuration/2); mid != 75 {
		t.Fatalf("reveal midpoint = %d, want 75 with ease-out", mid)
	}
}
