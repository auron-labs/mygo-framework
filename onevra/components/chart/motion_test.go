package chart

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
	base "github.com/egoist/mygo/ui/chart"
)

func TestDataMotionRetargetAndReset(t *testing.T) {
	values, keys := []float64{0, math.NaN()}, []string{"a"}
	opts := Options{AnimationDuration: time.Second}
	var current []float64
	var settled bool
	tt := ui.NewTester(func(c *ui.Context) {
		e := ui.Box(c)
		current = animateValues(c, e, bars, 2, keys, values, opts)
		settled = e.Settled(dataProgress)
	}, 100, 100)
	if current[0] != 0 || !settled {
		t.Fatal("initial data animated")
	}
	values[0] = 100 // Updating borrowed data in place must retain the previous frame.
	tt.Frame()
	if current[0] >= 100 || settled {
		t.Fatal("data update did not start a transition")
	}
	tt.Advance(60 * time.Millisecond)
	tt.Frame()
	if current[0] <= 0 || current[0] >= 100 || !math.IsNaN(current[1]) {
		t.Fatalf("incorrect intermediate values: %v", current)
	}
	values[0] = -100
	tt.Frame()
	if current[0] <= 0 || current[0] >= 100 || settled {
		t.Fatalf("retarget jumped to a dataset endpoint: %v", current)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if current[0] != -100 || !settled {
		t.Fatal("Reduced Motion did not finish an ongoing transition")
	}
	tt.SetPreferences(ui.Preferences{})
	values[0], keys[0] = 20, "replacement"
	tt.Frame()
	if current[0] != 20 || !settled {
		t.Fatal("new series interpolated unrelated data")
	}
	values[1] = 5
	tt.Frame()
	if current[1] != 5 || !settled {
		t.Fatal("missing value did not become visible immediately")
	}
	opts.AnimationDuration, values[0] = -1, 30
	tt.Frame()
	if current[0] != 30 || !settled {
		t.Fatal("disabled animation did not jump")
	}
	opts.AnimationDuration = 40 * time.Millisecond
	values[0] = 50
	tt.Frame()
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	if current[0] != 50 || !settled {
		t.Fatal("completed transition kept moving")
	}
}

func TestChartDataTransitions(t *testing.T) {
	views := map[string]func(*ui.Context, *State, Data, Options) ui.Element{}
	for name, view := range cartesianViews {
		views[name] = view
	}
	views["Donut"] = func(c *ui.Context, state *State, data Data, opts Options) ui.Element {
		return Donut(c, state, []Slice{
			{ID: "a", Label: "Alpha", Value: data.Series[0].Values[0]},
			{ID: "b", Label: "Beta", Value: data.Series[0].Values[1]},
		}, opts)
	}
	for name, view := range views {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", name, dark), func(t *testing.T) {
				data := Data{Labels: []string{"A", "B"}, Series: []Series{{ID: "a", Label: "Alpha", Values: []float64{10, 90}}}}
				opts := Options{Label: "Plot", AnimationDuration: time.Second}
				state := State{Index: -1}
				var settled bool
				build := func(c *ui.Context) {
					c.SetTheme(theme.Palette(dark))
					e := view(c, &state, data, opts)
					settled = e.Settled(dataProgress)
				}
				tt := ui.NewTester(build, 320, 380)
				before := slices.Clone(tt.Image().Pix)
				data.Series[0].Values[0], data.Series[0].Values[1] = 90, 10
				tt.Frame()
				if settled {
					t.Fatal("chart did not animate its data")
				}
				tt.Advance(50 * time.Millisecond)
				tt.Frame()
				middle := slices.Clone(tt.Image().Pix)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				if bytes.Equal(before, middle) || bytes.Equal(middle, tt.Image().Pix) || !settled {
					t.Fatal("chart did not paint an intermediate shape")
				}
				opts.AnimationDuration = -1
				want := ui.NewTester(build, 320, 380)
				if !bytes.Equal(want.Image().Pix, tt.Image().Pix) {
					t.Fatal("animation did not arrive at the nonanimated geometry")
				}
			})
		}
	}
}

func TestAnimatedStackAndDomainStayTogether(t *testing.T) {
	data := Data{Labels: []string{"A"}, Series: []Series{
		{ID: "a", Values: []float64{10}}, {ID: "b", Values: []float64{20}},
	}}
	var shown prepared
	tt := ui.NewTester(func(c *ui.Context) {
		shown = animatePrepared(c, ui.Box(c), data, Options{AnimationDuration: time.Second}, stacks, prepare(data, &State{}, stacks))
	}, 100, 100)
	data.Series[0].Values[0], data.Series[1].Values[0] = -40, 80
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	tt.Frame()
	if shown.values[0][0] == -40 || shown.y.Hi == 80 {
		t.Fatal("data or scale jumped to its target")
	}
	for _, series := range shown.stack {
		if series[0].Lo < shown.y.Lo || series[0].Hi > shown.y.Hi {
			t.Fatal("animated stack escaped its animated domain")
		}
	}
	if shown.values[0][0] > 0 && shown.stack[0][0].Hi != shown.stack[1][0].Lo {
		t.Fatal("stacked segments separated during animation")
	}
}

func TestAnimatedDonutBoundariesStayJoined(t *testing.T) {
	data := []Slice{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	values := []float64{10, 30, 60}
	var arcs []base.Slice
	tt := ui.NewTester(func(c *ui.Context) {
		arcs = animateSlices(c, ui.Box(c), data, []int{0, 1, 2}, base.Pie(values, .025), Options{AnimationDuration: time.Second})
	}, 100, 100)
	values = []float64{60, 25, 15}
	tt.Frame()
	tt.Advance(50 * time.Millisecond)
	tt.Frame()
	if arcs[0].End == base.Pie(values, .025)[0].End {
		t.Fatal("donut jumped to its target")
	}
	for i, arc := range arcs {
		if arc.End <= arc.Start || (i > 0 && arcs[i-1].End != arc.Start) {
			t.Fatal("animated sectors overlap or separate")
		}
	}
	if arcs[0].Start != 0 || math.Abs(arcs[2].End-2*math.Pi) > 1e-12 {
		t.Fatal("animated slices no longer fill a ring")
	}
}

func TestDonutHoverFollowsChangingGeometry(t *testing.T) {
	data := []Slice{{ID: "a", Value: 10}, {ID: "b", Value: 90}}
	state := State{Index: -1}
	tt := ui.NewTester(func(c *ui.Context) {
		Donut(c, &state, data, Options{Label: "Plot", AnimationDuration: time.Second})
	}, 280, 400)
	r, _ := tt.Find("Plot")
	x, y := r.X+r.W/2+80, r.Y+r.H/2
	tt.Move(x, y)
	if state.Index != 1 {
		t.Fatal("pointer did not select the initial slice")
	}
	data[0].Value, data[1].Value = 90, 10
	tt.Frame()
	tt.Advance(150 * time.Millisecond)
	tt.Frame()
	if state.Index != 0 {
		t.Fatal("stationary pointer did not follow the animated slice boundary")
	}
	tt.ClickAt(x, y)
	tt.Key(0, ui.KeyEnd)
	tt.Frame()
	if state.Index != 1 {
		t.Fatal("animated hover replaced keyboard selection")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Move(x-2, y)
	data[0].Value, data[1].Value = 10, 90
	tt.Frame()
	if state.Index != 1 {
		t.Fatal("Reduced Motion left hover selection on stale geometry")
	}
}
