package components

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

func TestKnobUsesClearOnlyWhileHeld(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, plane := range []bool{false, true} {
			held := float32(0)
			k := ui.Rect{X: 30, Y: 30, W: 40, H: 32}
			if plane {
				k.W = k.H
			}
			build := func(reference bool) *ui.Tester {
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Draw(func(p *ui.Painter, _ ui.Rect) {
						if reference {
							glass.Paint(p, k, k.H/2, glass.Glass{Style: glass.Clear})
						} else if plane {
							paintColorPlaneKnob(p, k, held)
						} else {
							paintSliderKnob(p, c.Theme(), k, held, false)
						}
					})
				}, 100, 100)
				tt.SetDark(dark)
				return tt
			}
			tt, reference := build(false), build(true)
			x, y := int(k.X+k.W/2), int(k.Y+k.H/2)
			rest := bytes.Clone(tt.Image().Pix)
			want := uint8(255)
			if plane {
				want = 0 // The original plane marker is a ring, not a filled thumb.
			}
			if pixel := tt.Image().RGBAAt(x, y); pixel.R != want {
				t.Fatalf("dark=%v plane=%v: resting face changed: %v", dark, plane, pixel)
			}
			held = 1
			tt.Frame()
			if got, want := tt.Image().RGBAAt(x, y), reference.Image().RGBAAt(x, y); got != want {
				t.Fatalf("dark=%v plane=%v: held material %v, want Clear %v", dark, plane, got, want)
			}
			held = 0
			tt.Frame()
			if !bytes.Equal(rest, tt.Image().Pix) {
				t.Fatal("release did not restore the resting appearance")
			}
		}
	}
}

func TestSliderPressAndReleaseAppearance(t *testing.T) {
	for _, ranged := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			value, high := 50.0, 100.0
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Padding(30).Children(func() {
					o := SliderOptions{}
					if ranged {
						o.Range = &high
					}
					Slider(c, &value, 0, 100, o).Label("Slider").Width(240)
				})
			}, 300, 100)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.SetDark(dark)
			r, _ := tt.Find("Slider")
			x, y := r.X+r.W/2, r.Y+r.H/2
			rest := bytes.Clone(tt.Image().Pix)
			tt.Press(x, y)
			if bytes.Equal(rest, tt.Image().Pix) {
				t.Fatal("press did not reveal glass")
			}
			tt.Release(x, y)
			if value != 50 || high != 100 || !bytes.Equal(rest, tt.Image().Pix) {
				t.Fatal("release changed values or did not restore the resting face")
			}
		}
	}
}

// paintClearLens builds glass.Paint's Clear parameters itself; at the
// plugin's own bezel it must paint the same pixels.
func TestClearLensMatchesGlass(t *testing.T) {
	k := ui.Rect{X: 20, Y: 15, W: 50, H: 34}
	for _, scale := range []float32{1, 2} {
		images := [2][]byte{}
		for i := range images {
			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill().Background(ui.RGB(250, 250, 250)).Draw(func(p *ui.Painter, r ui.Rect) {
					for x := float32(0); x < r.W; x += 8 {
						p.Fill(ui.Rect{X: x, Y: 0, W: 4, H: r.H}, ui.RGB(30, 90, 200), 0)
					}
					if i == 0 {
						glass.Paint(p, k, k.H/2, glass.Glass{Style: glass.Clear})
					} else {
						paintClearLens(p, k, k.H/2, min(36, k.H/2))
					}
				})
			}, 90, 64)
			tt.SetScale(scale)
			images[i] = bytes.Clone(tt.Image().Pix)
		}
		if !bytes.Equal(images[0], images[1]) {
			t.Fatalf("scale %v: paintClearLens no longer matches glass.Paint's Clear material", scale)
		}
	}
}
