package components

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/auron-labs/mygo-framework/onevra/theme"
	"github.com/egoist/mygo/ui"
)

func TestSpinnerRendering(t *testing.T) {
	for _, variant := range []SpinnerVariant{SpinnerRing, SpinnerSpokes, SpinnerVariant(99)} {
		for _, dark := range []bool{false, true} {
			for _, scale := range []float32{1, 1.5} {
				t.Run(fmt.Sprint(variant, "/", dark, "/", scale), func(t *testing.T) {
					var diameter float32
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(theme.Palette(dark))
						diameter = c.Theme().Space(8)
						ui.Column(c).Fill().Children(func() {
							Spinner(c, variant).Label("default")
							Spinner(c, variant).Size(80, 40).Label("wide")
						})
					}, 200, 150)
					tt.SetPreferences(ui.Preferences{TextScale: scale, ReduceMotion: true})
					if r, ok := tt.Find("default"); !ok || r.W != diameter || r.H != diameter {
						t.Fatalf("default bounds = %+v, want diameter %v", r, diameter)
					}
					if r, ok := tt.Find("wide"); !ok || r.W != 80 || r.H != 40 {
						t.Fatalf("custom bounds = %+v", r)
					}
					before := append([]byte(nil), tt.Image().Pix...)
					tt.Frame()
					if !bytes.Equal(before, tt.Image().Pix) {
						t.Fatal("reduced-motion rendering changed between frames")
					}
				})
			}
		}
	}
}

func TestSpinnerSpokesTextColor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprint(dark), func(t *testing.T) {
			var tint *ui.Color
			var muted ui.Color
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.Palette(dark))
				muted = c.Theme().TextMuted
				e := Spinner(c, SpinnerSpokes).Size(80, 40).Label("loading")
				if tint != nil {
					e.TextColor(*tint)
				}
			}, 100, 60)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			before := append([]byte(nil), tt.Image().Pix...)
			tint = &muted
			tt.Frame()
			if !bytes.Equal(before, tt.Image().Pix) {
				t.Fatal("default spokes do not use the theme's muted text color")
			}
			for _, color := range []ui.Color{ui.RGB(255, 0, 0), ui.RGB(0, 255, 0)} {
				tint = &color
				tt.Frame()
				r, ok := tt.Find("loading")
				if !ok {
					t.Fatal("spinner lost its accessible label")
				}
				bias := 0
				img := tt.Image()
				for y := int(r.Y); y < int(r.Y+r.H); y++ {
					for x := int(r.X); x < int(r.X+r.W); x++ {
						px := img.RGBAAt(x, y)
						delta := int(px.R) - int(px.G)
						if color.G > color.R {
							delta = -delta
						}
						bias = max(bias, delta)
					}
				}
				if bias < 60 {
					t.Fatalf("spokes ignored TextColor(%v): channel bias = %d", color, bias)
				}
			}
			tint = nil
			tt.Frame()
			if !bytes.Equal(before, tt.Image().Pix) {
				t.Fatal("removing TextColor did not restore muted spokes")
			}
		})
	}
}

func TestSpinnerSpokesCentered(t *testing.T) {
	render := func(width, height float32) *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			Spinner(c, SpinnerSpokes).Size(width, height)
		}, 100, 100)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		return tt
	}
	want := render(40, 40).Image()
	background := want.RGBAAt(99, 99)
	for _, size := range [][2]int{{80, 40}, {40, 80}} {
		got := render(float32(size[0]), float32(size[1])).Image()
		dx, dy := (size[0]-40)/2, (size[1]-40)/2
		for y := range 100 {
			for x := range 100 {
				pixel := background
				if x >= dx && x < dx+40 && y >= dy && y < dy+40 {
					pixel = want.RGBAAt(x-dx, y-dy)
				}
				if got.RGBAAt(x, y) != pixel {
					t.Fatalf("%v spinner is not circular and centered at pixel %d,%d", size, x, y)
				}
			}
		}
	}
}

func TestSpinnerRingMatchesProgressRing(t *testing.T) {
	render := func(spinner bool) []byte {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(theme.LightPalette())
			if spinner {
				Spinner(c, SpinnerRing).Size(80, 40)
			} else {
				ProgressRing(c, -1).Size(80, 40)
			}
		}, 100, 60)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		return append([]byte(nil), tt.Image().Pix...)
	}
	if !bytes.Equal(render(true), render(false)) {
		t.Fatal("ring spinner differs from indeterminate ProgressRing")
	}
}
