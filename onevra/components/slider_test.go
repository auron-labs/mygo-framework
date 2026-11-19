package components

import (
	"fmt"
	"math"
	"testing"

	"github.com/auron-labs/mygo-framework/v2/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestSliderModes(t *testing.T) {
	for _, mode := range []bool{false, true} {
		for _, step := range []float64{0, 10} {
			t.Run(fmt.Sprint(mode, "/", step), func(t *testing.T) {
				low, high := 20.0, 80.0
				changes := 0
				tt := ui.NewTester(func(c *ui.Context) {
					c.SetTheme(theme.Palette(mode))
					s := Slider(c, &low, 0, 100, SliderOptions{Range: &high, Step: step})
					s.Label("Price").AutoFocus().Width(240)
					if s.Changed() {
						changes++
					}
				}, 300, 100)
				delta := step
				if delta == 0 {
					delta = 1
				}
				tt.Key(0, ui.KeyRight)
				if low != 20+delta || high != 80 || changes != 1 {
					t.Fatalf("right: %v %v changes=%d", low, high, changes)
				}
				tt.Key(0, ui.KeyEnd)
				if low != 80 {
					t.Fatalf("minimum crossed maximum: %v", low)
				}
				tt.Key(0, ui.KeyRight)
				if changes != 2 {
					t.Fatalf("no-op reported a change: %d", changes)
				}
				tt.Key(0, ui.KeyHome)
				tt.Key(0, ui.KeyTab)
				tt.Key(0, ui.KeyLeft)
				if high != 80-delta || low != 0 {
					t.Fatalf("maximum keyboard: %v %v", low, high)
				}
				tt.Key(0, ui.KeyHome)
				if high != 0 {
					t.Fatalf("maximum Home = %v", high)
				}
				tt.Key(0, ui.KeyEnd)
				if high != 100 {
					t.Fatalf("maximum End = %v", high)
				}
			})
		}
	}
}

func TestSliderRangePointer(t *testing.T) {
	low, high := 20.0, 80.0
	tt := ui.NewTester(func(c *ui.Context) {
		Slider(c, &low, 0, 100, SliderOptions{Range: &high, Step: 10}).Label("Price").Width(240)
	}, 300, 100)
	r, ok := tt.Find("Price")
	if !ok {
		t.Fatal("range missing")
	}
	tt.Press(r.X+r.W*0.35, r.Y+10)
	tt.Release(r.X+r.W*0.35, r.Y+10)
	if low != 30 || high != 80 {
		t.Fatalf("nearest minimum: %v %v", low, high)
	}
	tt.Press(r.X+r.W*0.7, r.Y+10)
	tt.Move(r.X+r.W+50, r.Y+10)
	tt.Release(r.X+r.W+50, r.Y+10)
	if low != 30 || high != 100 {
		t.Fatalf("captured maximum drag: %v %v", low, high)
	}
	knob, ok := tt.Find("Price minimum")
	if !ok {
		t.Fatal("minimum missing")
	}
	tt.Press(knob.X+knob.W/2, knob.Y+knob.H/2)
	tt.Move(r.X-50, r.Y+10)
	tt.Release(r.X-50, r.Y+10)
	if low != 0 || high != 100 {
		t.Fatalf("thumb drag: %v %v", low, high)
	}
}

func TestSliderGlassTracksCapturedThumb(t *testing.T) {
	for _, ranged := range []bool{false, true} {
		low, high := 20.0, 80.0
		var slider *SliderControl
		tt := ui.NewTester(func(c *ui.Context) {
			opts := SliderOptions{}
			if ranged {
				opts.Range = &high
			}
			slider = Slider(c, &low, 0, 100, opts)
			slider.Label("Glass").Width(240)
		}, 300, 100)
		r, _ := tt.Find("Glass")
		tt.Press(r.X+r.W*0.7, r.Y+10)
		active := 0
		if ranged {
			active = 1
		}
		if !slider.held[active] || slider.held[1-active] {
			t.Fatalf("range=%v: wrong held thumb: %v", ranged, slider.held)
		}
		tt.Move(r.X+r.W+30, r.Y+10)
		if !slider.held[active] {
			t.Fatalf("range=%v: glass lost during captured drag", ranged)
		}
		tt.Release(r.X+r.W+30, r.Y+10)
		if slider.held != [2]bool{} {
			t.Fatalf("range=%v: glass stayed held after release", ranged)
		}
	}
}

func TestSliderSteppedSingle(t *testing.T) {
	value := 0.1
	tt := ui.NewTester(func(c *ui.Context) {
		Slider(c, &value, 0.1, 1, SliderOptions{Step: 0.2}).AutoFocus()
	}, 300, 100)
	tt.Key(0, ui.KeyRight)
	if math.Abs(value-0.3) > 1e-12 {
		t.Fatalf("step: %v", value)
	}
	tt.Key(0, ui.KeyEnd)
	if math.Abs(value-0.9) > 1e-12 {
		t.Fatalf("nondivisible End: %v", value)
	}
}

func TestSliderRangeDisabledAndNormalization(t *testing.T) {
	low, high := 120.0, -10.0
	changes := 0
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Disabled(true).Children(func() {
			s := Slider(c, &low, 0, 100, SliderOptions{Range: &high})
			s.Label("Disabled").AutoFocus().Width(240)
			if s.Changed() {
				changes++
			}
		})
	}, 300, 100)
	tt.Key(0, ui.KeyRight)
	r, _ := tt.Find("Disabled")
	tt.Press(r.X+r.W/2, r.Y+10)
	tt.Release(r.X+r.W/2, r.Y+10)
	if low != 0 || high != 100 || changes != 0 {
		t.Fatalf("disabled/normalization: %v %v %d", low, high, changes)
	}
}

func TestSliderKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		mode := false
		if dark {
			mode = true
		}
		value := 50.0
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.Palette(mode))
			Slider(c, &value, 0, 100).Label("volume").AutoFocus()
		}, 300, 100)
		tt.Key(0, ui.KeyRight)
		if value != 51 {
			t.Errorf("dark=%v: Right set %v, want 51", dark, value)
		}
		tt.Key(0, ui.KeyLeft)
		tt.Key(0, ui.KeyLeft)
		if value != 49 {
			t.Errorf("dark=%v: Left twice set %v, want 49", dark, value)
		}
		tt.Key(0, ui.KeyHome)
		if value != 0 {
			t.Errorf("dark=%v: Home set %v, want 0", dark, value)
		}
		tt.Key(0, ui.KeyEnd)
		if value != 100 {
			t.Errorf("dark=%v: End set %v, want 100", dark, value)
		}
	}
}

func TestSliderDrag(t *testing.T) {
	value := 0.0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.LightPalette())
		Slider(c, &value, 0, 100).Label("volume")
	}, 300, 100)
	r, ok := tt.Find("volume")
	if !ok {
		t.Fatal("slider not found")
	}
	// Drag from the left end to the right end of the slider.
	tt.Press(r.X+4, r.Y+r.H/2)
	tt.Move(r.X+r.W-4, r.Y+r.H/2)
	tt.Release(r.X+r.W-4, r.Y+r.H/2)
	if value < 90 {
		t.Errorf("drag set %v, want near 100", value)
	}
}

// A finger drags a slider on a phone's touch screen, and the raw touches a
// Linux or Windows touchscreen sends drag it too. A touchpad's scrolling,
// which reports no touches, moves nothing.
func TestSliderFingerDrag(t *testing.T) {
	value := 20.0
	changed := false
	var slider *SliderControl
	tt := ui.NewTester(func(c *ui.Context) {
		slider = Slider(c, &value, 0, 100)
		slider.Label("volume").Width(240)
		if slider.Changed() {
			changed = true
		}
	}, 300, 100)
	tt.Frame()
	r, ok := tt.Find("volume")
	if !ok {
		t.Fatal("slider not found")
	}
	tt.FingerDown(r.X+r.W*0.2, r.Y+r.H/2)
	tt.FingerMove(r.X+r.W*0.5, r.Y+r.H/2)
	tt.FingerUp(r.X+r.W*0.5, r.Y+r.H/2)
	if value != 50 {
		t.Fatalf("finger drag set %v, want 50", value)
	}
	if !changed {
		t.Fatal("finger drag reported no change")
	}
	changed, before := false, value
	tt.Scroll(r.X+r.W*0.5, r.Y+r.H/2, -50, 0)
	if value != before || changed {
		t.Fatalf("touchpad scroll set %v (changed %v), want no change", value, changed)
	}
	tt.TouchDown(1, r.X+r.W*0.2, r.Y+r.H/2)
	tt.Frame()
	for _, f := range []float32{0.3, 0.4, 0.5} {
		tt.TouchMove(1, r.X+r.W*f, r.Y+r.H/2)
		tt.Frame()
	}
	tt.TouchUp(1, r.X+r.W*0.5, r.Y+r.H/2)
	if value <= before {
		t.Fatalf("touchscreen drag set %v, want more than %v", value, before)
	}
	if !changed {
		t.Fatal("touchscreen drag reported no change")
	}
}

func TestSliderRangeFingerDrag(t *testing.T) {
	low, high := 20.0, 80.0
	tt := ui.NewTester(func(c *ui.Context) {
		Slider(c, &low, 0, 100, SliderOptions{Range: &high, Step: 10}).Label("Price").Width(240)
	}, 300, 100)
	tt.Frame()
	r, ok := tt.Find("Price")
	if !ok {
		t.Fatal("slider not found")
	}
	knob, ok := tt.Find("Price minimum")
	if !ok {
		t.Fatal("minimum thumb not found")
	}
	tt.FingerDown(knob.X+knob.W/2, knob.Y+knob.H/2)
	tt.FingerMove(r.X+r.W*0.5, r.Y+r.H/2)
	tt.FingerUp(r.X+r.W*0.5, r.Y+r.H/2)
	if low != 50 || high != 80 {
		t.Fatalf("finger drag set %v %v, want 50 80", low, high)
	}
	before := high
	tt.TouchDown(1, r.X+r.W*0.85, r.Y+r.H/2)
	tt.Frame()
	tt.TouchMove(1, r.X+r.W*0.95, r.Y+r.H/2)
	tt.Frame()
	tt.TouchUp(1, r.X+r.W*0.95, r.Y+r.H/2)
	if high <= before || low != 50 {
		t.Fatalf("touchscreen drag set %v %v, want the maximum above %v", low, high, before)
	}
}
