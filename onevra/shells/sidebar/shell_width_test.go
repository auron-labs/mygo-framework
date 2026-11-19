package sidebar

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShellWidthOptions(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		options                ShellOptions
		wantDefault, wantMin   int
		wantMax, wantDisplayed int
	}{
		{"defaults", ShellOptions{}, 220, 120, 0, 220},
		{"custom", ShellOptions{LeftWidth: 240, LeftMinWidth: 180, LeftMaxWidth: 300}, 240, 180, 300, 240},
		{"below minimum", ShellOptions{LeftWidth: 100, LeftMinWidth: 180}, 180, 180, 0, 180},
		{"above maximum", ShellOptions{LeftWidth: 400, LeftMaxWidth: 300}, 300, 120, 300, 300},
		{"inverted limits", ShellOptions{LeftMinWidth: 250, LeftMaxWidth: 150}, 250, 250, 250, 250},
		{"negative", ShellOptions{LeftWidth: -1, LeftMinWidth: -1, LeftMaxWidth: -1}, 220, 120, -1, 220},
		{"above forty percent", ShellOptions{LeftWidth: 600, LeftMinWidth: 500, LeftMaxWidth: 700}, 600, 500, 700, 600},
		{"viewport limit", ShellOptions{LeftWidth: 1200, LeftMinWidth: 1100}, 1200, 1100, 0, 997},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newShell(tc.options)
			if s.options.LeftWidth != tc.wantDefault || s.options.LeftMinWidth != tc.wantMin || s.options.LeftMaxWidth != tc.wantMax || s.leftWidth != float32(tc.wantDefault) {
				t.Fatalf("unexpected normalized options: %+v, width %v", s.options, s.leftWidth)
			}
			tt := ui.NewTester(s.view, 1000, 600)
			r, ok := tt.Find("Resize sidebar")
			if !ok || r.X+r.W/2 != float32(tc.wantDisplayed) {
				t.Fatal("incorrect initial sidebar width", r)
			}
		})
	}
}

func TestShellResizeBeyondFortyPercent(t *testing.T) {
	for _, maximum := range []int{0, 700} {
		t.Run(map[int]string{0: "unbounded", 700: "configured maximum"}[maximum], func(t *testing.T) {
			var content ui.Rect
			s := newShell(ShellOptions{
				LeftMaxWidth: maximum,
				Content:      func(c *ui.Context) { ui.Box(c).Fill().DrawOver(func(_ *ui.Painter, r ui.Rect) { content = r }) },
			})
			tt := ui.NewTester(s.view, 1000, 600)
			r, ok := tt.Find("Resize sidebar")
			if !ok {
				t.Fatal("missing resize handle")
			}
			x, y := r.X+r.W/2, r.Y+30
			tt.Press(x, y)
			tt.Move(600, y)
			tt.Release(600, y)
			if s.leftWidth != 600 || content.X != 600 || content.W != 400 {
				t.Fatalf("drag stopped early: width=%v, content=%+v", s.leftWidth, content)
			}
			tt.Key(0, ui.KeyRight)
			if s.leftWidth != 610 || content.W != 390 {
				t.Fatalf("keyboard stopped early: width=%v, content=%+v", s.leftWidth, content)
			}
		})
	}
}

func TestShellWidthLimitsAndReset(t *testing.T) {
	s := newShell(ShellOptions{LeftWidth: 240, LeftMinWidth: 180, LeftMaxWidth: 300})
	tt := ui.NewTester(s.view, 1000, 600)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	handle := func() ui.Rect {
		t.Helper()
		r, ok := tt.Find("Resize sidebar")
		if !ok {
			t.Fatal("missing resize handle")
		}
		return r
	}
	assertWidth := func(stored, visible float32) {
		t.Helper()
		r := handle()
		if s.leftWidth != stored || r.X+r.W/2 != visible {
			t.Fatalf("width=%v, visible=%v; want %v, %v", s.leftWidth, r.X+r.W/2, stored, visible)
		}
	}
	drag := func(dx float32) {
		r := handle()
		x, y := r.X+r.W/2, r.Y+30
		tt.Press(x, y)
		tt.Move(x+dx, y)
		tt.Release(x+dx, y)
	}
	// Separate click positions from earlier presses, so each pair is a new
	// double click rather than part of MyGo's multi-click sequence.
	reset := func(y float32) {
		r := handle()
		x := r.X + r.W/2
		tt.ClickAt(x, y)
		tt.ClickAt(x, y)
	}
	drag(1000)
	assertWidth(300, 300)
	tt.Key(0, ui.KeyRight)
	assertWidth(300, 300)
	drag(-1000)
	assertWidth(180, 180)
	tt.Key(0, ui.KeyLeft)
	assertWidth(180, 180)
	tt.Key(0, ui.KeyRight)
	assertWidth(190, 190)
	// A single click and secondary clicks must not reset the width.
	r := handle()
	tt.ClickAt(r.X+r.W/2, 150)
	assertWidth(190, 190)
	tt.RightClickAt(r.X+r.W/2, 180)
	tt.RightClickAt(r.X+r.W/2, 180)
	assertWidth(190, 190)
	reset(220)
	assertWidth(240, 240)

	drag(50)
	assertWidth(290, 290)
	s.SetLeftExpanded(false)
	tt.Frame()
	s.SetLeftExpanded(true)
	tt.Frame()
	s.SetPage(func(*ui.Context) {})
	tt.Frame()
	s.SetPage(nil)
	tt.Frame()
	assertWidth(290, 290)

	// A constrained viewport does not erase the chosen or default width.
	tt.SetSize(146, 600)
	assertWidth(290, 143)
	reset(260)
	assertWidth(240, 143)
	tt.SetSize(1000, 600)
	assertWidth(240, 240)
	tt.SetSize(146, 600)
	drag(-50)
	assertWidth(180, 143)
	tt.SetSize(1000, 600)
	assertWidth(180, 180)
	reset(300)
	assertWidth(240, 240)
}
