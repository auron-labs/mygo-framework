package sidebar

import (
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/icons/lucide"
	"github.com/egoist/mygo/ui"
)

// TestIconButtonTouchTarget checks that a coarse pointer's button takes the
// touch minimum; the fine pointer's 32 is what TestIconButton asserts.
func TestIconButtonTouchTarget(t *testing.T) {
	tt := ui.NewTesterWith(func(c *ui.Context) {
		ui.Row(c).Fill().Children(func() {
			IconButton(c, lucide.Settings(), "Settings")
		})
	}, 200, 100, ui.TesterOptions{Preferences: ui.Preferences{
		TextScale: 1, Pointer: ui.PointerCoarse, AnyPointer: ui.PointerCoarse,
		NoHover: true, NoAnyHover: true,
	}})
	r, ok := tt.Find("Settings")
	if !ok || r.W < 44 || r.H < 44 {
		t.Fatalf("hit target %+v, want at least a 44×44 square", r)
	}
}

func TestIconButton(t *testing.T) {
	for _, dark := range []bool{false, true} {
		clicks := 0
		disabled := false
		var buttonBounds ui.Rect
		var size float32
		tt := ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.Transparent)
			size = max(32, c.Theme().Rem(2))
			ui.Row(c).Fill().DragWindow().Cursor(ui.CursorText).Children(func() {
				button := IconButton(c, lucide.Settings(), "Settings").Disabled(disabled).
					DrawOver(func(_ *ui.Painter, r ui.Rect) { buttonBounds = r })
				if button.Clicked() {
					clicks++
				}
			})
		}, 200, 80)
		tt.SetDark(dark)
		r, ok := tt.Find("Settings")
		if !ok || r.W != 32 || r.H != 32 {
			t.Fatal("missing accessible square button", r)
		}
		alpha := func() uint8 { return tt.Image().RGBAAt(5, 40).A }
		tt.Move(150, 50)
		if alpha() != 0 {
			t.Fatal("resting button has a face")
		}
		tt.Move(5, 40)
		if tt.Cursor() != ui.CursorDefault {
			t.Fatal("icon button inherited the titlebar cursor")
		}
		hover := alpha()
		if hover == 0 || hover == 255 {
			t.Fatal("hover must be translucent", hover)
		}
		tt.Press(5, 40)
		if alpha() <= hover || alpha() == 255 {
			t.Fatal("press must be a stronger translucent tint", alpha())
		}
		tt.Release(5, 40)
		if clicks != 1 {
			t.Fatal("click lost to draggable titlebar", clicks)
		}
		tt.Key(0, ui.KeyEnter)
		tt.Key(0, ui.KeySpace)
		if clicks != 3 {
			t.Fatal("keyboard activation failed", clicks)
		}
		disabled = true
		tt.Frame()
		if err := tt.Click("Settings"); err != nil {
			t.Fatal(err)
		}
		tt.Key(0, ui.KeyEnter)
		if clicks != 3 || alpha() != 0 {
			t.Fatal("disabled button still reacts", clicks, alpha())
		}
		tt.SetPreferences(ui.Preferences{TextScale: 1.5})
		if r := buttonBounds; r.W != size || r.H != size {
			t.Fatal("button does not scale", r)
		}
	}
}
