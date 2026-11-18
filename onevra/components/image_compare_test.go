package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestImageCompareInteraction(t *testing.T) {
	for _, orientation := range []Orientation{Horizontal, Vertical} {
		t.Run(fmt.Sprint(orientation), func(t *testing.T) {
			position, changes := float32(50), 0
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(false))
				s := ImageCompare(c, &position, func() { ui.Box(c).Fill().Label("before") }, func() { ui.Box(c).Fill().Label("after") },
					ImageCompareOptions{Orientation: orientation, AspectRatio: 1, BeforeLabel: "Before", AfterLabel: "After"})
				s.Label("Compare").Width(200)
				if s.Changed() {
					changes++
				}
			}, 300, 300)
			tt.Advance(time.Second)
			before, _ := tt.Find("before")
			after, _ := tt.Find("after")
			if orientation == Horizontal && before.W != 100 || orientation == Vertical && before.H != 100 || after.W != 200 || after.H != 200 {
				t.Fatalf("the divider halves before: %+v %+v", before, after)
			}
			if !tt.HasText("Before") || !tt.HasText("After") {
				t.Fatal("missing captions")
			}
			r, ok := tt.Find("Compare")
			if !ok {
				t.Fatal("missing handle")
			}
			along := func(f float32) (float32, float32) {
				if orientation == Vertical {
					return r.X + r.W/2, r.Y + r.H*f
				}
				return r.X + r.W*f, r.Y + r.H/2
			}
			// A press anywhere moves the divider there.
			x, y := along(.25)
			tt.ClickAt(x, y)
			if position != 25 || changes != 1 {
				t.Fatalf("press: %v changes=%d", position, changes)
			}
			// A drag tracks the pointer and clamps at the edges.
			tt.Press(x, y)
			x, y = along(.75)
			tt.Move(x, y)
			if position != 75 {
				t.Fatalf("drag: %v", position)
			}
			tt.Move(r.X+r.W+50, r.Y+r.H+50)
			if position != 100 {
				t.Fatalf("drag past the edge: %v", position)
			}
			tt.Release(r.X+r.W+50, r.Y+r.H+50)

			back, forth, wrong := ui.KeyLeft, ui.KeyRight, ui.KeyDown
			if orientation == Vertical {
				back, forth, wrong = ui.KeyUp, ui.KeyDown, ui.KeyRight
			}
			tt.Key(0, back)
			if position != 99 {
				t.Fatalf("arrow: %v", position)
			}
			tt.Key(ui.Shift, back)
			if position != 89 {
				t.Fatalf("Shift+arrow: %v", position)
			}
			tt.Key(0, wrong)
			if position != 89 {
				t.Fatal("wrong-axis key moved the divider")
			}
			tt.Key(0, ui.KeyPageDown)
			tt.Key(0, forth)
			if position != 80 {
				t.Fatalf("PageDown and arrow: %v", position)
			}
			tt.Key(0, ui.KeyHome)
			if position != 0 {
				t.Fatalf("Home: %v", position)
			}
			tt.Key(0, ui.KeyEnd)
			if position != 100 {
				t.Fatalf("End: %v", position)
			}
			n := changes
			tt.Key(0, ui.KeyEnd)
			if changes != n {
				t.Fatal("no-op reported a change")
			}

			// Double-clicking the settled handle restores the default, and the
			// held second press does not move it back.
			position = 30
			tt.Frame()
			tt.Advance(time.Second)
			x, y = along(.3)
			tt.ClickAt(x, y)
			tt.Press(x, y)
			tt.Frame()
			if position != 50 {
				t.Fatalf("double-click: %v", position)
			}
			tt.Release(x, y)
			if position != 50 {
				t.Fatalf("release after double-click: %v", position)
			}
		})
	}
}

func TestImageCompareRevealsBefore(t *testing.T) {
	position := float32(150)
	tt := ui.NewTester(func(c *ui.Context) {
		ImageCompare(c, &position, func() { ui.Box(c).Fill().Background(ui.RGB(255, 0, 0)) }, func() { ui.Box(c).Fill().Background(ui.RGB(0, 0, 255)) },
			ImageCompareOptions{AspectRatio: -1}).Size(200, 100)
	}, 200, 100)
	tt.Advance(time.Second)
	if position != 150 {
		t.Fatal("clamping wrote back the position")
	}
	if p := tt.Image().RGBAAt(190, 10); p.R < 200 || p.B > 50 {
		t.Fatalf("an out-of-range position shows before everywhere: %v", p)
	}
	position = 25
	tt.Frame()
	tt.Advance(time.Second)
	if p := tt.Image().RGBAAt(20, 10); p.R < 200 || p.B > 50 {
		t.Fatalf("before left of the divider: %v", p)
	}
	if p := tt.Image().RGBAAt(150, 10); p.B < 200 || p.R > 50 {
		t.Fatalf("after right of the divider: %v", p)
	}
}

func TestImageCompareRejectsInvalidValues(t *testing.T) {
	nan := float32(0)
	nan /= nan
	for name, build := range map[string]func(*ui.Context){
		"nil": func(c *ui.Context) { ImageCompare(c, nil, nil, nil) },
		"nan": func(c *ui.Context) { ImageCompare(c, &nan, nil, nil) },
		"ratio": func(c *ui.Context) {
			p := float32(50)
			ImageCompare(c, &p, nil, nil, ImageCompareOptions{AspectRatio: nan})
		},
		"double": func(c *ui.Context) {
			p := float32(50)
			ImageCompare(c, &p, nil, nil, ImageCompareOptions{}, ImageCompareOptions{})
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			ui.NewTester(build, 100, 100)
		})
	}
}
