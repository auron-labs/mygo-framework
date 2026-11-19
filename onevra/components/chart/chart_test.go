package chart

import (
	"fmt"
	"image"
	stdcolor "image/color"
	"math"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

var sample = Data{Labels: []string{"Mon", "Tue", "Wed", "Thu"}, Series: []Series{
	{ID: "a", Label: "Alpha", Values: []float64{10, 40, -20, 30}},
	{ID: "b", Label: "Beta", Values: []float64{20, 10, -10, 40}},
}}

var cartesianViews = map[string]func(*ui.Context, *State, Data, Options) ui.Element{
	"Bar": Bar, "Line": Line, "Area": Area, "StackedBar": StackedBar,
}

func TestCartesianInteraction(t *testing.T) {
	for name, view := range cartesianViews {
		for _, scale := range []float32{.75, 1, 1.5} {
			t.Run(fmt.Sprintf("%s/%g", name, scale), func(t *testing.T) {
				state := State{Index: -1}
				var selected Selection
				opts := Options{Label: "Plot", Description: "Arrow keys", EmptyText: "Empty", Detail: func(_ *ui.Context, s Selection) { selected = s }}
				tt := ui.NewTester(func(c *ui.Context) {
					theme.Apply(c)
					ui.Column(c).Width(280).Scale(scale, scale).TransformOrigin(0, 0).Children(func() { view(c, &state, sample, opts) })
				}, 500, 600)
				for _, dark := range []bool{false, true} {
					tt.SetDark(dark)
					r, ok := tt.Find("Plot")
					if !ok || r.W <= 0 || r.H <= 0 {
						t.Fatalf("invalid plot: %+v", r)
					}
					tt.Move(r.X+r.W*.92, r.Y+r.H/2)
					if state.Index != 3 || selected.Label != "Thu" || len(selected.Values) != 2 {
						t.Fatalf("pointer selection: %+v", selected)
					}
					tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
					tt.Key(0, ui.KeyHome)
					tt.Key(0, ui.KeyRight)
					if state.Index != 1 || selected.Values[0].Value != 40 {
						t.Fatalf("keyboard selection: %+v", selected)
					}
					tt.Key(0, ui.KeyEnd)
					if state.Index != 3 {
						t.Fatal("End did not select last category")
					}
					tt.Key(0, ui.KeyEscape)
					if state.Index != -1 {
						t.Fatal("Escape did not clear selection")
					}
				}
			})
		}
	}
}

func TestSelectionTooltip(t *testing.T) {
	state := State{Index: -1}
	tt := ui.NewTester(func(c *ui.Context) {
		Bar(c, &state, sample, Options{Label: "Plot", Detail: func(c *ui.Context, s Selection) { ui.Text(c, "Tip "+s.Label) }})
	}, 400, 500)
	r, _ := tt.Find("Plot")
	tt.Move(r.X+r.W*.92, r.Y+r.H*.3)
	high, ok := tt.Find("Tip Thu")
	tt.Move(r.X+r.W*.92, r.Y+r.H*.6)
	low, _ := tt.Find("Tip Thu")
	if !ok || low.Y <= high.Y || low.X < r.X+r.W*.92 && low.X+low.W > r.X+r.W*.92 {
		t.Fatalf("tooltip does not follow the pointer beside it: %+v %+v", high, low)
	}
	tt.Move(r.X+r.W/2, r.Y+r.H+200)
	if tt.HasText("Tip Thu") {
		t.Fatal("tooltip stayed after the pointer left")
	}
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.Move(r.X+r.W/2, r.Y+r.H+200)
	tt.Key(0, ui.KeyHome)
	if !tt.HasText("Tip Mon") {
		t.Fatal("keyboard selection has no tooltip")
	}
	if err := tt.Click("Alpha"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("Tip Mon") {
		t.Fatal("tooltip stayed after the plot lost the focus")
	}
}

func TestLegendEmptyAndDataChanges(t *testing.T) {
	for name, view := range cartesianViews {
		t.Run(name, func(t *testing.T) {
			data := sample
			state := State{Index: 3}
			var selected Selection
			tt := ui.NewTester(func(c *ui.Context) {
				view(c, &state, data, Options{Label: "Plot", EmptyText: "Empty", Detail: func(_ *ui.Context, s Selection) { selected = s }})
			}, 280, 500)
			if err := tt.Click("Beta"); err != nil {
				t.Fatal(err)
			}
			r, _ := tt.Find("Plot")
			tt.Move(r.X+r.W*.92, r.Y+r.H/2)
			if !state.Hidden["b"] || len(selected.Values) != 1 || selected.Values[0].ID != "a" {
				t.Fatalf("legend did not hide series: %+v", selected)
			}
			data.Series = []Series{sample.Series[1], sample.Series[0]}
			tt.Frame()
			if len(selected.Values) != 1 || selected.Values[0].ID != "a" {
				t.Fatal("hidden state followed series position instead of identity")
			}
			if err := tt.Click("Alpha"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText("Empty") || state.Index != -1 {
				t.Fatal("all-hidden state is not empty")
			}
			if err := tt.Click("Beta"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if tt.HasText("Empty") {
				t.Fatal("legend cannot restore an empty chart")
			}
			state.Index, data.Labels = 3, data.Labels[:1]
			tt.Frame()
			if state.Index != -1 {
				t.Fatal("data shrink left a stale selection")
			}
		})
	}
}

func TestDisabledCharts(t *testing.T) {
	views := map[string]func(*ui.Context, *State){
		"Donut": func(c *ui.Context, state *State) {
			Donut(c, state, []Slice{{ID: "a", Label: "Alpha", Value: 2}, {ID: "b", Label: "Beta", Value: 3}}, Options{Label: "Plot"})
		},
		"Bubble": func(c *ui.Context, state *State) {
			Bubble(c, state, []Slice{{ID: "a", Label: "Alpha", Value: 2}, {ID: "b", Label: "Beta", Value: 3}}, Options{Label: "Plot"})
		},
	}
	for name, view := range cartesianViews {
		views[name] = func(c *ui.Context, state *State) { view(c, state, sample, Options{Label: "Plot"}) }
	}
	for name, view := range views {
		t.Run(name, func(t *testing.T) {
			state := State{Index: -1}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Fill().Disabled(true).Children(func() { view(c, &state) })
			}, 280, 500)
			if err := tt.Click("Plot"); err != nil {
				t.Fatal(err)
			}
			tt.Key(0, ui.KeyEnd)
			if err := tt.Click("Alpha"); err != nil {
				t.Fatal(err)
			}
			if state.Index != -1 || len(state.Hidden) != 0 {
				t.Fatalf("disabled chart changed state: %+v", state)
			}
		})
	}
}

func TestDomainAndMissingValues(t *testing.T) {
	data := Data{Labels: []string{"A", "B", "C", "D"}, Series: []Series{
		{ID: "a", Values: []float64{10, -20, math.NaN(), math.Inf(1), 999}},
		{ID: "b", Values: []float64{30, -40}},
	}}
	p := prepare(data, &State{}, stacks)
	if !p.valid || p.y.Lo > -60 || p.y.Hi < 40 || p.stack[1][0].Hi != 40 || p.stack[1][1].Lo != -60 {
		t.Fatalf("incorrect stacked domain: %+v", p)
	}
	if !math.IsNaN(p.values[0][2]) || !math.IsNaN(p.values[0][3]) || !math.IsNaN(p.values[1][3]) {
		t.Fatal("invalid and missing values must remain gaps")
	}
	if p := prepare(data, &State{Hidden: map[string]bool{"b": true}}, bars); p.y.Hi != 10 || p.y.Lo != -20 {
		t.Fatalf("hidden or excess values changed the domain: %+v", p.y)
	}
	for _, values := range [][]float64{nil, {math.NaN()}, {math.Inf(1)}, {-math.MaxFloat64, math.MaxFloat64}} {
		bad := Data{Labels: []string{"A", "B"}, Series: []Series{{ID: "a", Values: values}}}
		if prepare(bad, &State{}, bars).valid {
			t.Fatalf("invalid data has a scale: %v", values)
		}
	}
	zero := Data{Labels: []string{"A"}, Series: []Series{{ID: "a", Values: []float64{0}}}}
	if p := prepare(zero, &State{}, areas); !p.valid || p.y.Lo != 0 || p.y.Hi != 1 {
		t.Fatal("zero data should use a visible nondegenerate scale")
	}
}

func TestDonutSelectionAndFiltering(t *testing.T) {
	data := []Slice{{ID: "a", Label: "Alpha", Value: 3}, {ID: "zero", Label: "Zero"}, {ID: "b", Label: "Beta", Value: 1}, {ID: "bad", Label: "Invalid", Value: math.Inf(1)}}
	state := State{Index: -1}
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		Donut(c, &state, data, Options{Label: "Plot", EmptyText: "Empty", FormatValue: func(v float64) string { return fmt.Sprintf("%.0f h", v) }})
	}, 280, 500)
	if tt.HasText("Zero") || tt.HasText("Invalid") || !tt.HasText("4 h") {
		t.Fatal("donut did not filter invalid values or format the total")
	}
	if err := tt.Click("Beta"); err != nil {
		t.Fatal(err)
	}
	if state.Index != 2 || !tt.HasText("1 h") {
		t.Fatal("legend selection lost input index or formatting")
	}
	if err := tt.Click("Plot"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyRight)
	if state.Index != 2 {
		t.Fatal("keyboard selection did not skip zero values")
	}
	tt.Key(0, ui.KeyEscape)
	if state.Index != -1 {
		t.Fatal("Escape did not clear donut selection")
	}
	r, _ := tt.Find("Plot")
	tt.Move(r.X+r.W/2+80, r.Y+r.H/2)
	if state.Index != 0 {
		t.Fatal("ring hover did not select slice")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	if state.Index != -1 {
		t.Fatal("the ring hole should not select a slice")
	}
	data = []Slice{{ID: "a", Value: math.MaxFloat64}, {ID: "b", Value: math.MaxFloat64}}
	tt.Frame()
	tt.SetDark(true)
	if tt.Image() == nil || tt.HasText("Empty") {
		t.Fatal("large positive slices should still render")
	}
	data = nil
	tt.Frame()
	if state.Index != -1 || !tt.HasText("Empty") {
		t.Fatal("empty donut kept a selection")
	}
}

func TestBubblePacking(t *testing.T) {
	values := []float64{1, .02, .5, .5, .3, .9, .1, .05, .7, .25}
	for _, aspect := range []float64{.5, 1, 2.5, 4} {
		circles := pack(values, aspect)
		x0, y0, x1, y1, area := 0.0, 0.0, 0.0, 0.0, 0.0
		for i, a := range circles {
			if want := math.Sqrt(values[i]) * circles[0].r; math.Abs(a.r-want) > 1e-9 {
				t.Fatalf("radius %d = %g, want area proportional to value (%g)", i, a.r, want)
			}
			for _, b := range circles[i+1:] {
				if math.Hypot(a.x-b.x, a.y-b.y) < a.r+b.r {
					t.Fatalf("aspect %g: bubbles overlap: %+v %+v", aspect, a, b)
				}
			}
			x0, y0, x1, y1 = min(x0, a.x-a.r), min(y0, a.y-a.r), max(x1, a.x+a.r), max(y1, a.y+a.r)
			area += math.Pi * a.r * a.r
		}
		if math.Abs(x0+x1) > 1e-9 || math.Abs(y0+y1) > 1e-9 {
			t.Fatalf("aspect %g: pack is not centered", aspect)
		}
		if extent := max((x1-x0)/aspect, y1-y0); math.Abs(extent-1) > 1e-9 {
			t.Fatalf("aspect %g: pack does not fill the view: %g", aspect, extent)
		}
		if density := area / aspect; density < .5 {
			t.Fatalf("aspect %g: pack covers only %.2f of the view", aspect, density)
		}
	}
	for _, b := range viewBubbles(pack(values, 2), ui.Rect{X: 10, Y: 20, W: 300, H: 120}, 2) {
		if b.x-b.r < 10+bubbleMargin-.01 || b.x+b.r > 310-bubbleMargin+.01 || b.y-b.r < 20+bubbleMargin-.01 || b.y+b.r > 140-bubbleMargin+.01 {
			t.Fatalf("bubble outside the plot: %+v", b)
		}
	}
}

func TestBubbleSprings(t *testing.T) {
	data := []Slice{{ID: "a", Label: "Alpha", Value: 9}, {ID: "b", Label: "Beta", Value: 4}}
	rest := map[bubbleKey]bool{}
	tt := ui.NewTester(func(c *ui.Context) {
		e := Bubble(c, nil, data, Options{Label: "Plot"})
		for _, s := range data {
			for part := range bubbleLifted {
				rest[bubbleKey{s.ID, part}] = e.Settled(bubbleKey{s.ID, part})
			}
		}
	}, 400, 500)
	settled := func(id string, part uint8) bool { return rest[bubbleKey{id, part}] }
	if !settled("a", bubbleR) || !settled("b", bubbleR) || !settled("a", bubbleX) {
		t.Fatal("bubbles should show packed on first display")
	}
	data = append(data, Slice{ID: "c", Label: "Gamma", Value: 1})
	tt.Frame()
	if settled("a", bubbleR) || settled("c", bubbleR) || !settled("c", bubbleX) {
		t.Fatal("bubbles should spring to a new pack while a new one grows in place")
	}
	tt.Advance(2 * time.Second)
	tt.Frame()
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	data[0].Value = 1
	tt.Frame()
	if !settled("a", bubbleR) || !settled("b", bubbleX) {
		t.Fatal("Reduced Motion should skip the springs")
	}
}

func TestBubbleSelectionAndFiltering(t *testing.T) {
	data := []Slice{{ID: "a", Label: "Alpha", Value: 9}, {ID: "zero", Label: "Zero"}, {ID: "b", Label: "Beta", Value: 1}, {ID: "bad", Label: "Invalid", Value: math.NaN()}}
	state := State{Index: -1}
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		Bubble(c, &state, data, Options{Label: "Plot", EmptyText: "Empty", FormatValue: func(v float64) string { return fmt.Sprintf("%.0f h", v) }})
	}, 320, 500)
	if tt.HasText("Zero") || tt.HasText("Invalid") {
		t.Fatal("bubbles did not filter invalid values")
	}
	if err := tt.Click("Beta"); err != nil {
		t.Fatal(err)
	}
	if state.Index != 2 || tt.HasText("1 h") {
		t.Fatal("legend selection lost input index or showed a tooltip")
	}
	if err := tt.Click("Plot"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyRight)
	if state.Index != 2 || !tt.HasText("1 h") {
		t.Fatal("keyboard selection did not skip zero values or show its tooltip")
	}
	tt.Key(0, ui.KeyEscape)
	if state.Index != -1 {
		t.Fatal("Escape did not clear bubble selection")
	}
	r, _ := tt.Find("Plot")
	tt.Move(r.X+2, r.Y+2)
	if state.Index != -1 {
		t.Fatal("space between bubbles should not select one")
	}
	tt.Advance(2 * time.Second)
	aspect := bubbleAspect(r.W, r.H)
	largest := viewBubbles(pack([]float64{1, 1. / 9}, aspect), r, aspect)[0]
	tt.Move(largest.x, largest.y)
	if state.Index != 0 {
		t.Fatal("hovering the largest bubble did not select it")
	}
	tt.SetDark(true)
	if tt.Image() == nil {
		t.Fatal("bubbles did not render")
	}
	data = nil
	tt.Frame()
	if state.Index != -1 || !tt.HasText("Empty") {
		t.Fatal("empty bubble chart kept a selection")
	}
}

func TestLocalStateAndSlots(t *testing.T) {
	selected := [2]Selection{{Index: -1}, {Index: -1}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			for i := range 2 {
				ui.Box(c).Key(i).FillWidth().Children(func() {
					Line(c, nil, sample, Options{
						Label: fmt.Sprint(i), PlotHeight: 100, HideLegend: true,
						Detail: func(_ *ui.Context, s Selection) { selected[i] = s },
					})
				})
			}
			Bar(c, nil, Data{}, Options{PlotHeight: 60, Empty: func(c *ui.Context) { ui.Text(c, "Custom empty") }})
		})
	}, 280, 450)
	if err := tt.Click("0"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyHome)
	if selected[0].Index != 0 || selected[1].Index != -1 || !tt.HasText("Custom empty") {
		t.Fatalf("element-local state leaked or slots failed: %+v", selected)
	}
}

func TestSplitBarHighlightAndToggle(t *testing.T) {
	data := []Slice{{ID: "a", Label: "Alpha", Value: 3}, {ID: "zero", Label: "Zero"}, {ID: "b", Label: "Beta", Value: 1}}
	state := State{Index: -1}
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		SplitBar(c, &state, data, Options{Label: "Plot", FormatValue: func(v float64) string { return fmt.Sprintf("%.0f h", v) }})
	}, 360, 300)
	for _, text := range []string{"Total", "4 h", "75.0%", "25.0%"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q", text)
		}
	}
	if tt.HasText("Zero") {
		t.Fatal("non-positive slice appeared")
	}
	// The title shares the bar's name; the bar lies between readout and legend.
	readout, _ := tt.Find("4 h")
	legend, ok := tt.Find("Alpha 75.0%")
	if !ok {
		t.Fatal("missing legend item")
	}
	r := ui.Rect{X: 0, Y: (readout.Y + readout.H + legend.Y) / 2, W: 360}
	tt.Move(r.X+r.W*.2, r.Y)
	if state.Index != 0 || !tt.HasText("3 h") {
		t.Fatalf("hover did not highlight the first segment: %d", state.Index)
	}
	tt.Move(r.X+r.W*.97, r.Y)
	if state.Index != 2 {
		t.Fatalf("hover did not highlight the last segment: %d", state.Index)
	}
	tt.ClickAt(r.X+r.W/2, r.Y)
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyRight)
	if state.Index != 2 {
		t.Fatal("arrow keys did not skip the omitted slice")
	}
	tt.Key(0, ui.KeyEnter)
	if !state.Hidden["b"] || state.Index != -1 || !tt.HasText("100.0%") || !tt.HasText("Off") {
		t.Fatalf("Enter did not switch the segment off: %+v", state)
	}
	tt.Key(0, ui.KeyHome)
	tt.Key(0, ui.KeyEnter)
	if state.Hidden["a"] {
		t.Fatal("the last visible segment was switched off")
	}
	if err := tt.Click("Beta Off"); err != nil {
		t.Fatal(err)
	}
	if state.Hidden["b"] || !tt.HasText("25.0%") {
		t.Fatal("legend did not switch the segment back on")
	}
	tt.ClickAt(r.X+r.W/2, r.Y)
	tt.Key(0, ui.KeyEscape)
	if state.Index != -1 {
		t.Fatal("Escape did not clear the highlight")
	}
	data = nil
	tt.Frame()
	if !tt.HasText("—") {
		t.Fatal("empty split bar shows no empty state")
	}
}

func TestSplitBarEdgesReflow(t *testing.T) {
	state := State{Index: -1}
	data := []Slice{{ID: "a", Label: "A", Value: 1}, {ID: "b", Label: "B", Value: 1}}
	var edges []float64
	tt := ui.NewTester(func(c *ui.Context) {
		e := SplitBar(c, &state, data, Options{Label: "Plot", AnimationDuration: time.Second})
		edges = ui.Local(e, "onevra.chart.split", func() splitMotion { return splitMotion{} }).edges
	}, 360, 300)
	edge := func() float64 { return edges[0] }
	if edge() != .5 {
		t.Fatalf("initial edge = %v", edge())
	}
	data[1].Value = 3
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	if v := edge(); v >= .5 || v <= .25 {
		t.Fatalf("edge did not reflow gradually: %v", v)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if v := edge(); v != .25 {
		t.Fatalf("Reduced Motion did not settle the edge: %v", v)
	}
	if got := splitTargets([]float64{.9, .1}, 1, .3); math.Abs(got[0]+got[1]-1) > 1e-9 || got[1] != .3 {
		t.Fatalf("widening broke the 100%% total: %v", got)
	}
}

func TestSplitBarFadingLabelStaysInvisible(t *testing.T) {
	slices := []Slice{
		{ID: "a", Label: "Alpha", Value: 3, Color: ui.Hex("#8a2c00")},
		{ID: "b", Label: "Beta", Value: 1, Color: ui.Hex("#cbbfae")},
	}
	state := State{Index: -1}
	var text ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		theme.Apply(c)
		text = c.Theme().Text
		SplitBar(c, &state, slices, Options{Label: "Plot"})
	}, 360, 300)
	// The bar's width is known only after a frame, then labels fade in.
	for range 2 {
		tt.Advance(time.Second)
		tt.Frame()
	}
	near := func(got stdcolor.RGBA, want ui.Color) bool {
		diff := func(a, b uint8) bool { return int(a)-int(b) <= 2 && int(b)-int(a) <= 2 }
		return got.A == 255 && diff(got.R, want.R) && diff(got.G, want.G) && diff(got.B, want.B)
	}
	// paint counts the segment's white and theme-text glyph pixels.
	paint := func(img *image.RGBA) (white, dark int, found bool) {
		fill := ui.Hex("#8a2c00")
		var rows []int
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			n := 0
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				if near(img.RGBAAt(x, y), fill) {
					n++
				}
			}
			if n >= 20 {
				rows = append(rows, y)
			}
		}
		if len(rows) == 0 {
			return 0, 0, false
		}
		y0, y1 := rows[0], rows[len(rows)-1]
		x0, x1 := img.Bounds().Max.X, img.Bounds().Min.X
		for y := y0; y <= y1; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				if near(img.RGBAAt(x, y), fill) {
					x0, x1 = min(x0, x), max(x1, x)
				}
			}
		}
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				switch got := img.RGBAAt(x, y); {
				case got.R == 255 && got.G == 255 && got.B == 255:
					white++
				case near(got, text):
					dark++
				}
			}
		}
		return white, dark, true
	}
	if white, _, ok := paint(tt.Image()); !ok || white == 0 {
		t.Fatal("the segment's label is not painted")
	}
	slices[0].Value, slices[1].Value = 1, 4
	tt.Frame()
	for range 60 {
		tt.Advance(20 * time.Millisecond)
		tt.Frame()
		if _, dark, ok := paint(tt.Image()); !ok || dark > 0 {
			t.Fatalf("fading label fell back to the theme text color: %d pixels", dark)
		}
	}
	if white, _, _ := paint(tt.Image()); white > 0 {
		t.Fatal("the label did not finish fading out")
	}
}
