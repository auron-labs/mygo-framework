package components

import (
	"fmt"
	"math"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestSplitInteraction(t *testing.T) {
	for _, orientation := range []Orientation{Horizontal, Vertical} {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprint(orientation, "/", dark), func(t *testing.T) {
				size, changes := float32(100), 0
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme.Palette(dark))
					s := Split(c, &size, func() { ui.Box(c).Fill().Label("first") }, func() { ui.Box(c).Fill().Label("second") }, SplitOptions{Orientation: orientation})
					s.Label("Resize").Fill()
					if s.Changed() {
						changes++
					}
				}, 300, 300)
				a, _ := tt.Find("first")
				b, _ := tt.Find("second")
				if orientation == Horizontal && (a.W != 100 || b.X != 101 || b.W != 199) || orientation == Vertical && (a.H != 100 || b.Y != 101 || b.H != 199) {
					t.Fatalf("pane geometry: %+v %+v", a, b)
				}
				r, ok := tt.Find("Resize")
				if !ok {
					t.Fatal("missing divider")
				}
				x, y := r.X+r.W/2, r.Y+r.H/2
				before := tt.Image().RGBAAt(int(x), int(y))
				tt.Move(x, y)
				if tt.Image().RGBAAt(int(x), int(y)) != before {
					t.Fatal("hover changed line color")
				}
				cursor := ui.CursorResizeEW
				key, wrong := ui.KeyRight, ui.KeyDown
				dx, dy := float32(30), float32(0)
				if orientation == Vertical {
					cursor, key, wrong = ui.CursorResizeNS, ui.KeyDown, ui.KeyRight
					dx, dy = 0, 30
				}
				if tt.Cursor() != cursor {
					t.Fatal("wrong resize cursor")
				}
				// Press outside the visible line but within its transparent hit area.
				hx, hy := x, y
				if orientation == Horizontal {
					hx -= 2
				} else {
					hy -= 2
				}
				tt.Press(hx, hy)
				tt.Move(hx+dx, hy+dy)
				if size != 130 || changes != 1 {
					t.Fatalf("drag: size=%v changes=%d", size, changes)
				}
				if tt.Image().RGBAAt(int(x+dx), int(y+dy)) != before {
					t.Fatal("drag changed line color")
				}
				tt.Release(hx+dx, hy+dy)
				tt.Key(0, wrong)
				if size != 130 {
					t.Fatal("wrong-axis key resized panes")
				}
				tt.Key(0, key)
				if size != 140 || changes != 2 {
					t.Fatalf("keyboard: %v, %d", size, changes)
				}
				tt.Key(0, ui.KeyEnd)
				if size != 259 {
					t.Fatalf("End: %v", size)
				}
				tt.Key(0, ui.KeyHome)
				if size != 40 {
					t.Fatalf("Home: %v", size)
				}
				n := changes
				tt.Key(0, ui.KeyHome)
				if changes != n {
					t.Fatal("no-op reported change")
				}
			})
		}
	}
}

func TestSplitDefault(t *testing.T) {
	size, changes := float32(100), 0
	tt := ui.NewTester(func(c *ui.Context) {
		s := Split(c, &size, nil, nil, SplitOptions{Default: 400})
		s.Label("Resize").Fill()
		if s.Changed() {
			changes++
		}
	}, 300, 300)
	r, _ := tt.Find("Resize")
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.ClickAt(x, y)
	if size != 100 || changes != 0 {
		t.Fatalf("single click: %v, %d", size, changes)
	}
	// The default is clamped to the limits, as dragging would be.
	tt.ClickAt(x, y)
	if size != 259 || changes != 1 {
		t.Fatalf("double click: %v, %d", size, changes)
	}
}

func TestSplitConstraintsAndOrientation(t *testing.T) {
	size := float32(170)
	width := float32(300)
	orientation := Horizontal
	changes := 0
	tt := ui.NewTester(func(c *ui.Context) {
		s := Split(c, &size, func() { ui.Box(c).Fill().Label("first") }, nil, SplitOptions{Orientation: orientation, FirstMin: 60, SecondMin: 30})
		s.Width(width).Height(240)
		if s.Changed() {
			changes++
		}
	}, 400, 300)
	width = 61
	tt.Move(390, 290)
	a, _ := tt.Find("first")
	if a.W != 40 || size != 170 || changes != 0 {
		t.Fatalf("small container: %+v %v %d", a, size, changes)
	}
	width = 300
	tt.Move(391, 290)
	a, _ = tt.Find("first")
	if a.W != 170 {
		t.Fatalf("preferred size not restored: %+v", a)
	}
	orientation = Vertical
	tt.Move(392, 290)
	a, _ = tt.Find("first")
	if a.H != 170 || a.W != 300 {
		t.Fatalf("orientation switch: %+v", a)
	}
}

func TestSplitDisabled(t *testing.T) {
	size := float32(100)
	changes := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Disabled(true).Children(func() {
			s := Split(c, &size, nil, nil)
			s.AutoFocus().Fill()
			if s.Changed() {
				changes++
			}
		})
	}, 300, 300)
	tt.Press(100, 150)
	tt.Move(200, 150)
	tt.Release(200, 150)
	tt.Key(0, ui.KeyRight)
	if size != 100 || changes != 0 {
		t.Fatalf("disabled: %v %d", size, changes)
	}
}

func TestSplitFocusAndCapture(t *testing.T) {
	for _, orientation := range []Orientation{Horizontal, Vertical} {
		size := float32(100)
		var focused, visible bool
		tt := ui.NewTester(func(c *ui.Context) {
			s := Split(c, &size, func() { Button(c, "Before", ButtonOutline) }, func() { Button(c, "After", ButtonOutline) }, SplitOptions{Orientation: orientation})
			s.Fill().Padding(20)
			focused, visible = s.divider.Focused(), s.divider.FocusVisible()
		}, 300, 300)
		tt.Key(0, ui.KeyTab)
		tt.Key(0, ui.KeyTab)
		if !focused || !visible {
			t.Fatal("divider missing from pane tab order or missing focus indication")
		}
		tt.Key(0, ui.KeyTab)
		if focused {
			t.Fatal("Tab did not leave divider")
		}
		r, _ := tt.Find("Divider")
		x, y := r.X+r.W/2, r.Y+r.H/2
		if orientation == Horizontal && (r.X != 120 || r.Y != 20 || r.H != 260) || orientation == Vertical && (r.Y != 120 || r.X != 20 || r.W != 260) {
			t.Fatalf("padding displaced handle: %+v", r)
		}
		tt.Press(x, y)
		if orientation == Horizontal {
			x = 500
		} else {
			y = 500
		}
		tt.Move(x, y)
		if size != 219 {
			t.Fatalf("capture beyond pane: %v", size)
		}
		tt.Release(x, y)
		tt.Move(0, 0)
		if size != 219 {
			t.Fatal("capture retained after release")
		}
	}
}

func TestSplitInvalidInputs(t *testing.T) {
	valid, nan, inf := float32(100), float32(math.NaN()), float32(math.Inf(1))
	for _, test := range []struct {
		size    *float32
		options []SplitOptions
	}{
		{nil, nil},
		{&nan, nil},
		{&inf, nil},
		{&valid, []SplitOptions{{FirstMin: -1}}},
		{&valid, []SplitOptions{{SecondMin: nan}}},
		{&valid, []SplitOptions{{FirstMin: inf}}},
		{&valid, []SplitOptions{{}, {}}},
		{&valid, []SplitOptions{{Default: -1}}},
		{&valid, []SplitOptions{{Default: inf}}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid input accepted")
				}
			}()
			ui.NewTester(func(c *ui.Context) { Split(c, test.size, nil, nil, test.options...).Fill() }, 300, 300)
		}()
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid percentage input accepted")
				}
			}()
			ui.NewTester(func(c *ui.Context) { SplitPercent(c, test.size, nil, nil, test.options...).Fill() }, 300, 300)
		}()
	}
}

func TestSplitPercent(t *testing.T) {
	percent, width := float32(25), float32(301)
	orientation := Horizontal
	changes := 0
	tt := ui.NewTester(func(c *ui.Context) {
		s := SplitPercent(c, &percent, func() { ui.Box(c).Fill().Label("first") }, nil, SplitOptions{Orientation: orientation, FirstMin: 60, SecondMin: 30, Default: 40})
		s.Label("Resize").Width(width).Height(201)
		if s.Changed() {
			changes++
		}
	}, 700, 400)
	a, _ := tt.Find("first")
	if a.W != 75 {
		t.Fatalf("share of 300: %+v", a)
	}
	// Both panes follow the room in proportion, without writing the share.
	width = 601
	tt.Move(690, 390)
	if a, _ = tt.Find("first"); a.W != 150 || percent != 25 || changes != 0 {
		t.Fatalf("share of 600: %+v %v %d", a, percent, changes)
	}
	// The minimum applies without replacing the preferred share.
	width = 201
	tt.Move(691, 390)
	if a, _ = tt.Find("first"); a.W != 60 || percent != 25 {
		t.Fatalf("minimum: %+v %v", a, percent)
	}
	width = 601
	tt.Move(692, 390)
	r, _ := tt.Find("Resize")
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x+150, y)
	tt.Release(x+150, y)
	if percent != 50 || changes != 1 {
		t.Fatalf("drag: %v %d", percent, changes)
	}
	tt.Key(0, ui.KeyEnd)
	if percent != 95 {
		t.Fatalf("End: %v", percent)
	}
	// Double-clicking the divider restores the default share.
	r, _ = tt.Find("Resize")
	x, y = r.X+r.W/2, r.Y+r.H/2
	tt.ClickAt(x, y)
	tt.ClickAt(x, y)
	if percent != 40 || changes != 3 {
		t.Fatalf("double click: %v %d", percent, changes)
	}
	orientation = Vertical
	tt.Key(0, ui.KeyHome)
	near := func(a, b float32) bool { return math.Abs(float64(a-b)) < .01 }
	if a, _ = tt.Find("first"); !near(percent, 30) || !near(a.H, 60) || a.W != 601 {
		t.Fatalf("vertical: %+v %v", a, percent)
	}
}

func TestSplitLimits(t *testing.T) {
	for _, total := range []float32{0, 1, 20, 81, 300} {
		lo, hi := splitLimits(total, 40, 40)
		if lo < 0 || hi < lo || hi > max(float32(0), total-1) {
			t.Fatalf("invalid limits: %v %v %v", total, lo, hi)
		}
	}
}
