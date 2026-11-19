package components

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestColorPlaneThumbOverhang(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, disabled := range []bool{false, true} {
			for _, corner := range [][2]float32{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
				t.Run(fmt.Sprint(dark, disabled, corner), func(t *testing.T) {
					value := ui.RGB(255, 0, 0)
					off, changes := false, 0
					tt := ui.NewTester(func(c *ui.Context) {
						ui.Column(c).Padding(30).Disabled(off).Children(func() {
							if ColorView(c, &value).Changed() {
								changes++
							}
						})
					}, 360, 600)
					tt.SetDark(dark)
					r, _ := tt.Find("Saturation and brightness")
					cx, cy := r.X+r.W/2, r.Y+r.H/2
					x, y := r.X+corner[0]*r.W, r.Y+corner[1]*r.H
					tt.Press(cx, cy)
					tt.Move(x, y)
					tt.Release(x, y)
					before, count := value, changes
					off = disabled
					tt.Frame()
					// Start on the portion outside both edges of the plane.
					x += (corner[0]*2 - 1) * 4
					y += (corner[1]*2 - 1) * 4
					tt.Press(x, y)
					tt.Move(cx, cy)
					tt.Release(cx, cy)
					if disabled {
						if value != before || changes != count {
							t.Fatal("disabled plane accepted thumb input")
						}
					} else if value != ui.RGB(128, 64, 64) || changes <= count || !tt.Focused("Saturation and brightness") {
						t.Fatalf("overhang drag did not focus/update plane: %v, changes=%d", value, changes-count)
					}
				})
			}
		}
	}
}

func TestColorChannelOverhangSemantics(t *testing.T) {
	for _, hi := range []float64{1, 360} {
		for _, end := range []float64{0, hi} {
			for _, dark := range []bool{false, true} {
				value, disabled, changes := end, false, 0
				tt := ui.NewTester(func(c *ui.Context) {
					ui.Column(c).Padding(30).Disabled(disabled).Children(func() {
						e := colorChannel(c, &value, hi, "Channel", func(p *ui.Painter, r ui.Rect) {
							p.Fill(r, ui.RGB(200, 30, 100), 0)
						})
						if e.Changed() {
							changes++
						}
					})
				}, 300, 100)
				tt.SetDark(dark)
				r, _ := tt.Find("Channel")
				x, y := r.X+4, r.Y+r.H/2
				if end == hi {
					x = r.X + r.W - 4
				}
				tt.Press(x, y)
				if value != end || changes != 0 {
					t.Fatal("pressing the overhang changed an endpoint")
				}
				tt.Move(r.X+r.W/2, y)
				tt.Release(r.X+r.W/2, y)
				if value != hi/2 || changes != 1 || !tt.Focused("Channel") {
					t.Fatalf("captured drag: value=%v changes=%d", value, changes)
				}
				tt.Key(0, ui.KeyEnd)
				if value != hi || changes != 2 {
					t.Fatal("keyboard behavior lost")
				}
				value, disabled = end, true
				tt.Frame()
				tt.Press(x, y)
				tt.Move(r.X+r.W/2, y)
				tt.Release(r.X+r.W/2, y)
				if value != end || changes != 2 {
					t.Fatal("disabled channel accepted overhang input")
				}
			}
		}
	}
}

// A finger drags the saturation/brightness plane and a hue strip on a phone's
// touch screen, and the raw touches a Linux or Windows touchscreen sends do
// too.
func TestColorViewFingerDrag(t *testing.T) {
	for _, touch := range []bool{false, true} {
		t.Run(fmt.Sprint("raw=", touch), func(t *testing.T) {
			value := ui.RGB(255, 0, 0)
			changes := 0
			var state *colorViewState
			tt := ui.NewTester(func(c *ui.Context) {
				v := ColorView(c, &value)
				state = ui.Local(v.Element, "color-view", func() colorViewState { return colorViewState{} })
				if v.Changed() {
					changes++
				}
			}, 360, 600)
			tt.Frame()
			down := func(x, y float32) {
				if touch {
					tt.TouchDown(1, x, y)
					tt.Frame()
				} else {
					tt.FingerDown(x, y)
				}
			}
			move := func(x, y float32) {
				if touch {
					tt.TouchMove(1, x, y)
					tt.Frame()
				} else {
					tt.FingerMove(x, y)
				}
			}
			up := func(x, y float32) {
				if touch {
					tt.TouchUp(1, x, y)
				} else {
					tt.FingerUp(x, y)
				}
			}
			plane, ok := tt.Find("Saturation and brightness")
			if !ok {
				t.Fatal("missing plane")
			}
			down(plane.X+plane.W/2, plane.Y+plane.H/2)
			move(plane.X-10, plane.Y+plane.H+10)
			up(plane.X-10, plane.Y+plane.H+10)
			if value != ui.RGB(0, 0, 0) || changes == 0 {
				t.Fatalf("plane drag set %v, changes=%d", value, changes)
			}

			value, changes = ui.RGB(255, 0, 0), 0
			tt.Frame()
			hue, ok := tt.Find("Hue")
			if !ok {
				t.Fatal("missing hue strip")
			}
			down(hue.X+4, hue.Y+hue.H/2)
			move(hue.X+hue.W/2, hue.Y+hue.H/2)
			up(hue.X+hue.W/2, hue.Y+hue.H/2)
			if value.G == 0 || value.B == 0 || changes == 0 {
				t.Fatalf("hue drag set %v, changes=%d", value, changes)
			}
			if state.hsv.h < 170 || state.hsv.h > 200 {
				t.Fatalf("hue drag left hue at %v, want about half", state.hsv.h)
			}
		})
	}
}
