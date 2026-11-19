package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestToggleButton(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			on, changes := false, 0
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				ui.Column(c).Disabled(disabled).Children(func() {
					if ToggleButton(c, &on, "Bold").Changed() {
						changes++
					}
				})
			}, 300, 100)
			_ = tt.Click("Bold")
			if on != !disabled {
				t.Fatalf("click: on=%v disabled=%v", on, disabled)
			}
			_ = tt.Click("Bold")
			if on {
				t.Fatal("second click did not turn off")
			}
			tt.Key(0, ui.KeyTab)
			tt.Key(0, ui.KeySpace)
			if on != !disabled {
				t.Fatalf("Space: on=%v disabled=%v", on, disabled)
			}
			want := 3
			if disabled {
				want = 0
			}
			if changes != want {
				t.Fatalf("changes=%d want=%d", changes, want)
			}
		}
	}
}
