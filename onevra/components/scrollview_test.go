package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestScrollViewAxes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, axes := range []ScrollAxes{ScrollVertical, ScrollHorizontal, ScrollBoth, ScrollAxes(255)} {
			var state ui.ScrollState
			var content ui.Rect
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				ScrollView(c, ScrollViewOptions{Axes: axes}).Fill().TrackScroll(&state).Children(func() {
					ui.Box(c).Size(600, 800).Shrink(0).Label("content").DrawOver(func(_ *ui.Painter, r ui.Rect) { content = r })
				})
			}, 200, 200)
			x, y := float32(0), float32(600)
			if axes == ScrollHorizontal || axes == ScrollBoth {
				x = 400
			}
			if axes == ScrollHorizontal {
				y = 0
			}
			if state.MaxX != x || state.MaxY != y {
				t.Fatalf("dark=%v axes=%v: bounds %+v, want %v,%v", dark, axes, state, x, y)
			}
			initial := content
			tt.Scroll(100, 100, 50, 70)
			if state.X != min(x, 50) || state.Y != min(y, 70) {
				t.Fatalf("axes=%v: wheel %+v", axes, state)
			}
			state.X, state.Y = 1000, 1000
			tt.Frame()
			if state.X != x || state.Y != y {
				t.Fatalf("axes=%v: programmatic clamping %+v", axes, state)
			}
			r := content
			if r.X != initial.X-x || r.Y != initial.Y-y {
				t.Fatalf("axes=%v: content %+v", axes, r)
			}
		}
	}
}

func TestScrollViewArrangement(t *testing.T) {
	for _, axes := range []ScrollAxes{ScrollVertical, ScrollHorizontal, ScrollBoth} {
		tt := ui.NewTester(func(c *ui.Context) {
			ScrollView(c, ScrollViewOptions{Axes: axes}).Fill().Children(func() {
				ui.Box(c).Size(40, 40).Shrink(0).Label("first")
				ui.Box(c).Size(40, 40).Shrink(0).Label("second")
			})
		}, 200, 200)
		a, _ := tt.Find("first")
		b, _ := tt.Find("second")
		if axes == ScrollHorizontal {
			if b.X != a.X+40 || b.Y != a.Y {
				t.Fatalf("horizontal arrangement: %+v %+v", a, b)
			}
		} else if b.Y != a.Y+40 || b.X != a.X {
			t.Fatalf("column arrangement: %+v %+v", a, b)
		}
	}
}

func TestScrollViewInsets(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, axes := range []ScrollAxes{ScrollVertical, ScrollHorizontal} {
			var state ui.ScrollState
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				ScrollView(c, ScrollViewOptions{Axes: axes}).Fill().
					ScrollbarInsets(40, 10, 20, 30).TrackScroll(&state).Children(func() {
					ui.Box(c).Size(600, 800).Shrink(0).Label("content")
				})
			}, 200, 200)
			r, _ := tt.Find("content")
			if r.X != 0 || r.Y != 0 {
				t.Fatalf("insets moved content: %+v", r)
			}
			if axes == ScrollVertical {
				// The vertical thumb begins below the top inset, inset from the right.
				// MyGo shows the bars while the view is hovered, and only a shown bar
				// takes a press.
				tt.Move(184, 48)
				tt.Press(184, 48)
				tt.Move(184, 178)
				tt.Release(184, 178)
				if state.Y != state.MaxY || state.Y == 0 {
					t.Fatalf("vertical inset track drag: %+v", state)
				}
			} else {
				// The horizontal thumb begins after the left inset, above the bottom inset.
				tt.Move(40, 174)
				tt.Press(40, 174)
				tt.Move(188, 174)
				tt.Release(188, 174)
				if state.X != state.MaxX || state.X == 0 {
					t.Fatalf("horizontal inset track drag: %+v", state)
				}
			}
		}
	}
}
