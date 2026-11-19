package components

import (
	"bytes"
	"testing"
	"time"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestSwitchToggle(t *testing.T) {
	for _, dark := range []bool{false, true} {
		mode := false
		if dark {
			mode = true
		}
		on := false
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			Switch(c, &on, "Wi-Fi")
		}, 300, 100)
		// Find returns the switch track first: it is built and labeled
		// before the text. Click its center.
		r, ok := tt.Find("Wi-Fi")
		if !ok {
			t.Fatal("switch not found")
		}
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		if !on {
			t.Errorf("dark=%v: click did not turn on", dark)
		}
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		if on {
			t.Errorf("dark=%v: click did not turn off", dark)
		}
	}
}

func TestSwitchKeyboard(t *testing.T) {
	on := false
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Switch(c, &on, "Wi-Fi")
	}, 300, 100)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if !on {
		t.Error("Space did not toggle the focused switch")
	}
}

func TestSwitchHiddenLabel(t *testing.T) {
	on := false
	tt := ui.NewTester(func(c *ui.Context) {
		Switch(c, &on, "Smart notifications", SwitchOptions{HideLabel: true})
	}, 300, 100)
	if len(tt.Texts()) != 1 {
		t.Fatalf("expected only accessible label, got %v", tt.Texts())
	}
	if err := tt.Click("Smart notifications"); err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("named switch did not toggle")
	}
	tt.Key(0, ui.KeySpace)
	if on {
		t.Fatal("keyboard did not toggle hidden-label switch")
	}
}

func TestSwitchDrag(t *testing.T) {
	for _, compact := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			on      bool
			dx, end float32 // pointer travel, then the release's offset from the track
			want    bool
		}{
			{"drag on", false, 14, 0, true},
			{"drag off", true, -14, 0, false},
			{"short drag stays", false, 4, 0, false},
			{"drag back stays", true, -4, 0, true},
			{"release outside", false, 14, 60, true},
			{"jitter clicks", false, 2, 0, true},
		} {
			on := tc.on
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Padding(30).Children(func() {
					Switch(c, &on, "Wi-Fi", SwitchOptions{Compact: compact})
				})
			}, 300, 100)
			r, _ := tt.Find("Wi-Fi")
			x, y := r.X+r.W/2, r.Y+r.H/2
			tt.Press(x, y)
			for i := float32(1); i <= 4; i++ {
				tt.Move(x+tc.dx*i/4, y)
			}
			tt.Move(x+tc.dx, y+tc.end)
			tt.Release(x+tc.dx, y+tc.end)
			tt.Frame()
			if on != tc.want {
				t.Errorf("compact=%v %s: on = %v, want %v", compact, tc.name, on, tc.want)
			}
		}
	}
}

func TestSwitchCompactSize(t *testing.T) {
	var sizes [2]ui.Rect
	for i, compact := range []bool{false, true} {
		on := false
		tt := ui.NewTester(func(c *ui.Context) {
			Switch(c, &on, "Wi-Fi", SwitchOptions{Compact: compact})
		}, 300, 100)
		sizes[i], _ = tt.Find("Wi-Fi")
		if err := tt.Click("Wi-Fi"); err != nil || !on {
			t.Fatalf("compact=%v: click did not toggle", compact)
		}
	}
	if sizes[1].W >= sizes[0].W || sizes[1].H >= sizes[0].H {
		t.Fatalf("compact switch %v is not smaller than %v", sizes[1], sizes[0])
	}
}

func TestSwitchPressRevealsGlass(t *testing.T) {
	for _, dark := range []bool{false, true} {
		on := false
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Padding(30).Children(func() {
				Switch(c, &on, "Wi-Fi")
			})
		}, 300, 100)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.SetDark(dark)
		r, _ := tt.Find("Wi-Fi")
		x, y := r.X+r.W/2, r.Y+r.H/2
		rest := bytes.Clone(tt.Image().Pix)
		tt.Press(x, y)
		tt.Advance(switchTap / 2)
		if !bytes.Equal(rest, tt.Image().Pix) {
			t.Fatalf("dark=%v: a tap-length press showed glass", dark)
		}
		tt.Advance(switchTap / 2)
		if bytes.Equal(rest, tt.Image().Pix) {
			t.Fatalf("dark=%v: holding did not reveal glass", dark)
		}
		// Leaving straight down neither drags nor clicks.
		tt.Move(x, y+60)
		tt.Release(x, y+60)
		tt.Advance(time.Second)
		if on || !bytes.Equal(rest, tt.Image().Pix) {
			t.Fatalf("dark=%v: release did not restore the resting appearance", dark)
		}
	}
}

// A tap slides the knob as a programmatic change does, without glass.
func TestSwitchTapSlidesWithoutGlass(t *testing.T) {
	build := func(on *bool) *ui.Tester {
		return ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Padding(30).Children(func() {
				Switch(c, on, "Wi-Fi", SwitchOptions{HideLabel: true})
			})
		}, 200, 100)
	}
	tapped, set := false, false
	tt, ref := build(&tapped), build(&set)
	r, _ := tt.Find("Wi-Fi")
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(switchTap / 2)
	ref.Advance(switchTap / 2)
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	set = true
	ref.Frame()
	if !tapped {
		t.Fatal("tap did not toggle")
	}
	for i := range 8 {
		if !bytes.Equal(tt.Image().Pix, ref.Image().Pix) {
			t.Fatalf("frame %d differs from a plain slide", i)
		}
		tt.Advance(40 * time.Millisecond)
		ref.Advance(40 * time.Millisecond)
	}
}

// Letting go of a lifted knob without dragging it does not toggle.
func TestSwitchHeldReleaseKeepsState(t *testing.T) {
	for _, held := range []time.Duration{switchTap, switchTap + switchLift, time.Second} {
		on := false
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Padding(30).Children(func() {
				Switch(c, &on, "Wi-Fi", SwitchOptions{HideLabel: true})
			})
		}, 200, 100)
		r, _ := tt.Find("Wi-Fi")
		rest := bytes.Clone(tt.Image().Pix)
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
		tt.Advance(held)
		tt.Release(r.X+r.W/2, r.Y+r.H/2)
		if on {
			t.Fatalf("held %v: release toggled", held)
		}
		tt.Advance(switchLift)
		if !bytes.Equal(rest, tt.Image().Pix) {
			t.Fatalf("held %v: the lens did not set down in place", held)
		}
	}
}

// After a drag, the lens rides the knob to the nearer side and sets down
// only once it arrives. Only the lens rises 4 DIPs past the track, so the
// background there shows whether it is up.
func TestSwitchLiftsLensAcross(t *testing.T) {
	build := func(on *bool) *ui.Tester {
		return ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Padding(30).Children(func() {
				Switch(c, on, "Wi-Fi", SwitchOptions{HideLabel: true})
			})
		}, 200, 100)
	}
	end := true
	settled := build(&end)
	on := false
	tt := build(&on)
	r, _ := tt.Find("Wi-Fi")
	up := func() bool {
		img, y := tt.Image(), int(r.Y-4)
		return img.RGBAAt(int(r.X+r.W-r.H/2), y) != img.RGBAAt(2, y)
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	for i := float32(1); i <= 4; i++ {
		tt.Move(x+2*i, y)
	}
	tt.Release(x+8, y)
	if !on {
		t.Fatal("a drag past the middle did not turn on")
	}
	tt.Advance(170 * time.Millisecond)
	if !up() {
		t.Fatal("the lens landed before the knob arrived")
	}
	tt.Advance(time.Second)
	if !bytes.Equal(tt.Image().Pix, settled.Image().Pix) {
		t.Fatal("the lens did not land on the far side")
	}
}

// Only the knob lifts and drags: a press beside it is a click, however
// long, and pointer travel there does not move the knob.
func TestSwitchTrackPressClicks(t *testing.T) {
	for _, start := range []bool{false, true} {
		on := start
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Padding(30).Children(func() {
				Switch(c, &on, "Wi-Fi", SwitchOptions{HideLabel: true})
			})
		}, 200, 100)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		r, _ := tt.Find("Wi-Fi")
		// The end of the track away from the knob, moving away from it.
		x, dx := r.X+r.W-3, float32(-6)
		if start {
			x, dx = r.X+3, 6
		}
		rest := bytes.Clone(tt.Image().Pix)
		tt.Press(x, r.Y+r.H/2)
		tt.Advance(time.Second)
		tt.Move(x+dx, r.Y+r.H/2)
		if !bytes.Equal(rest, tt.Image().Pix) {
			t.Fatalf("on=%v: pressing beside the knob lifted or moved it", start)
		}
		tt.Release(x+dx, r.Y+r.H/2)
		if on == start {
			t.Fatalf("on=%v: a long press beside the knob did not toggle", start)
		}
	}
}
