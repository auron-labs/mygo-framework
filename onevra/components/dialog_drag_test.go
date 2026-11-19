package components

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestDialogWindowDragStrip(t *testing.T) {
	outer, inner := true, true
	backgroundClicks, outerClicks := 0, 0
	tt := ui.NewTester(func(c *ui.Context) {
		if Button(c, "Background", ButtonPrimary).Clicked() {
			backgroundClicks++
		}
		Dialog(c, &outer, DialogOptions{Inset: 48, WindowDragHeight: 48}, func(p ui.Element) {
			if Button(c, "Outer action", ButtonPrimary).Clicked() {
				outerClicks++
			}
			Dialog(c, &inner, DialogOptions{Inset: 48, WindowDragHeight: 48}, func(p ui.Element) {
				Text(c, "Inner")
			})
		})
	}, 600, 500)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	for _, x := range []float32{2, 300, 598} {
		for _, y := range []float32{2, 24, 47} {
			tt.ClickAt(x, y)
			tt.Press(x, y)
			tt.Move(x+20, y+80)
			tt.Release(x+20, y+80)
			if !outer || !inner || backgroundClicks != 0 || outerClicks != 0 {
				t.Fatalf("drag strip leaked at %g,%g: outer=%v inner=%v background=%d actions=%d", x, y, outer, inner, backgroundClicks, outerClicks)
			}
		}
	}
	tt.ClickAt(2, 60)
	if !outer || inner {
		t.Fatal("backdrop did not dismiss only the top layer")
	}
	tt.Frame()
	tt.ClickAt(300, 24)
	if !outer {
		t.Fatal("outer drag strip dismissed dialog")
	}
	if err := tt.Click("Outer action"); err != nil {
		t.Fatal(err)
	}
	if outerClicks != 1 {
		t.Fatal("panel action not interactive")
	}
	tt.Key(0, ui.KeyEscape)
	if outer {
		t.Fatal("Escape did not dismiss outer dialog")
	}
}

func TestDialogCustomClearance(t *testing.T) {
	for _, inset := range []float32{0, 24, 48, 64} {
		for _, dragHeight := range []float32{0, 48, 64} {
			open := true
			var bounds ui.Rect
			tt := ui.NewTester(func(c *ui.Context) {
				Dialog(c, &open, DialogOptions{Inset: inset, WindowDragHeight: dragHeight}, func(p ui.Element) {
					p.Width(1000).Height(1000).DrawOver(func(_ *ui.Painter, r ui.Rect) { bounds = r })
				})
			}, 360, 480)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			want := inset
			if want <= 0 {
				want = 20
			}
			want = max(want, dragHeight)
			if bounds.X < want-.5 || bounds.Y < want-.5 || bounds.X+bounds.W > 360-want+.5 || bounds.Y+bounds.H > 480-want+.5 {
				t.Fatalf("inset=%g drag=%g: %+v", inset, dragHeight, bounds)
			}
		}
	}
}
