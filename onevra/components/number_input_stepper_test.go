package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestNumberInputStepperInnerCorners(t *testing.T) {
	for _, dark := range []bool{false, true} {
		value := 5.0
		var hover, pressed ui.Color
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			hover, pressed = c.Theme().SurfaceHover, c.Theme().SurfacePressed
			NumberInput(c, &value, 0, 10, 1).Label("Amount")
		}, 200, 80)
		for _, label := range []string{"Amount Increase", "Amount Decrease"} {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatal(label)
			}
			x, y := r.X+r.W/2, r.Y+r.H/2
			tt.Move(x, y)
			for _, down := range []bool{false, true} {
				want := hover
				if down {
					tt.Press(x, y)
					want = pressed
				}
				img := tt.Image()
				// Sample both corners adjacent to the separator, outside the icon.
				py := int(r.Y+r.H) - 2
				if label == "Amount Decrease" {
					py = int(r.Y) + 1
				}
				for _, px := range []int{int(r.X) + 1, int(r.X+r.W) - 2} {
					got := img.RGBAAt(px, py)
					if got.R != want.R || got.G != want.G || got.B != want.B {
						t.Fatalf("%v %s pressed=%t: inner corner %v, want %v", dark, label, down, got, want)
					}
				}
			}
			tt.Release(x, y)
		}
	}
}
