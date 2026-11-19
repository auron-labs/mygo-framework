package components

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestIconButtonPlain(t *testing.T) {
	for _, dark := range []bool{false, true} {
		clicks := 0
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			c.Root().Background(ui.Transparent)
			ui.Row(c).Fill().Children(func() {
				if IconButton(c, lucide.Settings(), "Settings", IconButtonPlain).Clicked() {
					clicks++
				}
			})
		}, 200, 80)
		r, ok := tt.Find("Settings")
		if !ok || r.W != 32 || r.H != 32 {
			t.Fatalf("missing square icon button: %+v", r)
		}
		sample := func() uint8 { return tt.Image().RGBAAt(int(r.X+4), int(r.Y+r.H/2)).A }
		tt.Move(190, 70)
		if sample() != 0 {
			t.Fatal("resting button has a face")
		}
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		hover := sample()
		if hover == 0 || hover == 255 {
			t.Fatalf("hover must be a translucent tint: %d", hover)
		}
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		if sample() <= hover {
			t.Fatal("press must be a stronger translucent tint")
		}
		tt.Release(r.X+r.W/2, r.Y+r.H/2)
		if clicks != 1 {
			t.Fatalf("clicks=%d", clicks)
		}
	}
}

// TestIconButtonToolbarFace checks that the toolbar style keeps ButtonToolbar's
// raised, shadowed face instead of the plain tinted one.
func TestIconButtonToolbarFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		face, _, _, _ := toolbarFace(theme.Palette(dark))
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(dark))
			IconButton(c, lucide.Settings(), "Settings", IconButtonToolbar)
		}, 200, 80)
		r, ok := tt.Find("Settings")
		if !ok || r.W != 32 || r.H != 32 {
			t.Fatalf("missing square icon button: %+v", r)
		}
		// Inside the horizontal padding, clear of the glyph and border.
		p := tt.Image().RGBAAt(int(r.X+4), int(r.Y+r.H/2))
		if p.R != face.R || p.G != face.G || p.B != face.B {
			t.Fatalf("dark=%v: face %v, want %v", dark, p, face)
		}
	}
}
